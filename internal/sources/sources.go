// Package sources decides which GitHub repo a container's changelog comes from.
// The image's OCI source label is often a packaging wrapper, inherited from a
// base image, or missing, so a user override, a curated table and the
// template's project page take precedence over it, in that order.
package sources

import "strings"

// curated maps an image repo without its registry host to the upstream GitHub
// repo that publishes real releases. It stays small and verified, because a
// wrong default is worse than none.
var curated = map[string]string{
	// LinuxServer.io wrappers with real upstream Releases.
	"linuxserver/radarr":   "https://github.com/Radarr/Radarr",
	"linuxserver/sonarr":   "https://github.com/Sonarr/Sonarr",
	"linuxserver/lidarr":   "https://github.com/Lidarr/Lidarr",
	"linuxserver/prowlarr": "https://github.com/Prowlarr/Prowlarr",
	"linuxserver/readarr":  "https://github.com/Readarr/Readarr",
	"linuxserver/whisparr": "https://github.com/Whisparr/Whisparr",
	"linuxserver/bazarr":   "https://github.com/morpheus65535/bazarr",

	// Third-party and official images with real GitHub Releases.
	"library/redis":            "https://github.com/redis/redis",
	"library/mariadb":          "https://github.com/MariaDB/server",
	"clamav/clamav":            "https://github.com/Cisco-Talos/clamav",
	"ollama/ollama":            "https://github.com/ollama/ollama",
	"minio/minio":              "https://github.com/minio/minio",
	"jlesage/filebot":          "https://github.com/jlesage/docker-filebot",
	"jlesage/handbrake":        "https://github.com/jlesage/docker-handbrake",
	"storjlabs/storagenode":    "https://github.com/storj/storj",
	"jc21/nginx-proxy-manager": "https://github.com/NginxProxyManager/nginx-proxy-manager",
	"germannewsmaker/myspeed":  "https://github.com/gnmyt/MySpeed",
}

// Kinds reported by Resolve, for the "where did this come from" label.
const (
	KindOCI      = "oci"
	KindCurated  = "curated"
	KindOverride = "override"
	KindProject  = "project"
)

// Resolve returns the effective changelog source for an image repo and which
// layer decided it. The project page counts only when it points at a GitHub
// repo, since templates often link a homepage instead.
func Resolve(repo, ociSource string, overrides map[string]string, projectPage string) (source, kind string) {
	if repo != "" {
		if ov, ok := overrides[repo]; ok && strings.TrimSpace(ov) != "" {
			return ov, KindOverride
		}
		if up, ok := curatedUpstream(repo); ok {
			return up, KindCurated
		}
	}
	if gh, ok := NormalizeGitHubSource(projectPage); ok {
		return gh, KindProject
	}
	return ociSource, KindOCI
}

// curatedUpstream strips the registry host so the lscr.io, ghcr.io and
// docker.io forms of an image share one key.
func curatedUpstream(repo string) (string, bool) {
	path := repo
	// The first segment is a host only when it contains a dot or a port colon.
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		host := repo[:i]
		if strings.ContainsAny(host, ".:") {
			path = repo[i+1:]
		}
	}
	up, ok := curated[path]
	return up, ok
}

// NormalizeGitHubSource turns "owner/repo", a github.com URL (with or without a
// trailing path or ".git") or an scp-style remote into
// "https://github.com/owner/repo".
func NormalizeGitHubSource(in string) (string, bool) {
	s := strings.TrimSpace(in)
	if s == "" {
		return "", false
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "git@github.com:")
	s = strings.TrimPrefix(s, "github.com/")
	s = strings.TrimPrefix(s, "github.com:")
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	owner, repo := parts[0], parts[1]
	// A remaining host means another forge, such as "gitlab.com/x/y".
	if strings.ContainsAny(owner, ".:") {
		return "", false
	}
	return "https://github.com/" + owner + "/" + repo, true
}
