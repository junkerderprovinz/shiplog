package vulns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

type fakeRegistry struct {
	mu    sync.Mutex
	sboms map[string]string // digest -> SPDX document
	err   error
	calls map[string]int
}

func (f *fakeRegistry) SBOM(_ context.Context, repo, digest, arch string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[digest]++
	if f.err != nil {
		return nil, false, f.err
	}
	doc, ok := f.sboms[digest]
	return []byte(doc), ok, nil
}

type fakeContainers struct {
	files map[string]string // path -> content
	reads int
}

func (f *fakeContainers) Files(_ context.Context, id, path string) (map[string][]byte, error) {
	f.reads++
	out := map[string][]byte{}
	for p, body := range f.files {
		if p == path || strings.HasPrefix(p, path+"/") {
			out[p] = []byte(body)
		}
	}
	return out, nil
}

func spdx(purls ...string) string {
	var refs []string
	for _, p := range purls {
		refs = append(refs, fmt.Sprintf(`{"externalRefs":[{"referenceType":"purl","referenceLocator":%q}]}`, p))
	}
	return `{"predicate":{"packages":[` + strings.Join(refs, ",") + `]}}`
}

// vulnDB knows two critical flaws in openssl 3.0.11 and one high one in
// glibc 2.36-9, of which openssl 3.0.15 fixes one critical. Debian has no fix
// for the other critical one yet.
func vulnDB(t *testing.T) *DB {
	f := &fakeDB{
		affects: map[string][]string{
			"openssl 3.0.11": {"DEBIAN-CVE-1", "DEBIAN-CVE-2", "DEBIAN-CVE-4"},
			"openssl 3.0.15": {"DEBIAN-CVE-2"},
			"glibc 2.36-9":   {"DEBIAN-CVE-3"},
		},
		records: map[string]osvRecord{
			"DEBIAN-CVE-1": affected(rec("DEBIAN-CVE-1", "m", critVector, "CVE-1"), "Debian:12", "openssl", map[string]string{"fixed": "3.0.15"}),
			"DEBIAN-CVE-2": affected(rec("DEBIAN-CVE-2", "m", critVector, "CVE-2"), "Debian:12", "openssl"),
			"DEBIAN-CVE-3": affected(rec("DEBIAN-CVE-3", "m", highVector, "CVE-3"), "Debian:12", "glibc", map[string]string{"fixed": "2.36-9+deb12u1"}),
			"DEBIAN-CVE-4": rec("DEBIAN-CVE-4", "m", lowVector, "CVE-4"),
		},
	}
	return NewDB(f.server(t).URL, t.TempDir())
}

func newChecker(t *testing.T, reg *fakeRegistry, ctr *fakeContainers) *Checker {
	t.Helper()
	k := New(t.TempDir(), vulnDB(t), reg, ctr)
	k.arch = "amd64"
	return k
}

const (
	oldSBOM = "sha256:0a1d"
	newSBOM = "sha256:0e3f"
)

func registryWithBoth() *fakeRegistry {
	return &fakeRegistry{sboms: map[string]string{
		oldSBOM: spdx("pkg:deb/debian/libssl3@3.0.11?distro=debian-12&upstream=openssl", "pkg:deb/debian/libc6@2.36-9?distro=debian-12&upstream=glibc"),
		newSBOM: spdx("pkg:deb/debian/libssl3@3.0.15?distro=debian-12&upstream=openssl", "pkg:deb/debian/libc6@2.36-9?distro=debian-12&upstream=glibc"),
	}}
}

func TestTheRunningImageCountsItsCriticalAndHighFlaws(t *testing.T) {
	k := newChecker(t, registryWithBoth(), &fakeContainers{})
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}, "")
	if v.Scope != model.VulnScopeImage || v.Critical != 2 || v.High != 1 || v.NewestKnown {
		t.Fatalf("Check = %+v, want 2 critical and 1 high from the image's own list", v)
	}
	if v.NoFixCritical != 1 || v.NoFixHigh != 0 {
		t.Errorf("without a fix: %d critical and %d high, want 1 critical", v.NoFixCritical, v.NoFixHigh)
	}
	var order []string
	for _, it := range v.Items {
		order = append(order, fmt.Sprintf("%s %s %s %v", it.ID, it.Package, it.Severity, it.NoFix))
	}
	want := []string{"CVE-1 openssl critical false", "CVE-3 glibc high false", "CVE-2 openssl critical true"}
	if strings.Join(order, ", ") != strings.Join(want, ", ") {
		t.Errorf("items = %q, want the ones with a fix first: %q", order, want)
	}
}

