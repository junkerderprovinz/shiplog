package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// releasesJSON is a canned GitHub /repos/o/r/releases response: a JSON ARRAY of
// releases, newest first, as the real API returns. Six entries so the "recent N"
// path (min(5, …)) is exercised as a real cap.
const releasesJSON = `[
{"tag_name":"v2.0.0","body":"sixth feature","html_url":"https://github.com/o/r/releases/tag/v2.0.0","published_at":"2026-06-01T00:00:00Z"},
{"tag_name":"v1.5.0","body":"fifth feature","html_url":"https://github.com/o/r/releases/tag/v1.5.0","published_at":"2026-05-01T00:00:00Z"},
{"tag_name":"v1.4.0","body":"fourth feature","html_url":"https://github.com/o/r/releases/tag/v1.4.0","published_at":"2026-04-01T00:00:00Z"},
{"tag_name":"v1.3.0","body":"third feature","html_url":"https://github.com/o/r/releases/tag/v1.3.0","published_at":"2026-03-01T00:00:00Z"},
{"tag_name":"v1.2.0","body":"second feature","html_url":"https://github.com/o/r/releases/tag/v1.2.0","published_at":"2026-02-01T00:00:00Z"},
{"tag_name":"v1.1.0","body":"first feature","html_url":"https://github.com/o/r/releases/tag/v1.1.0","published_at":"2026-01-01T00:00:00Z"}
]`

