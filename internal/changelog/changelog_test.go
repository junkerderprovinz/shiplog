package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// releasesJSON is a canned GitHub /releases response for tags v1.0.0..v1.3.0.
const releasesJSON = `[
  {"tag_name":"v1.3.0","body":"third feature","html_url":"https://github.com/o/r/releases/tag/v1.3.0","published_at":"2026-03-01T00:00:00Z"},
  {"tag_name":"v1.2.0","body":"second feature","html_url":"https://github.com/o/r/releases/tag/v1.2.0","published_at":"2026-02-01T00:00:00Z"},
  {"tag_name":"v1.1.0","body":"first feature","html_url":"https://github.com/o/r/releases/tag/v1.1.0","published_at":"2026-01-15T00:00:00Z"},
  {"tag_name":"v1.0.0","body":"initial","html_url":"https://github.com/o/r/releases/tag/v1.0.0","published_at":"2026-01-01T00:00:00Z"}
]`

// fakeGitHub returns an httptest server that serves releasesJSON at the
// expected releases path and 404 for anything else.
func fakeGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/releases" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(releasesJSON))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGitHub_Get_SpansAndOrders(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL

	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v1.3.0")
	if !ok {
		t.Fatalf("expected handled=true")
	}
	if cl == nil {
		t.Fatal("expected non-nil changelog")
	}
	if cl.Provider != "github" {
		t.Errorf("Provider = %q, want github", cl.Provider)
	}
	if len(cl.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(cl.Entries))
	}
	if cl.Entries[0].Tag != "v1.3.0" || cl.Entries[1].Tag != "v1.2.0" {
		t.Errorf("entries order = [%s, %s], want [v1.3.0, v1.2.0]", cl.Entries[0].Tag, cl.Entries[1].Tag)
	}
	if cl.SkippedCount != 1 {
		t.Errorf("SkippedCount = %d, want 1", cl.SkippedCount)
	}
	if cl.Entries[0].Body != "third feature" {
		t.Errorf("body not mapped: %q", cl.Entries[0].Body)
	}
	if cl.Entries[0].URL != "https://github.com/o/r/releases/tag/v1.3.0" {
		t.Errorf("html_url not mapped: %q", cl.Entries[0].URL)
	}
	if cl.Entries[0].PublishedAt.IsZero() {
		t.Error("PublishedAt not parsed")
	}
	wantCompare := "compare/v1.1.0...v1.3.0"
	if !contains(cl.URL, wantCompare) {
		t.Errorf("URL = %q, want substring %q", cl.URL, wantCompare)
	}
	if cl.Raw != "third feature\n\nsecond feature" {
		t.Errorf("Raw = %q", cl.Raw)
	}
	if cl.FromTag != "v1.1.0" || cl.ToTag != "v1.3.0" {
		t.Errorf("From/To = %q/%q", cl.FromTag, cl.ToTag)
	}
}

func TestGitHub_Get_UpToDate_ShowsCurrentNotes(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	// Up to date (from == to == running version): show that version's release notes.
	cl, ok := gh.Get(context.Background(), c, "v1.3.0", "v1.3.0")
	if !ok || cl == nil {
		t.Fatal("expected a handled changelog")
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.3.0" {
		t.Fatalf("expected 1 entry v1.3.0, got %#v", cl.Entries)
	}
	if cl.Raw != "third feature" {
		t.Errorf("Raw = %q, want 'third feature'", cl.Raw)
	}
	if cl.SkippedCount != 0 {
		t.Errorf("SkippedCount = %d, want 0", cl.SkippedCount)
	}
}

func TestGitHub_Get_UpToDate_LatestTagFallsBackToNewest(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	// A ":latest" container that's up to date: no release is named "latest", so
	// fall back to the newest release.
	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
	if !ok || cl == nil {
		t.Fatal("expected a handled changelog")
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.3.0" {
		t.Fatalf("expected the newest release v1.3.0, got %#v", cl.Entries)
	}
}

