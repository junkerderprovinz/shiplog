package resolver

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// writeDockerConfig writes content as a config.json in a temp dir and returns
// its path.
func writeDockerConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestParseDockerConfig(t *testing.T) {
	cases := []struct {
		name string
		json string
		want map[string]registryLogin
	}{
		{
			name: "auth field, split on first colon",
			json: `{"auths":{"ghcr.io":{"auth":"` + b64("alice:pa:ss:word") + `"}}}`,
			want: map[string]registryLogin{"ghcr.io": {"alice", "pa:ss:word"}},
		},
		{
			name: "username and password fields",
			json: `{"auths":{"git.example.org":{"username":"bob","password":"hunter2"}}}`,
			want: map[string]registryLogin{"git.example.org": {"bob", "hunter2"}},
		},
		{
			name: "keys normalised: URL, case, path, port",
			json: `{"auths":{" HTTPS://Git.Example.ORG:5000/v2/ ":{"username":"u","password":"p"}}}`,
			want: map[string]registryLogin{"git.example.org:5000": {"u", "p"}},
		},
		{
			name: "every Docker Hub alias maps to the Hub registry host",
			json: `{"auths":{"https://index.docker.io/v1/":{"username":"u","password":"p"}}}`,
			want: map[string]registryLogin{dockerHubRegistry: {"u", "p"}},
		},
		{
			name: "Docker Hub aliases collide deterministically",
			json: `{"auths":{"registry-1.docker.io":{"username":"z","password":"z"},"docker.io":{"username":"a","password":"a"}}}`,
			want: map[string]registryLogin{dockerHubRegistry: {"a", "a"}},
		},
		{
			name: "empty placeholder (credsStore in use) skipped",
			json: `{"auths":{"ghcr.io":{}},"credsStore":"desktop"}`,
			want: nil,
		},
		{
			name: "identitytoken only skipped",
			json: `{"auths":{"ghcr.io":{"identitytoken":"abc"}}}`,
			want: nil,
		},
		{
			name: "credsStore and credHelpers without auths",
			json: `{"credsStore":"x","credHelpers":{"gcr.io":"gcloud"}}`,
			want: nil,
		},
		{
			name: "empty password skipped",
			json: `{"auths":{"ghcr.io":{"username":"u","password":""},"quay.io":{"auth":"` + b64("u:") + `"}}}`,
			want: nil,
		},
		{
			name: "bad base64 skipped",
			json: `{"auths":{"ghcr.io":{"auth":"!!!not base64!!!"}}}`,
			want: nil,
		},
		{
			name: "auth without colon skipped",
			json: `{"auths":{"ghcr.io":{"auth":"` + b64("nocolon") + `"}}}`,
			want: nil,
		},
		{
			name: "usable entries survive unusable neighbours",
			json: `{"auths":{"a.io":{},"b.io":{"username":"u","password":"p"}}}`,
			want: map[string]registryLogin{"b.io": {"u", "p"}},
		},
		{name: "malformed JSON", json: `{"auths":`, want: nil},
		{name: "empty file", json: ``, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDockerConfig([]byte(tc.json))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d logins, want %d", len(got), len(tc.want))
			}
			for host, w := range tc.want {
				if got[host] != w {
					t.Errorf("login for %q does not match the expected one", host)
				}
			}
		})
	}
}

func TestLoadDockerConfig_MissingAndUnreadable(t *testing.T) {
	if got := loadDockerConfig(filepath.Join(t.TempDir(), "absent.json")); len(got) != 0 {
		t.Errorf("missing file: got %d logins, want 0", len(got))
	}
	// A directory where the file should be is unreadable as a file.
	if got := loadDockerConfig(t.TempDir()); len(got) != 0 {
		t.Errorf("directory path: got %d logins, want 0", len(got))
	}
}

func TestWithDockerConfig_CountsLogins(t *testing.T) {
	p := writeDockerConfig(t, `{"auths":{"ghcr.io":{"username":"u","password":"p"},"x.io":{}}}`)
	r := New().WithDockerConfig(p)
	if r.DockerLogins() != 1 {
		t.Errorf("DockerLogins() = %d, want 1", r.DockerLogins())
	}
	if n := New().WithDockerConfig(filepath.Join(t.TempDir(), "nope")).DockerLogins(); n != 0 {
		t.Errorf("missing file: DockerLogins() = %d, want 0", n)
	}
}

// A stored login only ever goes to the registry's own realm (plus auth.docker.io
// for Docker Hub).
func TestLoginRealmOK(t *testing.T) {
	r := New() // production base URLs: https://<host>
	cases := []struct {
		name, host, realm string
		want              bool
	}{
		{"self-hosted own realm", "git.example.org", "https://git.example.org/v2/token?service=container_registry", true},
		{"own realm, host case-insensitive", "git.example.org", "https://GIT.example.org/token", true},
		{"own realm with port", "reg.example.org:5000", "https://reg.example.org:5000/token", true},
		{"other host", "git.example.org", "https://evil.example.net/token", false},
		{"suffix lookalike", "git.example.org", "https://git.example.org.evil.net/token", false},
		{"userinfo trick", "git.example.org", "https://git.example.org@evil.net/token", false},
		{"port differs", "git.example.org", "https://git.example.org:8443/token", false},
		{"scheme downgrade", "git.example.org", "http://git.example.org/token", false},
		{"relative realm", "git.example.org", "/token", false},
		{"garbage realm", "git.example.org", "::::", false},
		{"ghcr own realm", "ghcr.io", "https://ghcr.io/token", true},
		{"Docker Hub auth.docker.io", dockerHubRegistry, "https://auth.docker.io/token", true},
		{"Docker Hub own host", dockerHubRegistry, "https://registry-1.docker.io/token", true},
		{"Docker Hub lookalike", dockerHubRegistry, "https://auth.docker.io.evil.net/token", false},
		{"Docker Hub plain http", dockerHubRegistry, "http://auth.docker.io/token", false},
		{"auth.docker.io for another registry", "git.example.org", "https://auth.docker.io/token", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.loginRealmOK(tc.host, tc.realm); got != tc.want {
				t.Errorf("loginRealmOK(%q, %q) = %v, want %v", tc.host, tc.realm, got, tc.want)
			}
		})
	}
}
