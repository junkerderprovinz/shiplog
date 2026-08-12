package cafeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Lookup: pure matching logic, no network ---

func feedFrom(entries []Entry, blacklisted map[string]string, previous *Feed) *Feed {
	byName := make(map[string][]Entry, len(entries))
	for _, e := range entries {
		n := normalizeName(e.Name)
		byName[n] = append(byName[n], e)
	}
	return &Feed{byName: byName, blacklisted: blacklisted, previous: previous}
}

func TestLookupListedAndHealthy(t *testing.T) {
	f := feedFrom([]Entry{{Name: "OpenCloud", Repository: "junkerderprovinz/opencloud"}}, nil, nil)
	res, ok := f.Lookup("OpenCloud", "docker.io/junkerderprovinz/opencloud", "")
	if !ok || !res.Listed || res.Deprecated {
		t.Fatalf("want listed/healthy/ok, got %+v ok=%v", res, ok)
	}
}

// Reproduces the real coppit/handbrake feed entry found live: Deprecated with
// a moderator comment, while still genuinely listed (not absent).
func TestLookupDeprecatedIsNotAbsent(t *testing.T) {
	f := feedFrom([]Entry{{
		Name: "HandBrake", Repository: "coppit/handbrake",
		Deprecated: true, ModeratorComment: "A better supported and more up to date app is available from DJoss",
	}}, nil, nil)
	res, ok := f.Lookup("HandBrake", "docker.io/coppit/handbrake", "")
	if !ok {
		t.Fatal("want ok")
	}
	if !res.Listed {
		t.Error("a deprecated (but not removed) app must still read Listed=true — it's an editorial demotion, not a dead end")
	}
	if !res.Deprecated || res.Note == "" {
		t.Errorf("want Deprecated=true with a note, got %+v", res)
	}
}

func TestLookupAbsentWithNoPreviousCrawlIsInconclusive(t *testing.T) {
	f := feedFrom([]Entry{{Name: "OtherApp", Repository: "x/y"}}, nil, nil)
	_, ok := f.Lookup("GoneApp", "x/gone", "")
	if ok {
		t.Fatal("an absence with no previous crawl to confirm against must be inconclusive (ok=false)")
	}
}

func TestLookupAbsentPresentInPreviousCrawlIsInconclusive(t *testing.T) {
	prev := feedFrom([]Entry{{Name: "FlakyApp", Repository: "x/flaky"}}, nil, nil)
	cur := feedFrom([]Entry{{Name: "OtherApp", Repository: "x/y"}}, nil, prev)
	_, ok := cur.Lookup("FlakyApp", "x/flaky", "")
	if ok {
		t.Fatal("present last crawl, absent this one, must be inconclusive — a transient crawl gap, not confirmed removal (the exact false-positive shape already fixed once for the raw-URL proxy)")
	}
}

func TestLookupAbsentConfirmedAcrossTwoCrawls(t *testing.T) {
	prev := feedFrom([]Entry{{Name: "OtherApp", Repository: "x/y"}}, nil, nil)
	cur := feedFrom([]Entry{{Name: "OtherApp", Repository: "x/y"}}, nil, prev)
	res, ok := cur.Lookup("TrulyGoneApp", "x/gone", "")
	if !ok || res.Listed {
		t.Fatalf("absent from both crawls must be confirmed not-listed, got %+v ok=%v", res, ok)
	}
}

func TestLookupBlacklisted(t *testing.T) {
	f := feedFrom(
		[]Entry{{Name: "BadApp", Repository: "x/bad"}},
		map[string]string{"x/bad": "Repository no longer exists on dockerHub"},
		nil,
	)
	res, ok := f.Lookup("BadApp", "docker.io/x/bad", "")
	if !ok || res.Listed || res.Note == "" {
		t.Fatalf("blacklisted must read not-listed with a reason, got %+v ok=%v", res, ok)
	}
}

// Two maintainers publish an app under the same display Name (the real
// HandBrake case) — must disambiguate by the container's own repository,
// not just grab the first match.
func TestLookupAmbiguousNameDisambiguatedByRepo(t *testing.T) {
	f := feedFrom([]Entry{
		{Name: "HandBrake", Repository: "coppit/handbrake", Deprecated: true, ModeratorComment: "superseded"},
		{Name: "HandBrake", Repository: "jlesage/handbrake"},
	}, nil, nil)
	res, ok := f.Lookup("HandBrake", "docker.io/jlesage/handbrake", "")
	if !ok {
		t.Fatal("want ok")
	}
	if res.Deprecated {
		t.Error("matched the wrong maintainer's entry — jlesage's is not deprecated, coppit's is")
	}
}

func TestLookupAmbiguousNameDisambiguatedByTemplateURL(t *testing.T) {
	f := feedFrom([]Entry{
		{Name: "SameName", Repository: "a/x", TemplateURL: "https://raw.githubusercontent.com/a/tpl/main/x.xml"},
		{Name: "SameName", Repository: "b/x", TemplateURL: "https://raw.githubusercontent.com/b/tpl/main/x.xml", Deprecated: true},
	}, nil, nil)
	// repo doesn't narrow it (empty), templateURL must.
	res, ok := f.Lookup("SameName", "", "https://raw.githubusercontent.com/b/tpl/main/x.xml")
	if !ok || !res.Deprecated {
		t.Fatalf("want the b/x entry via templateURL match, got %+v ok=%v", res, ok)
	}
}

