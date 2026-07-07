// Package dockercli is a tiny, read-only Docker Engine API client that talks
// to the daemon over its unix socket. It only ever issues GET requests, so it
// cannot mutate container state by construction.
package dockercli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// OCI image labels we read. source points at the project repo; version (with
// revision as a fallback) is what the image declares as its own version, used to
// show a rolling container's installed version without first witnessing an update.
const (
	sourceLabel   = "org.opencontainers.image.source"
	versionLabel  = "org.opencontainers.image.version"
	revisionLabel = "org.opencontainers.image.revision"
)

// Client is a read-only Docker Engine API client over a unix socket.
type Client struct {
	http    *http.Client
	baseURL string // e.g. "http://docker/v1.43"; the host is ignored (unix dial)
}

// New returns a Client that dials the Docker daemon at socketPath.
func New(socketPath string) *Client {
	return &Client{
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				},
			},
		},
		baseURL: "http://docker/v1.43",
	}
}

// dockerContainer mirrors the subset of /containers/json we consume. Labels are
// included in the list response on modern engines, so one GET suffices.
type dockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	State   string            `json:"State"`
	Labels  map[string]string `json:"Labels"`
}

// dockerImage mirrors the subset of /images/{id}/json we consume. RepoDigests
// carries the registry MANIFEST digest ("repo@sha256:…") — the thing to compare
// against the registry, unlike ImageID which is the local config digest. Config
// and ContainerConfig hold the image's own OCI labels (the version/revision the
// image declares about itself).
type dockerImage struct {
	RepoDigests []string `json:"RepoDigests"`
	Config      struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	ContainerConfig struct {
		Labels map[string]string `json:"Labels"`
	} `json:"ContainerConfig"`
}

// imageInfo is the per-image data one inspect yields: the registry manifest
// digest(s), the version the image declares about itself, and whether the
// image only exists locally. isLocal is only ever set after a SUCCESSFUL
// inspect (empty RepoDigests = built locally, never pushed/pulled); a failed
// inspect leaves the zero value so a Docker hiccup can't demote a registry
// image to "local".
type imageInfo struct {
	digest  string
	digests []string
	version string
	isLocal bool
}

// List fetches all containers (running and stopped) and resolves each into a
// model.Container. It issues a single GET /containers/json?all=1.
func (c *Client) List(ctx context.Context) ([]model.Container, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/containers/json?all=1", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query docker: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("docker returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var raw []dockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode containers: %w", err)
	}

	out := make([]model.Container, 0, len(raw))
	infoCache := map[string]imageInfo{} // ImageID → digest+version (one inspect per image)
	for _, dc := range raw {
		repo, tag, pinnedDigest := splitImageRef(dc.Image)
		info, ok := infoCache[dc.ImageID]
		if !ok {
			info = c.inspectImage(ctx, dc.ImageID, repo)
			infoCache[dc.ImageID] = info
		}
		// Source = the OCI label if present, else derived from a ghcr.io image
		// (ghcr.io/owner/repo IS github.com/owner/repo) so the changelog resolves
		// even when the image sets no source label. Prefer the container's own
		// label set (cheap, in the list response); fall back to the image config.
		source := dc.Labels[sourceLabel]
		if source == "" {
			source = ghcrSource(repo)
		}
		out = append(out, model.Container{
			ID:           dc.ID,
			Name:         containerName(dc.Names),
			Image:        dc.Image,
			Repo:         repo,
			Tag:          tag,
			Digest:       info.digest,
			Digests:      info.digests,
			PinnedDigest: pinnedDigest,
			IsLocal:      info.isLocal,
			Source:       source,
			State:        dc.State,
			ImageVersion: info.version,
		})
	}
	return out, nil
}

