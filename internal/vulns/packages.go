// Package vulns counts the known vulnerabilities of a container's image by
// looking its packages up in an open vulnerability database. The package list
// comes from the image's own SBOM in the registry or, for the running image,
// from the package database inside the container, so no image is downloaded
// and nothing but package names and versions leaves the host.
package vulns

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// Package is one entry of a package list, named the way the vulnerability
// database files it: a distribution package under its source package.
type Package struct {
	Ecosystem string `json:"e"`
	Name      string `json:"n"`
	Version   string `json:"v"`
}

// languageEcosystems maps a package URL type to the database's ecosystem.
var languageEcosystems = map[string]string{
	"npm":      "npm",
	"pypi":     "PyPI",
	"nuget":    "NuGet",
	"cargo":    "crates.io",
	"gem":      "RubyGems",
	"composer": "Packagist",
}

// fromPURL reads a package URL as an SBOM writes it. Types and distributions
// the database does not cover come back false.
func fromPURL(purl string) (Package, bool) {
	rest, ok := strings.CutPrefix(purl, "pkg:")
	if !ok {
		return Package{}, false
	}
	var subpath string
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest, subpath = rest[:i], rest[i+1:]
	}
	var qs url.Values
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		qs, _ = url.ParseQuery(rest[i+1:])
		rest = rest[:i]
	}
	at := strings.LastIndexByte(rest, '@')
	if at < 0 {
		return Package{}, false
	}
	version, err := url.PathUnescape(rest[at+1:])
	if err != nil || version == "" {
		return Package{}, false
	}
	typ, path, ok := strings.Cut(rest[:at], "/")
	if !ok {
		return Package{}, false
	}
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if segs[i], err = url.PathUnescape(s); err != nil {
			return Package{}, false
		}
	}
	name := segs[len(segs)-1]
	namespace := strings.Join(segs[:len(segs)-1], "/")

	switch typ {
	case "deb", "apk":
		eco := distroEcosystem(namespace, qs)
		if eco == "" {
			return Package{}, false
		}
		if up := qs.Get("upstream"); up != "" {
			// An upstream may carry the source version as "name@version".
			if n, v, found := strings.Cut(up, "@"); found {
				name, version = n, v
			} else {
				name = up
			}
		}
		return Package{eco, name, version}, true
	case "golang":
		mod := strings.Join(segs, "/")
		if subpath != "" {
			mod += "/" + subpath
		}
		if mod == "stdlib" {
			version = strings.TrimPrefix(version, "go")
		}
		return Package{"Go", mod, version}, true
	case "maven":
		if namespace == "" {
			return Package{}, false
		}
		return Package{"Maven", namespace + ":" + name, version}, true
	}
	eco, known := languageEcosystems[typ]
	if !known {
		return Package{}, false
	}
	if namespace != "" {
		name = namespace + "/" + name
	}
	return Package{eco, name, version}, true
}

var (
	majorVersion = regexp.MustCompile(`^\d+`)
	minorVersion = regexp.MustCompile(`^\d+\.\d+`)
)

// distroEcosystem names a distribution release from a package URL's
// qualifiers, which come as "distro=debian-12" or as "os_version=13".
func distroEcosystem(distro string, qs url.Values) string {
	release := qs.Get("os_version")
	if d := qs.Get("distro"); d != "" {
		release = strings.TrimPrefix(d, distro+"-")
	}
	return releaseEcosystem(distro, release)
}

func releaseEcosystem(distro, release string) string {
	switch distro {
	case "debian":
		if m := majorVersion.FindString(release); m != "" {
			return "Debian:" + m
		}
	case "ubuntu":
		v := minorVersion.FindString(release)
		if v == "" {
			return ""
		}
		year, month, _ := strings.Cut(v, ".")
		// Ubuntu calls every April release of an even year LTS.
		if month == "04" && (year[len(year)-1]-'0')%2 == 0 {
			return "Ubuntu:" + v + ":LTS"
		}
		return "Ubuntu:" + v
	case "alpine":
		if v := minorVersion.FindString(release); v != "" {
			return "Alpine:v" + v
		}
	}
	return ""
}

// ecosystem names the distribution an os-release file describes, or "" for
// one the database has no packages of.
func ecosystem(osRelease string) string {
	var id, version string
	for _, line := range strings.Split(osRelease, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		switch k {
		case "ID":
			id = v
		case "VERSION_ID":
			version = v
		}
	}
	return releaseEcosystem(id, version)
}

// stanzas splits a package database into its blank-line separated records.
func stanzas(r io.Reader, each func(map[string]string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	rec := map[string]string{}
	flush := func() {
		if len(rec) > 0 {
			each(rec)
			rec = map[string]string{}
		}
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		// A dpkg continuation line belongs to a long field nothing here reads.
		if line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			rec[k] = strings.TrimSpace(v)
		}
	}
	flush()
}

// parseDpkg reads /var/lib/dpkg/status or a file of status.d. A package
// counts under its source package, whose version a binNMU leaves behind.
func parseDpkg(r io.Reader, eco string) []Package {
	var l list
	stanzas(r, func(rec map[string]string) {
		if st, ok := rec["Status"]; ok && !strings.HasSuffix(st, " installed") {
			return
		}
		name, version := rec["Package"], rec["Version"]
		if src := rec["Source"]; src != "" {
			name = src
			if n, v, ok := strings.Cut(src, " ("); ok {
				name, version = n, strings.TrimSuffix(v, ")")
			}
		}
		l.add(Package{eco, name, version})
	})
	return l.pkgs
}

// parseApk reads /lib/apk/db/installed, where a package counts under its
// origin.
func parseApk(r io.Reader, eco string) []Package {
	var l list
	stanzas(r, func(rec map[string]string) {
		name := rec["P"]
		if o := rec["o"]; o != "" {
			name = o
		}
		l.add(Package{eco, name, rec["V"]})
	})
	return l.pkgs
}

// parseSPDX reads the SPDX document of an SBOM attestation, wrapped in its
// in-toto statement or bare. Only the package URLs matter.
func parseSPDX(data []byte) ([]Package, error) {
	type spdxPackage struct {
		ExternalRefs []struct {
			ReferenceType    string `json:"referenceType"`
			ReferenceLocator string `json:"referenceLocator"`
		} `json:"externalRefs"`
	}
	var doc struct {
		Packages  []spdxPackage `json:"packages"`
		Predicate *struct {
			Packages []spdxPackage `json:"packages"`
		} `json:"predicate"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	pkgs := doc.Packages
	if doc.Predicate != nil {
		pkgs = doc.Predicate.Packages
	}
	var l list
	for _, p := range pkgs {
		for _, ref := range p.ExternalRefs {
			if ref.ReferenceType != "purl" {
				continue
			}
			if pkg, ok := fromPURL(ref.ReferenceLocator); ok {
				l.add(pkg)
			}
		}
	}
	return l.pkgs, nil
}

// list keeps packages in the order they came, each once.
type list struct {
	pkgs []Package
	seen map[Package]bool
}

func (l *list) add(p Package) {
	if p.Name == "" || p.Version == "" {
		return
	}
	if l.seen == nil {
		l.seen = map[Package]bool{}
	}
	if !l.seen[p] {
		l.seen[p] = true
		l.pkgs = append(l.pkgs, p)
	}
}