func TestLookupAmbiguousUnresolvableIsInconclusive(t *testing.T) {
	f := feedFrom([]Entry{
		{Name: "SameName", Repository: "a/x"},
		{Name: "SameName", Repository: "b/x"},
	}, nil, nil)
	_, ok := f.Lookup("SameName", "docker.io/c/x", "")
	if ok {
		t.Fatal("neither candidate's repository matches the container's — must be inconclusive, not a guess")
	}
}

func TestNormalizeRepoStripsDockerHubDefaults(t *testing.T) {
	cases := map[string]string{
		"docker.io/library/redis":    "redis",
		"docker.io/coppit/handbrake": "coppit/handbrake",
		"ghcr.io/x/y":                "ghcr.io/x/y", // non-Docker-Hub passes through unchanged
	}
	for in, want := range cases {
		if got := normalizeRepo(in); got != want {
			t.Errorf("normalizeRepo(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- Fetcher: HTTP + on-disk cache behaviour ---

const miniFeedTS1 = `{"applist":[{"Name":"App","Repository":"x/app"}],"last_updated_timestamp":1}`
const miniFeedTS2 = `{"applist":[{"Name":"App","Repository":"x/app"},{"Name":"NewApp","Repository":"x/new"}],"last_updated_timestamp":2}`

func testServer(t *testing.T, feedBody string, ts string, fetchCalls *int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/lastUpdated", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"last_updated_timestamp":` + ts + `}`))
	})
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		if fetchCalls != nil {
			*fetchCalls++
		}
		_, _ = w.Write([]byte(feedBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetcherLoadFetchesOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	srv := testServer(t, miniFeedTS1, "1", nil)
	f := NewFetcher(dir)
	f.feedURL, f.lastUpdatedURL = srv.URL+"/feed", srv.URL+"/lastUpdated"

	feed, err := f.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := feed.Lookup("App", "x/app", ""); !ok {
		t.Fatal("freshly fetched feed should contain the seeded entry")
	}
	if _, err := os.Stat(filepath.Join(dir, cacheFile)); err != nil {
		t.Errorf("expected the feed to be cached to disk: %v", err)
	}
}

func TestFetcherLoadSkipsRefetchWhenTimestampUnchanged(t *testing.T) {
	dir := t.TempDir()
	var fullFetches int
	srv := testServer(t, miniFeedTS1, "1", &fullFetches)
	f := NewFetcher(dir)
	f.feedURL, f.lastUpdatedURL = srv.URL+"/feed", srv.URL+"/lastUpdated"

	if _, err := f.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fullFetches != 1 {
		t.Fatalf("want exactly 1 full-feed fetch across two Loads with an unchanged timestamp, got %d", fullFetches)
	}
}

func TestFetcherLoadRefetchesWhenTimestampChanges(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	ts := "1"
	body := miniFeedTS1
	mux.HandleFunc("/lastUpdated", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"last_updated_timestamp":` + ts + `}`))
	})
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	f := NewFetcher(dir)
	f.feedURL, f.lastUpdatedURL = srv.URL+"/feed", srv.URL+"/lastUpdated"

	if _, err := f.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	ts, body = "2", miniFeedTS2
	feed, err := f.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := feed.Lookup("NewApp", "x/new", ""); !ok {
		t.Fatal("second Load should have refetched and picked up the new entry")
	}
	// And the previous crawl must now be available for absence cross-checks.
	if feed.previous == nil {
		t.Fatal("expected the prior crawl to be cached as .previous after a refetch")
	}
}

func TestFetcherLoadFallsBackToCacheOnFetchFailure(t *testing.T) {
	dir := t.TempDir()
	srv := testServer(t, miniFeedTS1, "1", nil)
	f := NewFetcher(dir)
	f.feedURL, f.lastUpdatedURL = srv.URL+"/feed", srv.URL+"/lastUpdated"
	if _, err := f.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv.Close() // upstream now unreachable

	feed, err := f.Load(context.Background())
	if err != nil {
		t.Fatalf("a fetch failure with a cache on disk must degrade gracefully, not error: %v", err)
	}
	if _, ok := feed.Lookup("App", "x/app", ""); !ok {
		t.Fatal("fallback feed should still contain the previously cached entry")
	}
}

func TestFetcherLoadErrorsOnlyWhenNothingCachedAtAll(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	f := NewFetcher(dir)
	f.feedURL, f.lastUpdatedURL = srv.URL+"/feed", srv.URL+"/lastUpdated"

	_, err := f.Load(context.Background())
	if err == nil {
		t.Fatal("want an error: fetch failed AND nothing was ever cached — genuinely nothing to check against")
	}
	if !strings.Contains(err.Error(), "cafeed") {
		t.Errorf("error should be identifiable as coming from cafeed, got: %v", err)
	}
}