func TestAFlawTheUpdateClosesHasAFix(t *testing.T) {
	f := &fakeDB{
		affects: map[string][]string{"openssl 3.0.11": {"DEBIAN-CVE-1"}},
		records: map[string]osvRecord{"DEBIAN-CVE-1": affected(rec("DEBIAN-CVE-1", "m", critVector, "CVE-1"), "Debian:12", "openssl")},
	}
	k := New(t.TempDir(), NewDB(f.server(t).URL, t.TempDir()), registryWithBoth(), &fakeContainers{})
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}, newSBOM)
	if v.NoFixCritical != 0 || len(v.Items) != 1 || v.Items[0].NoFix || !v.ClosesCritical() {
		t.Errorf("Check = %+v, want the update to count as the fix", v)
	}
}

func TestTheNewestImageTellsWhatAnUpdateCloses(t *testing.T) {
	k := newChecker(t, registryWithBoth(), &fakeContainers{})
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}, newSBOM)
	if !v.NewestKnown || v.FixedCritical != 1 || v.FixedHigh != 0 {
		t.Fatalf("Check = %+v, want one critical closed", v)
	}
	if !v.ClosesCritical() {
		t.Error("ClosesCritical = false")
	}
	fixed := map[string]bool{}
	for _, it := range v.Items {
		fixed[it.ID] = it.Fixed
	}
	if !fixed["CVE-1"] || fixed["CVE-2"] || fixed["CVE-3"] {
		t.Errorf("fixed = %v, want only CVE-1", fixed)
	}
}

func TestWithoutTheNewestListNobodyClaimsAFix(t *testing.T) {
	reg := registryWithBoth()
	delete(reg.sboms, newSBOM)
	k := newChecker(t, reg, &fakeContainers{})
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}, newSBOM)
	if v.NewestKnown || v.FixedCritical != 0 || v.ClosesCritical() {
		t.Errorf("Check = %+v, want the fix unknown", v)
	}
	if v.Critical != 2 {
		t.Errorf("critical = %d, want the running image still counted", v.Critical)
	}
}

func TestWithoutAnImageListTheContainersOwnPackagesCount(t *testing.T) {
	ctr := &fakeContainers{files: map[string]string{
		"/usr/lib/os-release": "ID=debian\nVERSION_ID=\"12\"\n",
		"/var/lib/dpkg/status": "Package: libssl3\nStatus: install ok installed\nSource: openssl\nVersion: 3.0.11\n\n" +
			"Package: libc6\nStatus: install ok installed\nSource: glibc\nVersion: 2.36-9\n",
	}}
	k := newChecker(t, &fakeRegistry{}, ctr)
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/x/app", Digest: oldSBOM}, "")
	if v.Scope != model.VulnScopeSystem || v.Critical != 2 || v.High != 1 {
		t.Errorf("Check = %+v, want the system packages counted", v)
	}
}

func TestADistrolessImageReadsItsStatusFolder(t *testing.T) {
	ctr := &fakeContainers{files: map[string]string{
		"/etc/os-release":                "ID=debian\nVERSION_ID=\"12\"\n",
		"/var/lib/dpkg/status.d/libssl3": "Package: libssl3\nSource: openssl\nVersion: 3.0.11\n",
	}}
	k := newChecker(t, &fakeRegistry{}, ctr)
	v := k.Check(context.Background(), model.Container{ID: "c1", IsLocal: true}, "")
	if v.Scope != model.VulnScopeSystem || v.Critical != 2 {
		t.Errorf("Check = %+v", v)
	}
}

func TestAnAlpineContainerReadsItsApkDatabase(t *testing.T) {
	ctr := &fakeContainers{files: map[string]string{
		"/etc/os-release":       "ID=alpine\nVERSION_ID=3.20.3\n",
		"/lib/apk/db/installed": "P:libssl3\nV:3.0.11\no:openssl\n",
	}}
	reg := &fakeRegistry{}
	k := newChecker(t, reg, ctr)
	v := k.Check(context.Background(), model.Container{ID: "c1", IsLocal: true}, "")
	if v.Scope != model.VulnScopeSystem || v.Reason != "" {
		t.Errorf("Check = %+v, want the apk database read", v)
	}
	if len(reg.calls) != 0 {
		t.Errorf("a local image was looked up in a registry: %v", reg.calls)
	}
}