// ghcrSource maps a ghcr.io image repo to its GitHub source URL (ghcr packages
// live under github.com/<owner>/<repo>). Returns "" for non-ghcr repos.
func ghcrSource(repo string) string {
	const prefix = "ghcr.io/"
	if !strings.HasPrefix(repo, prefix) {
		return ""
	}
	parts := strings.Split(repo[len(prefix):], "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return "https://github.com/" + parts[0] + "/" + parts[1]
}

// inspectImage inspects an image once and returns its registry manifest digest
// ("sha256:…") plus the version the image declares about itself. A zero imageInfo
// is returned on any error (e.g. built locally, never pulled), in which case the
// engine simply won't claim a digest update nor a label version.
func (c *Client) inspectImage(ctx context.Context, imageID, repo string) imageInfo {
	if imageID == "" {
		return imageInfo{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/images/"+imageID+"/json", nil)
	if err != nil {
		return imageInfo{}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return imageInfo{}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return imageInfo{}
	}
	var img dockerImage
	if err := json.NewDecoder(resp.Body).Decode(&img); err != nil {
		return imageInfo{}
	}
	return imageInfo{
		digest:  pickDigest(img.RepoDigests, repo),
		digests: allDigests(img.RepoDigests),
		version: imageVersion(img),
		isLocal: len(img.RepoDigests) == 0,
	}
}

// allDigests extracts the digest part of every RepoDigests entry.
func allDigests(repoDigests []string) []string {
	var out []string
	for _, rd := range repoDigests {
		if at := strings.LastIndex(rd, "@"); at >= 0 {
			out = append(out, rd[at+1:])
		}
	}
	return out
}

// imageVersion reads the version the image declares about itself: the OCI
// version label, falling back to the revision label. Config.Labels is the modern
// location; ContainerConfig.Labels is the legacy one. Returns "" when neither
// label is present.
func imageVersion(img dockerImage) string {
	for _, labels := range []map[string]string{img.Config.Labels, img.ContainerConfig.Labels} {
		if v := labels[versionLabel]; v != "" {
			return v
		}
		if v := labels[revisionLabel]; v != "" {
			return v
		}
	}
	return ""
}

// pickDigest returns the "@sha256:…" digest for repo from a RepoDigests list,
// falling back to the first entry's digest (all entries of one image share the
// same manifest digest).
func pickDigest(repoDigests []string, repo string) string {
	var first string
	for _, rd := range repoDigests {
		at := strings.LastIndex(rd, "@")
		if at < 0 {
			continue
		}
		name, dig := rd[:at], rd[at+1:]
		if first == "" {
			first = dig
		}
		if normalizeRepo(name) == repo {
			return dig
		}
	}
	return first
}

// containerName returns the first name with its leading '/' stripped.
func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

// splitImageRef splits a Docker image reference into a normalized repo, tag and
// pinning digest.
//
// Bare/short names are normalized to their canonical Docker Hub form
// ("redis" -> "docker.io/library/redis", "user/app" -> "docker.io/user/app");
// registry-qualified refs are kept as-is. A missing tag defaults to "latest".
//
// A digest suffix ("repo@sha256:…", optionally "repo:tag@sha256:…") pins the
// image; it must be split off BEFORE tag detection or the colon inside
// "sha256:…" would be mistaken for the tag separator. A digest-pinned ref
// without an explicit tag has no tag at all (not "latest"). A ref that is a
// bare image ID ("sha256:…" or the plain hex) names no repo whatsoever.
func splitImageRef(ref string) (repo, tag, pinnedDigest string) {
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		ref, pinnedDigest = ref[:at], ref[at+1:]
	}
	if ref == "" || isImageID(ref) {
		return "", "", pinnedDigest
	}

	repo = ref

	// Split off the tag. A ':' only delimits a tag when it appears after the
	// last '/', otherwise it's a registry port (e.g. "registry:5000/app").
	if i := strings.LastIndex(ref, ":"); i >= 0 && !strings.Contains(ref[i+1:], "/") {
		repo, tag = ref[:i], ref[i+1:]
	} else if pinnedDigest == "" {
		tag = "latest"
	}

	return normalizeRepo(repo), tag, pinnedDigest
}

// imageIDRe matches a bare image ID reference: 64 hex chars, optionally with
// the "sha256:" prefix.
var imageIDRe = regexp.MustCompile(`^(sha256:)?[0-9a-f]{64}$`)

// isImageID reports whether ref references an image by ID instead of by name.
func isImageID(ref string) bool { return imageIDRe.MatchString(ref) }

// normalizeRepo expands a Docker Hub short name to its fully-qualified form and
// leaves any already registry-qualified reference untouched.
func normalizeRepo(repo string) string {
	slash := strings.IndexByte(repo, '/')

	// No slash: a single-segment name like "redis" -> docker.io/library/redis.
	if slash < 0 {
		return "docker.io/library/" + repo
	}

	// The first path segment is a registry only if it looks like a hostname
	// (contains '.' or ':') or is the special "localhost".
	first := repo[:slash]
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return repo
	}

	// Otherwise it's a Docker Hub "user/app" short form.
	return "docker.io/" + repo
}
