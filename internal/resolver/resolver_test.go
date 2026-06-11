package resolver

import (
	"context"
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

	newestTag, sameTagDigest, err := r.Resolve(
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

func TestResolve_AnonymousTokenFlow(t *testing.T) {
	srv := newTestServer(true) // first call 401, must follow WWW-Authenticate
	defer srv.Close()

	r := newResolverFor(srv)

	newestTag, sameTagDigest, err := r.Resolve(
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

	newestTag, sameTagDigest, err := r.Resolve(
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
