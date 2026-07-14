package sources

import "testing"

func TestResolvePrecedence(t *testing.T) {
	ov := map[string]string{"lscr.io/linuxserver/radarr": "https://github.com/me/fork"}
	tests := []struct {
		name, repo, oci, wantSrc, wantKind string
	}{
		{"override wins over curated + oci", "lscr.io/linuxserver/radarr", "https://github.com/linuxserver/docker-radarr", "https://github.com/me/fork", KindOverride},
		{"curated wins over oci", "lscr.io/linuxserver/sonarr", "https://github.com/linuxserver/docker-sonarr", "https://github.com/Sonarr/Sonarr", KindCurated},
		{"curated matches docker.io host", "docker.io/linuxserver/lidarr", "", "https://github.com/Lidarr/Lidarr", KindCurated},
		{"curated matches ghcr host", "ghcr.io/linuxserver/prowlarr", "", "https://github.com/Prowlarr/Prowlarr", KindCurated},
		{"unknown lsio app falls through to oci", "lscr.io/linuxserver/plex", "https://github.com/linuxserver/docker-plex", "https://github.com/linuxserver/docker-plex", KindOCI},
		{"non-lsio keeps oci", "ghcr.io/hotio/seerr", "https://github.com/hotio/seerr", "https://github.com/hotio/seerr", KindOCI},
		{"empty repo can't be keyed", "", "https://github.com/x/y", "https://github.com/x/y", KindOCI},
		{"blank override ignored", "lscr.io/linuxserver/bazarr", "https://github.com/linuxserver/docker-bazarr", "https://github.com/morpheus65535/bazarr", KindCurated},
	}
	// add a blank override to prove it's skipped (falls to curated)
	ov["lscr.io/linuxserver/bazarr"] = "  "
	for _, tt := range tests {
		src, kind := Resolve(tt.repo, tt.oci, ov)
		if src != tt.wantSrc || kind != tt.wantKind {
			t.Errorf("%s: Resolve(%q,%q) = (%q,%q); want (%q,%q)", tt.name, tt.repo, tt.oci, src, kind, tt.wantSrc, tt.wantKind)
		}
	}
}

func TestNormalizeGitHubSource(t *testing.T) {
	ok := map[string]string{
		"Radarr/Radarr":                                 "https://github.com/Radarr/Radarr",
		"github.com/Sonarr/Sonarr":                      "https://github.com/Sonarr/Sonarr",
		"https://github.com/Lidarr/Lidarr":              "https://github.com/Lidarr/Lidarr",
		"https://github.com/Prowlarr/Prowlarr/releases": "https://github.com/Prowlarr/Prowlarr",
		"https://github.com/me/tool.git":                "https://github.com/me/tool",
		"git@github.com:me/tool.git":                    "https://github.com/me/tool",
		"  Owner/Repo  ":                                "https://github.com/Owner/Repo",
	}
	for in, want := range ok {
		got, valid := NormalizeGitHubSource(in)
		if !valid || got != want {
			t.Errorf("NormalizeGitHubSource(%q) = (%q,%v); want (%q,true)", in, got, valid, want)
		}
	}
	bad := []string{"", "   ", "justoneword", "gitlab.com/x/y", "/", "owner/"}
	for _, in := range bad {
		if got, valid := NormalizeGitHubSource(in); valid {
			t.Errorf("NormalizeGitHubSource(%q) = (%q,true); want invalid", in, got)
		}
	}
}
