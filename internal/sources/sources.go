// Package sources decides which GitHub repo a container's changelog is mined
// from. ShipLog defaults to the image's OCI source label
// (org.opencontainers.image.source), but that label is often the packaging
// wrapper (LinuxServer's docker-<app>), plain wrong (inherited from a base
// image), or missing. This package layers two corrections on top:
//
//   - a small CURATED map of common LinuxServer wrappers to their upstream repo
//     (only apps that publish real GitHub Releases),
//   - the container template's PROJECT PAGE (<Project> in the dockerMan
//     template) when it points at a GitHub repo — most templates do, which
//     covers images whose OCI label is missing or inherited from a base image
//     (community suggestion by btTeddy), and
//   - USER OVERRIDES that always win.
//
// Precedence: override > curated > project page > the image's OCI source label.
package sources

import "strings"

// lsioUpstream maps a LinuxServer.io app (the part after "linuxserver/" in the
// image repo) to its upstream GitHub source. Kept deliberately small and to
// apps with real GitHub Releases — a wrong default is worse than none, and a
// manual override always beats this. Registry host is ignored, so lscr.io,
// ghcr.io and docker.io forms all match.
var lsioUpstream = map[string]string{
	"radarr":   "https://github.com/Radarr/Radarr",
	"sonarr":   "https://github.com/Sonarr/Sonarr",
	"lidarr":   "https://github.com/Lidarr/Lidarr",
	"prowlarr": "https://github.com/Prowlarr/Prowlarr",
	"readarr":  "https://github.com/Readarr/Readarr",
	"whisparr": "https://github.com/Whisparr/Whisparr",
	"bazarr":   "https://github.com/morpheus65535/bazarr",
}

// Kinds reported by Resolve, for the human "where did this come from" label.
const (
	KindOCI      = "oci"
	KindCurated  = "curated"
	KindOverride = "override"
	KindProject  = "project"
)

// Resolve returns the effective changelog source for an image repo, given the
// source derived from the image (the OCI label / ghcr fallback), the set of
// user overrides (repo → source) and the container template's project page.
// It also reports which layer decided, so the UI can label it. An empty repo
// (bare image-ID ref) can't be keyed for overrides/curated, but the project
// page is keyed by container and still applies. The project page only counts
// when it actually points at a GitHub repo — templates often link a homepage
// (radarr.video etc.), which can't be mined for releases.
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

// curatedUpstream maps a LinuxServer image repo to its upstream GitHub source.
// It strips the registry host so "lscr.io/linuxserver/radarr",
// "ghcr.io/linuxserver/radarr" and "docker.io/linuxserver/radarr" all resolve.
func curatedUpstream(repo string) (string, bool) {
	path := repo
	// Drop a leading registry host ("lscr.io/", "ghcr.io/", "docker.io/"): the
	// first segment is a host only when it contains a dot (or a port colon).
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		host := repo[:i]
		if strings.ContainsAny(host, ".:") {
			path = repo[i+1:]
		}
	}
	const nsPrefix = "linuxserver/"
	if strings.HasPrefix(path, nsPrefix) {
		if up, ok := lsioUpstream[strings.TrimPrefix(path, nsPrefix)]; ok {
			return up, true
		}
	}
	return "", false
}

// NormalizeGitHubSource turns user input into the canonical
// "https://github.com/owner/repo" form the changelog resolver understands, or
// reports false when it isn't a usable owner/repo. It accepts a bare
// "owner/repo", a "github.com/owner/repo", an "https://github.com/owner/repo"
// URL (with or without a trailing "/releases" or ".git"), and scp-style remotes.
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
	// Drop a trailing "/releases" (and anything after owner/repo) so a pasted
	// releases URL works.
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	owner, repo := parts[0], parts[1]
	// Guard against a host slipping through (e.g. "gitlab.com/x/y").
	if strings.ContainsAny(owner, ".:") {
		return "", false
	}
	return "https://github.com/" + owner + "/" + repo, true
}
