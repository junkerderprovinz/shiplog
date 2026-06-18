package resolver

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer builds a fake OCI registry v2 backend. When requireAuth is
// true the registry rejects unauthenticated requests with a 401 carrying a
// WWW-Authenticate header that points the resolver at this server's /token
// endpoint; the second (bearer-authenticated) attempt then succeeds.
func newTestServer(requireAuth bool) *httptest.Server {
	mux := http.NewServeMux()

	// Anonymous bearer token endpoint.
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"x"}`))
	})

	// Tags list.
	mux.HandleFunc("/v2/lib/app/tags/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"lib/app","tags":["1.0.0","1.2.0","1.1.0","latest","nightly"]}`))
	})

	// Manifest HEAD for tag 1.0.0 → digest sha256:NEW.
	mux.HandleFunc("/v2/lib/app/manifests/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:NEW")
		w.WriteHeader(http.StatusOK)
	})

	// Manifest HEAD for the newest semver tag 1.2.0 → digest sha256:NEWEST.
	mux.HandleFunc("/v2/lib/app/manifests/1.2.0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:NEWEST")
		w.WriteHeader(http.StatusOK)
	})

	// Manifest HEAD for tag latest → digest sha256:LATEST.
	mux.HandleFunc("/v2/lib/app/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:LATEST")
		w.WriteHeader(http.StatusOK)
	})

	if !requireAuth {
		return httptest.NewServer(mux)
	}

	// Wrap the mux: every /v2/ request must carry a bearer token, otherwise
	// reply 401 + WWW-Authenticate. /token and /v2/ (version check) stay open.
	srv := httptest.NewServer(nil)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/lib/app/") && r.Header.Get("Authorization") == "" {
			realm := srv.URL + "/token"
			w.Header().Set("WWW-Authenticate",
				`Bearer realm="`+realm+`",service="registry.test",scope="repository:lib/app:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
	return srv
}

// newResolverFor returns a Resolver whose baseURL maps every host to the test
// server, so the resolver talks only to the fake registry.
func newResolverFor(srv *httptest.Server) *Resolver {
	r := New()
	r.baseURL = func(host string) string { return srv.URL }
	return r
}

func TestResolve_NewestTagAndDigest(t *testing.T) {
	srv := newTestServer(false)
	defer srv.Close()

	r := newResolverFor(srv)

	newestTag, sameTagDigest, newestVerTag, newestVerDigest, err := r.Resolve(
		context.Background(), "docker.io/lib/app", "1.0.0", "sha256:OLD")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if newestTag != "1.2.0" {
		t.Errorf("newestTag = %q, want %q", newestTag, "1.2.0")
	}
	if sameTagDigest != "sha256:NEW" {
		t.Errorf("sameTagDigest = %q, want %q", sameTagDigest, "sha256:NEW")
	}
	if newestVerTag != "1.2.0" {
		t.Errorf("newestVerTag = %q, want %q", newestVerTag, "1.2.0")
	}
	// Pinned semver tag: the proof-digest fetch is skipped, so it stays empty.
	if newestVerDigest != "" {
		t.Errorf("newestVerDigest = %q, want empty for a pinned tag", newestVerDigest)
	}
}

func TestResolve_AnonymousTokenFlow(t *testing.T) {
	srv := newTestServer(true) // first call 401, must follow WWW-Authenticate
	defer srv.Close()

	r := newResolverFor(srv)

	newestTag, sameTagDigest, _, _, err := r.Resolve(
		context.Background(), "docker.io/lib/app", "1.0.0", "sha256:OLD")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if newestTag != "1.2.0" {
		t.Errorf("newestTag = %q, want %q", newestTag, "1.2.0")
	}
	if sameTagDigest != "sha256:NEW" {
		t.Errorf("sameTagDigest = %q, want %q", sameTagDigest, "sha256:NEW")
	}
}

func TestResolve_NonSemverTag(t *testing.T) {
	srv := newTestServer(false)
	defer srv.Close()

	r := newResolverFor(srv)

	newestTag, sameTagDigest, newestVerTag, newestVerDigest, err := r.Resolve(
		context.Background(), "docker.io/lib/app", "latest", "sha256:OLD")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if newestTag != "latest" {
		t.Errorf("newestTag = %q, want %q", newestTag, "latest")
	}
	if sameTagDigest != "sha256:LATEST" {
		t.Errorf("sameTagDigest = %q, want %q", sameTagDigest, "sha256:LATEST")
	}
	// A rolling tag still resolves the newest version AND its digest, so the
	// engine can prove which version a :latest container is running.
	if newestVerTag != "1.2.0" {
		t.Errorf("newestVerTag = %q, want %q", newestVerTag, "1.2.0")
	}
	if newestVerDigest != "sha256:NEWEST" {
		t.Errorf("newestVerDigest = %q, want %q", newestVerDigest, "sha256:NEWEST")
	}
}

func TestResolve_FollowsTagPagination(t *testing.T) {
	// Page 1 returns old tags + a Link to page 2; page 2 has the newer tags.
	// Reading only page 1 would pick 0.59; following the Link finds 1.8.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/lib/app/tags/list" && r.URL.Query().Get("last") == "":
			w.Header().Set("Link", `</v2/lib/app/tags/list?n=500&last=0.59>; rel="next"`)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":["0.58","0.59"]}`))
		case r.URL.Path == "/v2/lib/app/tags/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":["1.7","1.8"]}`)) // page 2, no Link → stop
		case strings.HasPrefix(r.URL.Path, "/v2/lib/app/manifests/"):
			w.Header().Set("Docker-Content-Digest", "sha256:X")
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := newResolverFor(srv)
	newest, _, _, _, err := r.Resolve(context.Background(), "docker.io/lib/app", "1.7", "sha256:o")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if newest != "1.8" {
		t.Errorf("newestTag = %q, want 1.8 (must follow pagination to page 2)", newest)
	}
}

// When Docker Hub credentials are configured, the token request for a docker.io
// image must carry HTTP Basic auth (so the issued token has the authenticated,
// higher rate limit). For other registries, no credentials must leak.
func TestResolve_DockerHubBasicAuthOnToken(t *testing.T) {
	var gotTokenAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		gotTokenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"x"}`))
	})
	mux.HandleFunc("/v2/library/app/tags/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tags":["1.0.0","1.2.0","latest"]}`))
	})
	mux.HandleFunc("/v2/library/app/manifests/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:Z")
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(nil)
	defer srv.Close()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/library/app/") && r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate",
				`Bearer realm="`+srv.URL+`/token",service="registry.docker.io",scope="repository:library/app:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})

	r := newResolverFor(srv).WithDockerHubAuth("alice", "dh-secret")
	// "docker.io/library/app" → host registry-1.docker.io → Basic auth expected.
	if _, _, _, _, err := r.Resolve(context.Background(), "docker.io/library/app", "latest", ""); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:dh-secret"))
	if gotTokenAuth != want {
		t.Errorf("token request Authorization = %q, want %q", gotTokenAuth, want)
	}
}