func TestGitHub_Get_DeprecatedWhenArchived(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r/releases":
			_, _ = w.Write([]byte(releasesJSON))
		case "/repos/o/r":
			_, _ = w.Write([]byte(`{"archived":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v1.3.0")
	if !ok || cl == nil {
		t.Fatal("expected a handled changelog")
	}
	if !cl.Deprecated {
		t.Error("expected Deprecated=true for an archived repo")
	}
}

func TestGitHub_Get_NonGitHubSource_NotHandled(t *testing.T) {
	gh := New("")
	c := model.Container{Source: "https://gitlab.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if ok || cl != nil {
		t.Fatalf("non-github source: got (%v, %v), want (nil, false)", cl, ok)
	}
}

func TestGitHub_Get_EmptySource_NotHandled(t *testing.T) {
	gh := New("")
	c := model.Container{Source: ""}
	cl, ok := gh.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if ok || cl != nil {
		t.Fatalf("empty source: got (%v, %v), want (nil, false)", cl, ok)
	}
}

func TestGitHub_Get_APIError_FallsThrough(t *testing.T) {
	// Server that 404s everything → non-200 → (nil, false).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if ok || cl != nil {
		t.Fatalf("api error: got (%v, %v), want (nil, false)", cl, ok)
	}
}

func TestGitHub_Get_GitHubButNoMatches_StillHandled(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	// Span above all releases → zero entries but still handled (don't fall through).
	cl, ok := gh.Get(context.Background(), c, "v9.0.0", "v9.9.9")
	if !ok {
		t.Fatalf("github repo with no matches must still be handled")
	}
	if cl == nil || len(cl.Entries) != 0 {
		t.Fatalf("expected non-nil changelog with 0 entries, got %#v", cl)
	}
	if cl.SkippedCount != 0 {
		t.Errorf("SkippedCount = %d, want 0", cl.SkippedCount)
	}
}

func TestGitHub_Get_TrailingGitAndSlash(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	for _, src := range []string{
		"https://github.com/o/r.git",
		"https://github.com/o/r/",
	} {
		c := model.Container{Source: src}
		cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v1.3.0")
		if !ok || cl == nil {
			t.Fatalf("source %q: expected handled changelog", src)
		}
		if len(cl.Entries) != 2 {
			t.Errorf("source %q: len(Entries) = %d, want 2", src, len(cl.Entries))
		}
	}
}

func TestFallback_AlwaysHandled(t *testing.T) {
	fb := Fallback{}
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := fb.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if !ok || cl == nil {
		t.Fatalf("fallback must always handle, got (%v, %v)", cl, ok)
	}
	if cl.Provider != "fallback" {
		t.Errorf("Provider = %q, want fallback", cl.Provider)
	}
	if cl.Raw != "" {
		t.Errorf("Raw = %q, want empty", cl.Raw)
	}
	if len(cl.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(cl.Entries))
	}
	if !contains(cl.URL, "compare/v1.0.0...v2.0.0") {
		t.Errorf("URL = %q, want github compare link", cl.URL)
	}
	if cl.FromTag != "v1.0.0" || cl.ToTag != "v2.0.0" {
		t.Errorf("From/To = %q/%q", cl.FromTag, cl.ToTag)
	}
}

func TestFallback_NonURLSource_EmptyURL(t *testing.T) {
	fb := Fallback{}
	c := model.Container{Source: ""}
	cl, ok := fb.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if !ok || cl == nil {
		t.Fatal("fallback must always handle")
	}
	if cl.URL != "" {
		t.Errorf("URL = %q, want empty for non-URL source", cl.URL)
	}
}

func TestChain_FallsThroughToFallback(t *testing.T) {
	// github pointed at a 404 server, empty Source → github not handled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL

	ch := Chain{gh, Fallback{}}
	c := model.Container{Source: ""} // github bails on non-github source
	cl, ok := ch.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if !ok || cl == nil {
		t.Fatalf("chain must be handled by fallback, got (%v, %v)", cl, ok)
	}
	if cl.Provider != "fallback" {
		t.Errorf("Provider = %q, want fallback", cl.Provider)
	}
}

func TestChain_PicksGitHubFirst(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL

	ch := Chain{gh, Fallback{}}
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := ch.Get(context.Background(), c, "v1.1.0", "v1.3.0")
	if !ok || cl == nil {
		t.Fatalf("chain must be handled, got (%v, %v)", cl, ok)
	}
	if cl.Provider != "github" {
		t.Errorf("Provider = %q, want github (first handler wins)", cl.Provider)
	}
}

func TestChain_NoneHandled(t *testing.T) {
	// A chain with only a github provider that bails → (nil, false).
	gh := New("")
	ch := Chain{gh}
	c := model.Container{Source: ""}
	cl, ok := ch.Get(context.Background(), c, "v1.0.0", "v2.0.0")
	if ok || cl != nil {
		t.Fatalf("no handler: got (%v, %v), want (nil, false)", cl, ok)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
