package dockercli

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

const (
	immichManifest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	redisManifest  = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// cannedContainersJSON is a trimmed but realistic /containers/json?all=1 body
// from a modern engine: Labels are present in the list response, so a single
// GET is enough to resolve everything we need.
const cannedContainersJSON = `[
  {
    "Id": "a1b2c3d4immich",
    "Names": ["/immich"],
    "Image": "ghcr.io/imagegenius/immich:1.122.0",
    "ImageID": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
    "State": "running",
    "Status": "Up 3 hours",
    "Labels": {
      "org.opencontainers.image.source": "https://github.com/immich-app/immich",
      "some.other.label": "ignored"
    }
  },
  {
    "Id": "e5f6g7h8redis",
    "Names": ["/redis"],
    "Image": "redis",
    "ImageID": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
    "State": "exited",
    "Status": "Exited (0) 5 minutes ago",
    "Labels": {}
  }
]`

func TestListOverUnixSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "d.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		// Go 1.26 on Windows supports AF_UNIX, but a host may still deny it;
		// skip so CI on Linux still exercises this test.
		t.Skipf("AF_UNIX unavailable on this host (net.Listen unix failed): %v", err)
	}

	mux := http.NewServeMux()
	var gotMethod, gotPath, gotQuery string
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cannedContainersJSON))
	})
	// Image inspect supplies the registry manifest digest (RepoDigests) and the
	// image's own OCI labels (Config.Labels) — immich declares its version, redis
	// declares none.
	mux.HandleFunc("/v1.43/images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "1111") {
			_, _ = w.Write([]byte(`{"RepoDigests":["ghcr.io/imagegenius/immich@` + immichManifest + `"],` +
				`"Config":{"Labels":{"org.opencontainers.image.version":"1.122.0"}}}`))
		} else {
			_, _ = w.Write([]byte(`{"RepoDigests":["redis@` + redisManifest + `"]}`))
		}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	c := New(sock)
	c.baseURL = "http://docker/v1.43" // default, made explicit for clarity

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("request method = %q, want GET (client must be read-only)", gotMethod)
	}
	if gotPath != "/v1.43/containers/json" {
		t.Errorf("request path = %q, want /v1.43/containers/json", gotPath)
	}
	if gotQuery != "all=1" {
		t.Errorf("request query = %q, want all=1", gotQuery)
	}

	if len(got) != 2 {
		t.Fatalf("List() returned %d containers, want 2", len(got))
	}

	// Find by name so we don't depend on slice order.
	byName := map[string]model.Container{}
	for _, ctr := range got {
		byName[ctr.Name] = ctr
	}

	immich, ok := byName["immich"]
	if !ok {
		t.Fatalf("no container named %q (got %v)", "immich", byName)
	}
	if immich.Name != "immich" {
		t.Errorf("immich Name = %q, want immich (leading slash must be stripped)", immich.Name)
	}
	if immich.Repo != "ghcr.io/imagegenius/immich" {
		t.Errorf("immich Repo = %q, want ghcr.io/imagegenius/immich", immich.Repo)
	}
	if immich.Tag != "1.122.0" {
		t.Errorf("immich Tag = %q, want 1.122.0", immich.Tag)
	}
	if immich.Source != "https://github.com/immich-app/immich" {
		t.Errorf("immich Source = %q, want the github URL", immich.Source)
	}
	if immich.State != "running" {
		t.Errorf("immich State = %q, want running", immich.State)
	}
	if immich.Digest != immichManifest {
		t.Errorf("immich Digest = %q, want the manifest digest %q", immich.Digest, immichManifest)
	}
	if immich.ImageVersion != "1.122.0" {
		t.Errorf("immich ImageVersion = %q, want 1.122.0 (from the OCI version label)", immich.ImageVersion)
	}

	redis, ok := byName["redis"]
	if !ok {
		t.Fatalf("no container named %q (got %v)", "redis", byName)
	}
	if redis.Repo != "docker.io/library/redis" {
		t.Errorf("redis Repo = %q, want docker.io/library/redis", redis.Repo)
	}
	if redis.Tag != "latest" {
		t.Errorf("redis Tag = %q, want latest (default)", redis.Tag)
	}
	if redis.Source != "" {
		t.Errorf("redis Source = %q, want empty (no source label)", redis.Source)
	}
	if redis.State != "exited" {
		t.Errorf("redis State = %q, want exited", redis.State)
	}
	if redis.Digest != redisManifest {
		t.Errorf("redis Digest = %q, want the manifest digest %q", redis.Digest, redisManifest)
	}
	if redis.ImageVersion != "" {
		t.Errorf("redis ImageVersion = %q, want empty (image declares no version label)", redis.ImageVersion)
	}
}

