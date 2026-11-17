package vulns

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

type (
	// Registry hands out the SBOM a build attached to an image, as
	// resolver.Resolver does. found is false when the image has none.
	Registry interface {
		SBOM(ctx context.Context, repo, digest, arch string) (doc []byte, found bool, err error)
	}
	// Containers reads files from a container, as dockercli.Client does: every
	// regular file at or below path, none when it does not exist.
	Containers interface {
		Files(ctx context.Context, id, path string) (map[string][]byte, error)
	}
)

const (
	// recheck is how long a finding stands before the database is asked again.
	recheck = 24 * time.Hour
	// maxItems caps the list a window shows.
	maxItems = 20
)

// Checker counts the known vulnerabilities of a container's running image and
// what its newest image closes of them.
type Checker struct {
	db   *DB
	reg  Registry
	ctr  Containers
	dir  string
	arch string
	now  func() time.Time

	mu    sync.Mutex
	found map[string]cached
}

type cached struct {
	at       time.Time
	scope    string
	findings map[string]Finding
}

// New keeps the package lists of images under dir.
func New(dir string, db *DB, reg Registry, ctr Containers) *Checker {
	return &Checker{
		db: db, reg: reg, ctr: ctr,
		dir:   filepath.Join(dir, "images"),
		arch:  runtime.GOARCH,
		now:   time.Now,
		found: map[string]cached{},
	}
}

// Check counts what the running image of c carries and, given the digest of
// the image an update would pull, what that image no longer does.
func (k *Checker) Check(ctx context.Context, c model.Container, newestDigest string) *model.Vulns {
	run, ok, err := k.running(ctx, c)
	switch {
	case err != nil:
		log.Printf("shiplog: vulnerability check of %s failed: %v", c.Name, err)
		return &model.Vulns{Reason: model.VulnLookupFailed}
	case !ok:
		return &model.Vulns{Reason: model.VulnNoPackages}
	}
	v := &model.Vulns{Scope: run.scope}

	var newest map[string]Finding
	if newestDigest != "" && c.Repo != "" && !c.IsLocal && newestDigest != runningDigest(c) {
		n, found, err := k.image(ctx, c, newestDigest)
		if err != nil {
			log.Printf("shiplog: vulnerability check of the newest image of %s failed: %v", c.Name, err)
		}
		if found {
			newest, v.NewestKnown = n.findings, true
		}
	}

	for id, f := range run.findings {
		if f.Severity < High {
			continue
		}
		_, still := newest[id]
		fixed := v.NewestKnown && !still
		if f.Severity == Critical {
			v.Critical++
			if fixed {
				v.FixedCritical++
			}
		} else {
			v.High++
			if fixed {
				v.FixedHigh++
			}
		}
		v.Items = append(v.Items, model.VulnItem{ID: id, Package: f.Package, Severity: f.Severity.String(), Fixed: fixed})
	}
	sort.Slice(v.Items, func(i, j int) bool {
		a, b := v.Items[i], v.Items[j]
		if a.Severity != b.Severity {
			return a.Severity == "critical"
		}
		return a.ID < b.ID
	})
	if len(v.Items) > maxItems {
		v.Items = v.Items[:maxItems]
	}
	return v
}

func runningDigest(c model.Container) string {
	if c.PinnedDigest != "" {
		return c.PinnedDigest
	}
	return c.Digest
}

// running prefers the image's own list, which also names the packages of the
// app's language, and falls back to the system packages inside the container.
func (k *Checker) running(ctx context.Context, c model.Container) (cached, bool, error) {
	if d := runningDigest(c); d != "" && c.Repo != "" && !c.IsLocal {
		res, found, err := k.image(ctx, c, d)
		if found || err != nil {
			return res, found, err
		}
	}
	key := "ctr:" + c.ID + "|" + c.ImageID
	if res, ok := k.cached(key); ok {
		return res, true, nil
	}
	pkgs := k.systemPackages(ctx, c)
	if len(pkgs) == 0 {
		return cached{}, false, nil
	}
	res, err := k.lookup(ctx, key, model.VulnScopeSystem, pkgs)
	return res, err == nil, err
}