// fakeGitHub serves the releases LIST at /repos/o/r/releases and the repo object
// at /repos/o/r (archived:false); everything else 404s like the real API.
func fakeGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r/releases":
			_, _ = w.Write([]byte(releasesJSON))
		case "/repos/o/r":
			_, _ = w.Write([]byte(`{"archived":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A pinned version bump whose exact release exists shows THAT single release,
// Recent=false.
func TestGitHub_Get_ExactVersionMatch(t *testing.T) {
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
		t.Fatalf("expected 1 entry v1.2.0 (the exact match), got %#v", cl.Entries)
	}
	if cl.Recent {
		t.Error("Recent = true, want false for an exact version match")
	}
	if cl.Entries[0].Body != "second feature" || cl.Raw != "second feature" {
		t.Errorf("body/raw not mapped to the matched release: body=%q raw=%q", cl.Entries[0].Body, cl.Raw)
	}
	if cl.Entries[0].URL != "https://github.com/o/r/releases/tag/v1.2.0" {
		t.Errorf("html_url not mapped: %q", cl.Entries[0].URL)
	}
	if cl.Entries[0].PublishedAt.IsZero() {
		t.Error("PublishedAt not parsed")
	}
	if cl.URL != "https://github.com/o/r/releases" {
		t.Errorf("URL = %q, want repo releases link", cl.URL)
	}
	if cl.FromTag != "v1.1.0" || cl.ToTag != "v1.2.0" {
		t.Errorf("From/To = %q/%q", cl.FromTag, cl.ToTag)
	}
}

// The v-prefix is tolerated on both sides: a bare "1.2.0" toTag still matches a
// "v1.2.0" release exactly (Recent=false), not the recent-N path.
func TestGitHub_Get_ExactVersionMatch_VPrefixTolerant(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL

	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "1.1.0", "1.2.0") // no leading v
	if !ok || cl == nil {
		t.Fatalf("expected a handled changelog, got (%v, %v)", cl, ok)
	}
	if len(cl.Entries) != 1 || cl.Entries[0].Tag != "v1.2.0" || cl.Recent {
		t.Fatalf("expected exact match v1.2.0 with Recent=false, got %#v Recent=%v", cl.Entries, cl.Recent)
	}
}

// A rolling tag (":latest") has no version to match, so the recent N releases
// are shown (capped at 5), Recent=true, Raw = the newest release's body.
func TestGitHub_Get_RollingTag_ShowsRecent(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
	if !ok || cl == nil {
		t.Fatal("expected a handled changelog")
	}
	if !cl.Recent {
		t.Error("Recent = false, want true for a rolling tag")
	}
	if len(cl.Entries) != 5 {
		t.Fatalf("expected 5 recent entries (cap of 6 available), got %d: %#v", len(cl.Entries), cl.Entries)
	}
	if cl.Entries[0].Tag != "v2.0.0" {
		t.Errorf("newest-first ordering broken: entries[0] = %q, want v2.0.0", cl.Entries[0].Tag)
	}
	if cl.Raw != "sixth feature" {
		t.Errorf("Raw = %q, want the newest release body", cl.Raw)
	}
}

// A pinned version tag with no matching release falls to the recent N (not a
// single "latest"), Recent=true.
func TestGitHub_Get_NoMatchingRelease_ShowsRecent(t *testing.T) {
	srv := fakeGitHub(t)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	cl, ok := gh.Get(context.Background(), c, "v1.1.0", "v9.9.9") // no release v9.9.9
	if !ok || cl == nil {
		t.Fatalf("expected a handled changelog, got (%v, %v)", cl, ok)
	}
	if !cl.Recent {
		t.Error("Recent = false, want true when no release matches toTag")
	}
	if len(cl.Entries) != 5 || cl.Entries[0].Tag != "v2.0.0" {
		t.Fatalf("expected recent N starting at v2.0.0, got %#v", cl.Entries)
	}
}

// The releases LIST is cached: a second Get within the TTL makes NO new HTTP
// call, and after the TTL an ETag conditional GET → 304 keeps the cached data.
func TestGitHub_Get_CachesList_And304Revalidates(t *testing.T) {
	const etag = `"v1-list-etag"`
	var releasesCalls, archivedCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r/releases":
			releasesCalls++
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified) // unchanged: does not count against the rate limit
				return
			}
			w.Header().Set("ETag", etag)
			_, _ = w.Write([]byte(releasesJSON))
		case "/repos/o/r":
			archivedCalls++
			_, _ = w.Write([]byte(`{"archived":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}

	// First fetch: one releases call + one archived call, list cached with ETag.
	if _, ok := gh.Get(context.Background(), c, "latest", "latest"); !ok {
		t.Fatal("first Get: expected handled")
	}
	if releasesCalls != 1 || archivedCalls != 1 {
		t.Fatalf("after first Get: releasesCalls=%d archivedCalls=%d, want 1/1", releasesCalls, archivedCalls)
	}

	// Second Get within the TTL: served entirely from cache, no new HTTP calls.
	if _, ok := gh.Get(context.Background(), c, "latest", "latest"); !ok {
		t.Fatal("second Get: expected handled")
	}
	if releasesCalls != 1 || archivedCalls != 1 {
		t.Fatalf("within TTL a second Get must not touch the API: releasesCalls=%d archivedCalls=%d, want 1/1", releasesCalls, archivedCalls)
	}

	// Force the cache stale: the next Get revalidates with If-None-Match and gets
	// a 304, so the releases endpoint is hit again but the archived one is not,
	// and the cached data is still served.
	gh.ttl = 0
	cl, ok := gh.Get(context.Background(), c, "latest", "latest")
	if !ok || cl == nil {
		t.Fatal("third Get: expected handled")
	}
	if releasesCalls != 2 {
		t.Errorf("after TTL expiry the list must be revalidated: releasesCalls=%d, want 2", releasesCalls)
	}
	if archivedCalls != 1 {
		t.Errorf("a 304 must not rebuild the cache (no new archived call): archivedCalls=%d, want 1", archivedCalls)
	}
	if len(cl.Entries) != 5 || cl.Entries[0].Tag != "v2.0.0" {
		t.Errorf("304 must keep the cached data, got %#v", cl.Entries)
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
	// Server that 404s everything → non-200, not rate-limited, no cache → (nil, false).
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

// A github repo whose releases list is empty ([]) must fall through so the
// version-delta Fallback handles it, rather than returning an empty github
// changelog that would shadow the Fallback.
func TestGitHub_Get_EmptyReleases_FallsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r/releases":
			_, _ = w.Write([]byte(`[]`)) // repo exists but has cut no releases
		case "/repos/o/r":
			_, _ = w.Write([]byte(`{"archived":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	gh := New("")
	gh.baseURL = srv.URL
	c := model.Container{Source: "https://github.com/o/r"}
	cl, ok := gh.Get(context.Background(), c, "v9.0.0", "v9.9.9")
	if ok || cl != nil {
		t.Fatalf("empty releases: got (%v, %v), want (nil, false)", cl, ok)
	}
}

// A 403 with X-RateLimit-Remaining: 0 and NO prior cache surfaces an honest
// RateLimited changelog (handled=true) so the UI shows a note instead of a blank.
func TestGitHub_Get_RateLimited_NoCache_Handled(t *testing.T) {
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
	if !ok || cl == nil {
		t.Fatalf("rate-limited with no cache: got (%v, %v), want a handled changelog", cl, ok)
	}
	if !cl.RateLimited {
		t.Error("RateLimited = false, want true")
	}
	if cl.Provider != "github" {
		t.Errorf("Provider = %q, want github", cl.Provider)
	}
	if cl.URL != "https://github.com/o/r/releases" {
		t.Errorf("URL = %q, want repo releases link", cl.URL)
	}
	if len(cl.Entries) != 0 {
		t.Errorf("expected no entries in a rate-limit note, got %#v", cl.Entries)
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
		if len(cl.Entries) == 0 || cl.Entries[0].Tag != "v2.0.0" {
			t.Errorf("source %q: entries = %#v, want newest-first starting at v2.0.0", src, cl.Entries)
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
	if cl.URL != "https://github.com/o/r/releases" {
		t.Errorf("URL = %q, want github releases link", cl.URL)
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