func TestImageVersion(t *testing.T) {
	mk := func(cfg, ctr map[string]string) dockerImage {
		var img dockerImage
		img.Config.Labels = cfg
		img.ContainerConfig.Labels = ctr
		return img
	}
	cases := []struct {
		name string
		img  dockerImage
		want string
	}{
		{"version label wins",
			mk(map[string]string{
				"org.opencontainers.image.version":  "2.7.2",
				"org.opencontainers.image.revision": "deadbeef",
			}, nil), "2.7.2"},
		{"revision is the fallback when version is absent",
			mk(map[string]string{"org.opencontainers.image.revision": "deadbeef"}, nil), "deadbeef"},
		{"legacy ContainerConfig labels are read too",
			mk(nil, map[string]string{"org.opencontainers.image.version": "1.0.0"}), "1.0.0"},
		{"Config takes precedence over ContainerConfig",
			mk(map[string]string{"org.opencontainers.image.version": "2.0.0"},
				map[string]string{"org.opencontainers.image.version": "1.0.0"}), "2.0.0"},
		{"no labels yields empty",
			mk(nil, nil), ""},
	}
	for _, c := range cases {
		if got := imageVersion(c.img); got != c.want {
			t.Errorf("%s: imageVersion = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPickDigest(t *testing.T) {
	cases := []struct {
		name        string
		repoDigests []string
		repo        string
		want        string
	}{
		{"matches the container repo",
			[]string{"ghcr.io/x/y@sha256:aaa", "docker.io/library/redis@sha256:bbb"},
			"docker.io/library/redis", "sha256:bbb"},
		{"short docker hub name normalises and matches",
			[]string{"redis@sha256:bbb"}, "docker.io/library/redis", "sha256:bbb"},
		{"no match falls back to the first digest",
			[]string{"ghcr.io/x/y@sha256:aaa"}, "docker.io/library/redis", "sha256:aaa"},
		{"empty list yields empty (locally-built image)",
			nil, "docker.io/library/redis", ""},
	}
	for _, c := range cases {
		if got := pickDigest(c.repoDigests, c.repo); got != c.want {
			t.Errorf("%s: pickDigest = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestGhcrSource(t *testing.T) {
	cases := map[string]string{
		"ghcr.io/open-webui/open-webui": "https://github.com/open-webui/open-webui",
		"ghcr.io/x/y":                   "https://github.com/x/y",
		"ghcr.io/x/y/z":                 "https://github.com/x/y", // extra path dropped
		"docker.io/library/redis":       "",                       // not ghcr
		"ghcr.io/onlyowner":             "",                       // no repo segment
	}
	for repo, want := range cases {
		if got := ghcrSource(repo); got != want {
			t.Errorf("ghcrSource(%q) = %q, want %q", repo, got, want)
		}
	}
}

func TestSplitImageRef(t *testing.T) {
	const dig = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	tests := []struct {
		ref        string
		wantRepo   string
		wantTag    string
		wantPinned string
	}{
		{"redis", "docker.io/library/redis", "latest", ""},
		{"ubuntu:22.04", "docker.io/library/ubuntu", "22.04", ""},
		{"library/redis", "docker.io/library/redis", "latest", ""},
		{"library/redis:7", "docker.io/library/redis", "7", ""},
		{"myuser/myapp", "docker.io/myuser/myapp", "latest", ""},
		{"myuser/myapp:v2", "docker.io/myuser/myapp", "v2", ""},
		{"ghcr.io/imagegenius/immich:1.122.0", "ghcr.io/imagegenius/immich", "1.122.0", ""},
		{"ghcr.io/x/y", "ghcr.io/x/y", "latest", ""},
		{"registry:5000/team/app:dev", "registry:5000/team/app", "dev", ""},
		{"localhost/foo", "localhost/foo", "latest", ""},
		{"localhost:5000/foo:bar", "localhost:5000/foo", "bar", ""},
		{"docker.io/library/redis", "docker.io/library/redis", "latest", ""},

		// Digest-pinned refs: the digest must be split off BEFORE tag detection
		// (the colon inside "sha256:…" is not a tag separator), and a pin without
		// an explicit tag has NO tag, not "latest".
		{"redis@" + dig, "docker.io/library/redis", "", dig},
		{"ghcr.io/x/y@" + dig, "ghcr.io/x/y", "", dig},
		{"ghcr.io/x/y:1.2.3@" + dig, "ghcr.io/x/y", "1.2.3", dig},
		{"registry:5000/team/app@" + dig, "registry:5000/team/app", "", dig},

		// Bare image-ID refs name no repo at all.
		{dig, "", "", ""},
		{strings.TrimPrefix(dig, "sha256:"), "", "", ""},
	}
	for _, tt := range tests {
		gotRepo, gotTag, gotPinned := splitImageRef(tt.ref)
		if gotRepo != tt.wantRepo || gotTag != tt.wantTag || gotPinned != tt.wantPinned {
			t.Errorf("splitImageRef(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.ref, gotRepo, gotTag, gotPinned, tt.wantRepo, tt.wantTag, tt.wantPinned)
		}
	}
}

func TestAllDigests(t *testing.T) {
	got := allDigests([]string{
		"ghcr.io/x/y@sha256:aaa",
		"mirror.example.com/x/y@sha256:bbb",
		"no-digest-entry",
	})
	want := []string{"sha256:aaa", "sha256:bbb"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("allDigests = %v, want %v", got, want)
	}
}