// image returns the findings for the image behind digest. A registry that
// cannot answer reads as an image without a list, since the system packages
// can still stand in. Only a failed database lookup is an error.
func (k *Checker) image(ctx context.Context, c model.Container, digest string) (cached, bool, error) {
	key := "img:" + digest
	if res, ok := k.cached(key); ok {
		return res, true, nil
	}
	pkgs, found := k.imagePackages(ctx, c, digest)
	if !found {
		return cached{}, false, nil
	}
	res, err := k.lookup(ctx, key, model.VulnScopeImage, pkgs)
	return res, err == nil, err
}

func (k *Checker) cached(key string) (cached, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	res, ok := k.found[key]
	return res, ok && k.now().Sub(res.at) < recheck
}

func (k *Checker) lookup(ctx context.Context, key, scope string, pkgs []Package) (cached, error) {
	f, err := k.db.Find(ctx, pkgs)
	if err != nil {
		return cached{}, err
	}
	res := cached{at: k.now(), scope: scope, findings: f}
	k.mu.Lock()
	k.found[key] = res
	k.mu.Unlock()
	return res, nil
}

// imageList is what the disk keeps per image digest. An image's SBOM never
// changes, so neither a list nor its absence is asked for twice.
type imageList struct {
	Found    bool      `json:"found"`
	Packages []Package `json:"packages,omitempty"`
}

var safeDigest = regexp.MustCompile(`^[a-z0-9]+:[a-f0-9]+$`)

func (k *Checker) imagePackages(ctx context.Context, c model.Container, digest string) ([]Package, bool) {
	var file string
	if safeDigest.MatchString(digest) {
		file = filepath.Join(k.dir, strings.Replace(digest, ":", "-", 1)+".json")
		if b, err := os.ReadFile(file); err == nil {
			var l imageList
			if json.Unmarshal(b, &l) == nil {
				return l.Packages, l.Found
			}
		}
	}
	doc, found, err := k.reg.SBOM(ctx, c.Repo, digest, k.arch)
	if err != nil {
		log.Printf("shiplog: no package list for an image of %s from the registry: %v", c.Name, err)
		return nil, false
	}
	l := imageList{Found: found}
	if found {
		pkgs, perr := parseSPDX(doc)
		if perr != nil {
			log.Printf("shiplog: the package list of an image of %s does not read: %v", c.Name, perr)
		}
		l.Packages, l.Found = pkgs, len(pkgs) > 0
	}
	if file != "" {
		if b, merr := json.Marshal(l); merr == nil {
			_ = writeFile(file, b)
		}
	}
	return l.Packages, l.Found
}

// releaseFiles may be links to each other, and only a regular file comes back.
var releaseFiles = []string{"/usr/lib/os-release", "/etc/os-release"}

// systemPackages reads the distribution's package database in the container.
func (k *Checker) systemPackages(ctx context.Context, c model.Container) []Package {
	eco := ""
	for _, p := range releaseFiles {
		if b := k.file(ctx, c, p)[p]; b != nil {
			eco = ecosystem(string(b))
			break
		}
	}
	var l list
	switch {
	case strings.HasPrefix(eco, "Debian:"), strings.HasPrefix(eco, "Ubuntu:"):
		for _, path := range []string{"/var/lib/dpkg/status", "/var/lib/dpkg/status.d"} {
			for name, b := range k.file(ctx, c, path) {
				if strings.HasSuffix(name, ".md5sums") {
					continue
				}
				for _, p := range parseDpkg(bytes.NewReader(b), eco) {
					l.add(p)
				}
			}
		}
	case strings.HasPrefix(eco, "Alpine:"):
		for _, b := range k.file(ctx, c, "/lib/apk/db/installed") {
			for _, p := range parseApk(bytes.NewReader(b), eco) {
				l.add(p)
			}
		}
	}
	return l.pkgs
}

func (k *Checker) file(ctx context.Context, c model.Container, path string) map[string][]byte {
	files, err := k.ctr.Files(ctx, c.ID, path)
	if err != nil {
		log.Printf("shiplog: could not read %s in %s: %v", path, c.Name, err)
		return nil
	}
	return files
}
