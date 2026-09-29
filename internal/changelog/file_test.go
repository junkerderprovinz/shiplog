package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// historyTxt follows Domoticz's History.txt: plain "Version" lines, the
// upcoming release on top, and a Markdown sub-heading inside one section.
const historyTxt = `Version 2026.4
This release contains important security fixes.
- Added: Matter, SmokeCoAlarm cluster
- Fixed: Charts, hourly bars

Version 2026.3 (August 2nd 2026)
- Fixed: Email subject encoding

### Breaking Changes
- Removed the Time counter

Version 2026.2 (May 31st 2026)
- Added: something older
`

const keepAChangelog = `# Changelog

## [Unreleased](https://github.com/o/r/compare/v1.3.0...HEAD)
- Work in progress

## [1.3.0](https://github.com/o/r/compare/v1.2.0...v1.3.0) - 2026-03-01
### Added
- Third feature

## [1.2.0] - 2026-02-01
### Fixed
- Second fix

## [1.1.0] - 2026-01-01
- First release
`

func serveFile(t *testing.T, contentType, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/CHANGELOG" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFile_RollingTag_ShowsNewestSection(t *testing.T) {
	srv := serveFile(t, "text/plain; charset=utf-8", historyTxt)
	c := model.Container{ChangelogFile: srv.URL + "/CHANGELOG"}

	cl, ok := NewFile().Get(context.Background(), c, "latest", "latest")
	if !ok {
		t.Fatal("expected a handled changelog")
	}
	if cl.Provider != "file" || cl.URL != c.ChangelogFile || cl.Recent {
		t.Errorf("got provider %q, url %q, recent %v", cl.Provider, cl.URL, cl.Recent)
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "Version 2026.4" {
		t.Fatalf("entries = %+v, want only Version 2026.4", cl.Entries)
	}
	if !strings.HasPrefix(cl.Raw, "## Version 2026.4\n") || !strings.Contains(cl.Raw, "SmokeCoAlarm") || strings.Contains(cl.Raw, "Email subject") {
		t.Errorf("raw = %q", cl.Raw)
	}
}

func TestFile_SubHeadingStaysInItsSection(t *testing.T) {
	secs := splitSections(historyTxt)
	if len(secs) != 3 {
		t.Fatalf("got %d sections, want 3", len(secs))
	}
	if !strings.Contains(secs[1].body, "### Breaking Changes") || !strings.Contains(secs[1].body, "Time counter") {
		t.Errorf("2026.3 body = %q", secs[1].body)
	}
}

func TestFile_VersionSpan_ListsSectionsBetween(t *testing.T) {
	srv := serveFile(t, "text/markdown", keepAChangelog)
	c := model.Container{ChangelogFile: srv.URL + "/CHANGELOG"}

	cl, ok := NewFile().Get(context.Background(), c, "1.1.0", "v1.3.0")
	if !ok {
		t.Fatal("expected a handled changelog")
	}
	if !cl.Recent || len(cl.Entries) != 2 {
		t.Fatalf("recent %v, entries %+v; want 1.3.0 and 1.2.0", cl.Recent, cl.Entries)
	}
	if got := cl.Entries[0].Tag; got != "1.3.0 - 2026-03-01" {
		t.Errorf("first title = %q", got)
	}
	if want := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC); !cl.Entries[0].PublishedAt.Equal(want) {
		t.Errorf("date = %v, want %v", cl.Entries[0].PublishedAt, want)
	}
	if !strings.Contains(cl.Entries[0].Body, "### Added") || !strings.Contains(cl.Entries[1].Body, "Second fix") {
		t.Errorf("bodies = %q / %q", cl.Entries[0].Body, cl.Entries[1].Body)
	}
}

func TestFile_UnreleasedIsNeverTheNewestSection(t *testing.T) {
	srv := serveFile(t, "text/plain", keepAChangelog)
	cl, ok := NewFile().Get(context.Background(), model.Container{ChangelogFile: srv.URL + "/CHANGELOG"}, "latest", "latest")
	if !ok || len(cl.Entries) != 1 || !strings.HasPrefix(cl.Entries[0].Tag, "1.3.0") {
		t.Fatalf("got %+v, want the 1.3.0 section", cl)
	}
}

func TestFile_SetextHeadings(t *testing.T) {
	secs := splitSections("2.0.0\n=====\n- big change\n\n1.9.0\n=====\n- small change\n")
	if len(secs) != 2 || secs[0].title != "2.0.0" || secs[0].body != "- big change" {
		t.Fatalf("sections = %+v", secs)
	}
}

func TestFile_NoVersionHeadings_ShowsFirstLines(t *testing.T) {
	var b strings.Builder
	for i := range 60 {
		b.WriteString("- change ")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString("\n")
	}
	srv := serveFile(t, "text/plain", "\n\n"+b.String())

	cl, ok := NewFile().Get(context.Background(), model.Container{ChangelogFile: srv.URL + "/CHANGELOG"}, "latest", "latest")
	if !ok || cl.Entries != nil {
		t.Fatalf("got (%+v, %v)", cl, ok)
	}
	if n := strings.Count(cl.Raw, "\n") + 1; n != headLines || !strings.HasPrefix(cl.Raw, "- change a") {
		t.Errorf("raw has %d lines, starts %q", n, cl.Raw[:12])
	}
}

func TestFile_WebPageOrMissing_NotHandled(t *testing.T) {
	page := serveFile(t, "text/html; charset=utf-8", "<html>Version 1.0</html>")
	for _, u := range []string{page.URL + "/CHANGELOG", page.URL + "/missing"} {
		if cl, ok := NewFile().Get(context.Background(), model.Container{ChangelogFile: u}, "latest", "latest"); ok {
			t.Errorf("%s: got %+v, want unhandled", u, cl)
		}
	}
}

func TestFile_NoFileSet_NotHandled(t *testing.T) {
	if _, ok := NewFile().Get(context.Background(), model.Container{Source: "https://github.com/o/r"}, "1.0.0", "1.1.0"); ok {
		t.Fatal("a container without a changelog file must fall through")
	}
}

func TestFile_RevalidatesWithETag(t *testing.T) {
	hits, revalidated := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"v1"` {
			revalidated++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(historyTxt))
	}))
	t.Cleanup(srv.Close)

	f := NewFile()
	c := model.Container{ChangelogFile: srv.URL + "/History.txt"}
	f.Get(context.Background(), c, "latest", "latest")
	f.Get(context.Background(), c, "latest", "latest")
	if hits != 1 {
		t.Fatalf("a fresh copy was fetched again: %d hits", hits)
	}
	f.cache[c.ChangelogFile].fetchedAt = time.Now().Add(-time.Hour)
	cl, ok := f.Get(context.Background(), c, "latest", "latest")
	if !ok || revalidated != 1 || cl.Entries[0].Tag != "Version 2026.4" {
		t.Fatalf("hits %d, revalidated %d, ok %v", hits, revalidated, ok)
	}
}