func TestKernelPackagesDoNotCount(t *testing.T) {
	// The container runs the host's kernel, so the headers its image carries
	// from the kernel's source package bring no flaw of their own.
	reg := &fakeRegistry{sboms: map[string]string{
		oldSBOM: spdx("pkg:deb/debian/linux-libc-dev@3.0.11?distro=debian-13&upstream=linux"),
	}}
	f := &fakeDB{affects: map[string][]string{"linux 3.0.11": {"DEBIAN-CVE-1"}}, records: map[string]osvRecord{"DEBIAN-CVE-1": rec("DEBIAN-CVE-1", "m", critVector, "CVE-1")}}
	k := New(t.TempDir(), NewDB(f.server(t).URL, t.TempDir()), reg, &fakeContainers{})
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}, "")
	if v.Critical != 0 || len(f.queries) != 0 {
		t.Errorf("Check = %+v after %d queries, want the kernel left out", v, len(f.queries))
	}
}

func TestAnUncoveredSystemSaysSo(t *testing.T) {
	ctr := &fakeContainers{files: map[string]string{"/etc/os-release": "ID=arch\n"}}
	k := newChecker(t, &fakeRegistry{}, ctr)
	v := k.Check(context.Background(), model.Container{ID: "c1", IsLocal: true}, "")
	if v.Reason != model.VulnNoPackages || v.Critical != 0 {
		t.Errorf("Check = %+v, want no_packages", v)
	}
}

func TestAFailedLookupSaysSoInsteadOfZero(t *testing.T) {
	reg := registryWithBoth()
	k := New(t.TempDir(), NewDB("http://127.0.0.1:1", t.TempDir()), reg, &fakeContainers{})
	v := k.Check(context.Background(), model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}, "")
	if v.Reason != model.VulnLookupFailed {
		t.Errorf("Check = %+v, want lookup_failed", v)
	}
}

func TestAnImageListIsFetchedOncePerDigest(t *testing.T) {
	reg := registryWithBoth()
	dir := t.TempDir()
	db := vulnDB(t)
	c := model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}
	New(dir, db, reg, &fakeContainers{}).Check(context.Background(), c, newSBOM)
	// A restart keeps the lists on disk.
	New(dir, db, reg, &fakeContainers{}).Check(context.Background(), c, newSBOM)
	if reg.calls[oldSBOM] != 1 || reg.calls[newSBOM] != 1 {
		t.Errorf("registry calls = %v, want one per digest", reg.calls)
	}
}

func TestADigestWithoutAListIsNotAskedAgain(t *testing.T) {
	reg := &fakeRegistry{}
	dir := t.TempDir()
	c := model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}
	for range 2 {
		New(dir, vulnDB(t), reg, &fakeContainers{}).Check(context.Background(), c, "")
	}
	if reg.calls[oldSBOM] != 1 {
		t.Errorf("registry calls = %v, want one", reg.calls)
	}
}

func TestARegistryErrorIsAskedAgainNextTime(t *testing.T) {
	reg := &fakeRegistry{err: errors.New("429")}
	dir := t.TempDir()
	c := model.Container{ID: "c1", Repo: "docker.io/library/app", Digest: oldSBOM}
	New(dir, vulnDB(t), reg, &fakeContainers{}).Check(context.Background(), c, "")
	reg.err = nil
	reg.sboms = registryWithBoth().sboms
	v := New(dir, vulnDB(t), reg, &fakeContainers{}).Check(context.Background(), c, "")
	if v.Scope != model.VulnScopeImage {
		t.Errorf("Check = %+v, want the image list once the registry answers", v)
	}
}

func TestFindingsAreReusedForADay(t *testing.T) {
	ctr := &fakeContainers{files: map[string]string{
		"/etc/os-release":      "ID=debian\nVERSION_ID=\"12\"\n",
		"/var/lib/dpkg/status": "Package: openssl\nStatus: install ok installed\nVersion: 3.0.11\n",
	}}
	k := newChecker(t, &fakeRegistry{}, ctr)
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	k.now = func() time.Time { return now }
	c := model.Container{ID: "c1", IsLocal: true}
	k.Check(context.Background(), c, "")
	reads := ctr.reads
	now = now.Add(23 * time.Hour)
	k.Check(context.Background(), c, "")
	if ctr.reads != reads {
		t.Errorf("read the container again within a day")
	}
	now = now.Add(2 * time.Hour)
	k.Check(context.Background(), c, "")
	if ctr.reads == reads {
		t.Errorf("did not read the container again after a day")
	}
}
