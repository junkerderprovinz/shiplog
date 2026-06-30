package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// latestJSON / tagJSON are canned GitHub single-release responses. The new
// provider fetches the TARGET release (one call): /releases/tags/<tag> for a
// pinned version, else /releases/latest for a rolling tag.
const (
	latestJSON = `{"tag_name":"v1.3.0","body":"third feature","html_url":"https://github.com/o/r/releases/tag/v1.3.0","published_at":"2026-03-01T00:00:00Z"}`
	tagJSON    = `{"tag_name":"v1.2.0","body":"second feature","html_url":"https://github.com/o/r/releases/tag/v1.2.0","published_at":"2026-02-01T00:00:00Z"}`
)

// fakeGitHub serves /releases/latest, /releases/tags/v1.2.0 and the repo object;
// everything else (incl. unknown tags) 404s like the real API.
func fakeGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			_, _ = w.Write([]byte(latestJSON))
		case "/repos/o/r/releases/tags/v1.2.0":
			_, _ = w.Write([]byte(tagJSON))
		case "/repos/o/r":
			_, _ = w.Write([]byte(`{"archived":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// An update to a pinned target tag shows THAT release's notes in a single call.
func TestGitHub_Get_ShowsTargetReleaseNotes(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL

	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v1.2.0")
	if !ok || cl == nil {
		t.Fatalf("expected a handled changelog, got (%v, %v)", cl, ok)
	}
	if cl.Provider != "github" {
		t.Errorf("Provider = %q, want github", cl.Provider)
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.2.0" {
		t.Fatalf("expected 1 entry v1.2.0 (the target), got %#v", cl.Entries)
	}
	if cl.SkippedCount != 0 {
		t.Errorf("SkippedCount = %d, want 0 (only the target release is shown)", cl.SkippedCount)
	}
	if cl.Entries[0].Body != "second feature" || cl.Raw != "second feature" {
		t.Errorf("body/raw not mapped to the target release: body=%q raw=%q", cl.Entries[0].Body, cl.Raw)
	}
	if cl.Entries[0].URL != "https://github.com/o/r/releases/tag/v1.2.0" {
		t.Errorf("html_url not mapped: %q", cl.Entries[0].URL)
	}
	if cl.Entries[0].PublishedAt.IsZero() {
		t.Error("PublishedAt not parsed")
	}
	if cl.FromTag != "v1.1.0" || cl.ToTag != "v1.2.0" {
		t.Errorf("From/To = %q/%q", cl.FromTag, cl.ToTag)
	}
}

// A pinned target with no release of its own falls back to the latest release
// (still a single successful fetch after the 404 on the exact tag).
func TestGitHub_Get_PinnedTagWithoutOwnRelease_UsesLatest(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v1.9.9") // no release v1.9.9
	if !ok || cl == nil {
		t.Fatalf("expected a handled changelog, got (%v, %v)", cl, ok)
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.3.0" {
		t.Fatalf("expected fallback to latest v1.3.0, got %#v", cl.Entries)
	}
}

func TestGitHub_Get_RollingTag_UsesLatestRelease(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	// A ":latest" container: the target tag is non-version, so go straight to the
	// repo's latest release (one call).
	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
	if !ok || cl == nil {
		t.Fatal("expected a handled changelog")
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.3.0" {
		t.Fatalf("expected the latest release v1.3.0, got %#v", cl.Entries)
	}
}

func TestGitHub_Get_DeprecatedWhenArchived(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			_, _ = w.Write([]byte(latestJSON))
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
	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
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

// A github repo whose target/latest both 404 (no releases at all) must fall
// through so the version-delta Fallback handles it, rather than returning an
// empty github changelog that would shadow the Fallback.
func TestGitHub_Get_NoUsableRelease_FallsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // no /releases/latest, no /releases/tags/*
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v9.0.0", "v9.9.9")
	if ok || cl != nil {
		t.Fatalf("no usable release: got (%v, %v), want (nil, false)", cl, ok)
	}
}

func TestGitHub_Get_RateLimited_FallsThrough(t *testing.T) {
	// A 403 with X-RateLimit-Remaining: 0 is GitHub's anonymous rate-limit
	// response: the provider must fall through (so the chain reaches Fallback).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
	if ok || cl != nil {
		t.Fatalf("rate-limited: got (%v, %v), want (nil, false)", cl, ok)
	}
}

// A pinned target whose exact-release fetch returns a 429 (rate limit, not a
// 404) is a real failure: it must NOT silently fall back to the latest release,
// it must fall through so the Fallback's version-delta wins.
func TestGitHub_Get_TargetTag429_FallsThrough(t *testing.T) {
	var latestCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			latestCalls++
			http.NotFound(w, r)
		default: // the exact-tag endpoint is rate limited
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v1.2.0")
	if ok || cl != nil {
		t.Fatalf("target 429: got (%v, %v), want (nil, false)", cl, ok)
	}
	if latestCalls != 0 {
		t.Errorf("a 429 on the exact tag must not fall back to /releases/latest (called %d×)", latestCalls)
	}
}

func TestIsRateLimited(t *testing.T) {
	mk := func(code int, remaining string) *http.Response {
		h := http.Header{}
		if remaining != "" {
			h.Set("X-RateLimit-Remaining", remaining)
		}
		return &http.Response{StatusCode: code, Header: h}
	}
	cases := []struct {
		name string
		resp *http.Response
		want bool
	}{
		{"403 with remaining 0 is rate limited", mk(http.StatusForbidden, "0"), true},
		{"429 with remaining 0 is rate limited", mk(http.StatusTooManyRequests, "0"), true},
		{"403 with remaining left is not rate limited", mk(http.StatusForbidden, "12"), false},
		{"403 without the header is not rate limited", mk(http.StatusForbidden, ""), false},
		{"404 is never rate limited", mk(http.StatusNotFound, "0"), false},
	}
	for _, c := range cases {
		if got := isRateLimited(c.resp); got != c.want {
			t.Errorf("%s: isRateLimited = %v, want %v", c.name, got, c.want)
		}
	}
}

// A repo with no releases must NOT spend the second (archived) GitHub call: it
// falls through to the Fallback before the archived check is ever reached.
func TestGitHub_Get_NoReleases_SkipsArchivedCall(t *testing.T) {
	var archivedCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r":
			archivedCalls++
			_, _ = w.Write([]byte(`{"archived":false}`))
		default:
			http.NotFound(w, r) // no release endpoints
		}
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
	if ok || cl != nil {
		t.Fatalf("no releases: got (%v, %v), want (nil, false)", cl, ok)
	}
	if archivedCalls != 0 {
		t.Errorf("archived endpoint called %d time(s), want 0 (no release → no second call)", archivedCalls)
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
		cl, ok := gh.Get(context.Background(), c, "latest", "latest")
		if !ok || cl == nil {
			t.Fatalf("source %q: expected handled changelog", src)
		}
		if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.3.0" {
			t.Errorf("source %q: entries = %#v, want [v1.3.0]", src, cl.Entries)
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