func TestNewestSemver_AcceptsTwoPartTags(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{[]string{"0.16", "0.17", "0.18", "latest"}, "0.18"},                // OpenHands-style two-part tags
		{[]string{"0.17.5", "0.18", "0.16.9"}, "0.18"},                      // a two-part tag beats an older three-part one
		{[]string{"1.0.0", "1.2.0", "1.1.0", "latest", "nightly"}, "1.2.0"}, // three-part still works
		{[]string{"1.8.0-rc1", "1.7.9"}, "1.7.9"},                           // prerelease never ranks as newest
		{[]string{"latest", "nightly", "main"}, ""},                         // nothing semver
	}
	for _, c := range cases {
		if got := newestSemver(c.tags); got != c.want {
			t.Errorf("newestSemver(%v) = %q, want %q", c.tags, got, c.want)
		}
	}
}

func TestSplitRepo(t *testing.T) {
	cases := []struct {
		repo     string
		wantHost string
		wantPath string
	}{
		{"docker.io/library/redis", "registry-1.docker.io", "library/redis"},
		{"docker.io/lib/app", "registry-1.docker.io", "lib/app"},
		{"ghcr.io/x/y", "ghcr.io", "x/y"},
		{"myreg.example.com/ns/name", "myreg.example.com", "ns/name"},
	}
	for _, c := range cases {
		host, path := splitRepo(c.repo)
		if host != c.wantHost || path != c.wantPath {
			t.Errorf("splitRepo(%q) = (%q, %q), want (%q, %q)",
				c.repo, host, path, c.wantHost, c.wantPath)
		}
	}
}
