package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// plg writes a manifest in the shape real plugins use: entities that refer
// to other entities, and release notes with a bare &.
func plg(version, min string, notes ...string) string {
	minAttr := ""
	if min != "" {
		minAttr = ` min="` + min + `"`
	}
	return `<?xml version='1.0' standalone='yes'?>
<!DOCTYPE PLUGIN [
<!ENTITY name		"demo.plugin">
<!ENTITY author		"someone">
<!ENTITY version	"` + version + `">
<!ENTITY gitURL		"https://raw.githubusercontent.com/someone/&name;/main">
<!ENTITY pluginURL	"&gitURL;/&name;.plg">
]>

<PLUGIN name="&name;" author="&author;" version="&version;" pluginURL="&pluginURL;" support="https://forums.unraid.net/x"` + minAttr + `>

<CHANGES>
` + strings.Join(notes, "\n\n") + `
</CHANGES>

<FILE Run="/bin/bash"><INLINE>echo hi</INLINE></FILE>
</PLUGIN>
`
}

type host struct {
	t                        *testing.T
	boot, installed, fetched string
}

func newHost(t *testing.T) *host {
	t.Helper()
	root := t.TempDir()
	h := &host{t: t, boot: filepath.Join(root, "boot"), installed: filepath.Join(root, "installed"), fetched: filepath.Join(root, "fetched")}
	for _, d := range []string{h.boot, h.installed, h.fetched} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

// install puts the .plg on the flash and links it from the installed list,
// as Unraid does.
func (h *host) install(file, body string) {
	h.t.Helper()
	target := filepath.Join(h.boot, file)
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(h.installed, file)); err != nil {
		h.t.Fatal(err)
	}
}

func (h *host) fetch(file, body string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.fetched, file), []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *host) scan(unraid string) []Plugin {
	h.t.Helper()
	ps, err := (&Scanner{Installed: h.installed, Fetched: h.fetched, Unraid: func() string { return unraid }}).Scan()
	if err != nil {
		h.t.Fatal(err)
	}
	return ps
}

func TestParseExpandsNestedEntities(t *testing.T) {
	m, err := Parse([]byte(plg("2026.10.12", "6.12.0", "###2026.10.12\n- Fixed: this & that")))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "demo.plugin" || m.Version != "2026.10.12" || m.Min != "6.12.0" ||
		m.URL != "https://raw.githubusercontent.com/someone/demo.plugin/main/demo.plugin.plg" {
		t.Errorf("manifest = %+v", m)
	}
	if !strings.Contains(m.Changes, "this & that") {
		t.Errorf("changes = %q", m.Changes)
	}
}

func TestANewerFetchedPlgIsAnUpdate(t *testing.T) {
	h := newHost(t)
	h.install("demo.plg", plg("2026.01.01", ""))
	h.fetch("demo.plg", plg("2026.03.01", "", "###2026.03.01\n- New", "###2026.02.01\n- Middle", "###2026.01.01\n- Installed"))
	ps := h.scan("7.2.0")
	if len(ps) != 1 {
		t.Fatalf("plugins = %+v", ps)
	}
	p := ps[0]
	if p.State != StateAvailable || p.Installed != "2026.01.01" || p.Latest != "2026.03.01" || p.File != "demo.plg" {
		t.Errorf("plugin = %+v", p)
	}
	if want := "###2026.03.01\n- New\n\n###2026.02.01\n- Middle"; p.Changes != want {
		t.Errorf("changes = %q, want %q", p.Changes, want)
	}
	if p.CheckedAt.IsZero() || time.Since(p.CheckedAt) > time.Minute {
		t.Errorf("checkedAt = %v", p.CheckedAt)
	}
}

func TestTheSameOrAnOlderFetchedPlgIsCurrent(t *testing.T) {
	h := newHost(t)
	h.install("a.plg", plg("2026.03.01", ""))
	h.fetch("a.plg", plg("2026.03.01", ""))
	h.install("b.plg", plg("2026.03.01", ""))
	h.fetch("b.plg", plg("2026.02.01", ""))
	for _, p := range h.scan("") {
		if p.State != StateCurrent || p.Changes != "" {
			t.Errorf("%s = %+v, want current", p.File, p)
		}
	}
}

func TestWithoutACheckSinceBootThePluginIsUnchecked(t *testing.T) {
	h := newHost(t)
	h.install("demo.plg", plg("2026.01.01", ""))
	if ps := h.scan(""); len(ps) != 1 || ps[0].State != StateUnchecked || ps[0].Latest != "" {
		t.Errorf("plugins = %+v", ps)
	}
}

// plugin update refuses a release whose min is above the running Unraid.
func TestAReleaseForANewerUnraidIsIncompatible(t *testing.T) {
	h := newHost(t)
	h.install("demo.plg", plg("2026.01.01", ""))
	h.fetch("demo.plg", plg("2026.03.01", "7.3.0"))
	if ps := h.scan("7.2.4"); ps[0].State != StateIncompatible || !ps[0].HasUpdate() {
		t.Errorf("on 7.2.4: %+v", ps[0])
	}
	if ps := h.scan("7.3.0"); ps[0].State != StateAvailable {
		t.Errorf("on 7.3.0: %+v", ps[0])
	}
}

// Like `plugin checkall`: the OS plugin, plain files, dangling links and
// plugins without a pluginURL are not plugins to update.
func TestTheScanSkipsWhatUnraidDoesNotUpdate(t *testing.T) {
	h := newHost(t)
	h.install("unRAIDServer.plg", plg("7.2.4", ""))
	h.install("good.plg", plg("2026.01.01", ""))
	if err := os.WriteFile(filepath.Join(h.installed, "plain.plg"), []byte(plg("1", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(h.boot, "gone.plg"), filepath.Join(h.installed, "gone.plg")); err != nil {
		t.Fatal(err)
	}
	h.install("local.plg", `<?xml version='1.0'?><PLUGIN name="local" version="1"></PLUGIN>`)
	h.install("broken.plg", "not xml at all")
	ps := h.scan("")
	if len(ps) != 1 || ps[0].File != "good.plg" {
		t.Errorf("plugins = %+v, want good.plg alone", ps)
	}
}

func TestCompareVersionsReadsUnraidVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"7.2.0", "7.2.0", 0}, {"7.2.0", "7.10.0", -1}, {"6.12.10", "6.12.9", 1},
		{"7.2", "7.2.0", 0}, {"7.2.0-rc.1", "7.2.0", -1}, {"7.2.0-rc.2", "7.2.0-rc.1", 1},
	} {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestReadUnraidVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "unraid-version")
	if err := os.WriteFile(p, []byte("version=\"7.2.4\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadUnraidVersion(p); got != "7.2.4" {
		t.Errorf("got %q", got)
	}
	if got := ReadUnraidVersion(p + ".missing"); got != "" {
		t.Errorf("missing file: got %q", got)
	}
}

// Real notes carry old headings with typos, such as ###2033.02.18 below
// ###2025.11.18, which must not come back as news.
func TestTheNotesStopAtTheInstalledRelease(t *testing.T) {
	notes := "###2026.10.12\n- New\n\n###2025.08.07\n- Installed\n\n###2033.02.18\n- Typo year"
	if got, want := Newer(notes, "2025.08.07"), "###2026.10.12\n- New"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNotesWithoutVersionHeadingsComeBackWhole(t *testing.T) {
	if got := Newer("Bug fixes & speedups.", "1.0"); got != "Bug fixes & speedups." {
		t.Errorf("got %q", got)
	}
}
