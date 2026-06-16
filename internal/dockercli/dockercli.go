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
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// sourceLabel is the OCI label that points at a project's source repository.
const sourceLabel = "org.opencontainers.image.source"

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
// against the registry, unlike ImageID which is the local config digest.
type dockerImage struct {
	RepoDigests []string `json:"RepoDigests"`
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
	digestCache := map[string]string{} // ImageID → manifest digest (one inspect per image)
	for _, dc := range raw {
		repo, tag := splitImageRef(dc.Image)
		dig, ok := digestCache[dc.ImageID]
		if !ok {
			dig = c.imageDigest(ctx, dc.ImageID, repo)
			digestCache[dc.ImageID] = dig
		}
		out = append(out, model.Container{
			ID:     dc.ID,
			Name:   containerName(dc.Names),
			Image:  dc.Image,
			Repo:   repo,
			Tag:    tag,
			Digest: dig,
			Source: dc.Labels[sourceLabel],
			State:  dc.State,
		})
	}
	return out, nil
}

// imageDigest inspects an image and returns its registry manifest digest
// ("sha256:…"), or "" if the image has none (e.g. built locally, never pulled),
// in which case the engine simply won't claim a digest update.
func (c *Client) imageDigest(ctx context.Context, imageID, repo string) string {
	if imageID == "" {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/images/"+imageID+"/json", nil)
	if err != nil {
		return ""
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var img dockerImage
	if err := json.NewDecoder(resp.Body).Decode(&img); err != nil {
		return ""
	}
	return pickDigest(img.RepoDigests, repo)
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

// splitImageRef splits a Docker image reference into a normalized repo and tag.
//
// Bare/short names are normalized to their canonical Docker Hub form
// ("redis" -> "docker.io/library/redis", "user/app" -> "docker.io/user/app");
// registry-qualified refs are kept as-is. A missing tag defaults to "latest".
func splitImageRef(ref string) (repo, tag string) {
	repo, tag = ref, "latest"

	// Split off the tag. A ':' only delimits a tag when it appears after the
	// last '/', otherwise it's a registry port (e.g. "registry:5000/app").
	if i := strings.LastIndex(ref, ":"); i >= 0 && !strings.Contains(ref[i+1:], "/") {
		repo, tag = ref[:i], ref[i+1:]
	}

	return normalizeRepo(repo), tag
}

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
