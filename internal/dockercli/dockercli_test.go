package dockercli

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
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
	if immich.Digest != "sha256:1111111111111111111111111111111111111111111111111111111111111111" {
		t.Errorf("immich Digest = %q, want the ImageID sha", immich.Digest)
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
}

func TestSplitImageRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantRepo string
		wantTag  string
	}{
		{"redis", "docker.io/library/redis", "latest"},
		{"ubuntu:22.04", "docker.io/library/ubuntu", "22.04"},
		{"library/redis", "docker.io/library/redis", "latest"},
		{"library/redis:7", "docker.io/library/redis", "7"},
		{"myuser/myapp", "docker.io/myuser/myapp", "latest"},
		{"myuser/myapp:v2", "docker.io/myuser/myapp", "v2"},
		{"ghcr.io/imagegenius/immich:1.122.0", "ghcr.io/imagegenius/immich", "1.122.0"},
		{"ghcr.io/x/y", "ghcr.io/x/y", "latest"},
		{"registry:5000/team/app:dev", "registry:5000/team/app", "dev"},
		{"localhost/foo", "localhost/foo", "latest"},
		{"localhost:5000/foo:bar", "localhost:5000/foo", "bar"},
		{"docker.io/library/redis", "docker.io/library/redis", "latest"},
	}
	for _, tt := range tests {
		gotRepo, gotTag := splitImageRef(tt.ref)
		if gotRepo != tt.wantRepo || gotTag != tt.wantTag {
			t.Errorf("splitImageRef(%q) = (%q, %q), want (%q, %q)",
				tt.ref, gotRepo, gotTag, tt.wantRepo, tt.wantTag)
		}
	}
}
