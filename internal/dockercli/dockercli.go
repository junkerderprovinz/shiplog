// Package dockercli is a read-only Docker Engine API client over the unix
// socket. It issues GET requests only, so it cannot change container state.
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

// OCI image labels for the project repo and the version an image declares.
const (
	sourceLabel   = "org.opencontainers.image.source"
	versionLabel  = "org.opencontainers.image.version"
	revisionLabel = "org.opencontainers.image.revision"
)

// managedLabel is set by Unraid's Docker Manager on every container it creates
// from a template.
const managedLabel = "net.unraid.docker.managed"

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

type dockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	State   string            `json:"State"`
	Labels  map[string]string `json:"Labels"`
}

// dockerImage is the part of /images/{id}/json in use. RepoDigests carries the
// registry manifest digest, which unlike ImageID can be compared with the
// registry.
type dockerImage struct {
	RepoDigests []string `json:"RepoDigests"`
	Config      struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	ContainerConfig struct {
		Labels map[string]string `json:"Labels"`
	} `json:"ContainerConfig"`
}

// imageInfo is what one image inspect yields. isLocal is set only after a
// successful inspect, so a failed one cannot mark a registry image as local.
type imageInfo struct {
	digest  string
	digests []string
	version string
	isLocal bool
}

// List returns all containers, running and stopped, inspecting each image once.
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
	infoCache := map[string]imageInfo{} // by ImageID
	for _, dc := range raw {
		repo, tag, pinnedDigest := splitImageRef(dc.Image)
		info, ok := infoCache[dc.ImageID]
		if !ok {
			info = c.inspectImage(ctx, dc.ImageID, repo)
			infoCache[dc.ImageID] = info
		}
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
			Managed:      dc.Labels[managedLabel] != "",
		})
	}
	return out, nil
}

// ghcrSource maps ghcr.io/owner/repo to github.com/owner/repo, so an image
// without a source label still gets a changelog.
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

// inspectImage returns a zero imageInfo on any error, so the engine claims
// neither a digest update nor a label version for that image.
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

func allDigests(repoDigests []string) []string {
	var out []string
	for _, rd := range repoDigests {
		if at := strings.LastIndex(rd, "@"); at >= 0 {
			out = append(out, rd[at+1:])
		}
	}
	return out
}

// imageVersion prefers the version label over the revision label, checking the
// current Config before the legacy ContainerConfig.
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

// pickDigest returns the RepoDigests digest for repo, else the first one.
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

func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

// splitImageRef splits an image reference into a normalized repo, tag and
// pinning digest. A missing tag means "latest" unless a digest pins the image;
// a bare image ID names no repo.
func splitImageRef(ref string) (repo, tag, pinnedDigest string) {
	// The digest goes first, or the colon in "sha256:" would read as a tag.
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		ref, pinnedDigest = ref[:at], ref[at+1:]
	}
	if ref == "" || isImageID(ref) {
		return "", "", pinnedDigest
	}

	repo = ref

	// A colon before the last slash is a registry port, as in "registry:5000/app".
	if i := strings.LastIndex(ref, ":"); i >= 0 && !strings.Contains(ref[i+1:], "/") {
		repo, tag = ref[:i], ref[i+1:]
	} else if pinnedDigest == "" {
		tag = "latest"
	}

	return normalizeRepo(repo), tag, pinnedDigest
}

var imageIDRe = regexp.MustCompile(`^(sha256:)?[0-9a-f]{64}$`)

func isImageID(ref string) bool { return imageIDRe.MatchString(ref) }

// normalizeRepo expands a Docker Hub short name such as "redis" or "user/app"
// and leaves a registry-qualified reference alone.
func normalizeRepo(repo string) string {
	slash := strings.IndexByte(repo, '/')
	if slash < 0 {
		return "docker.io/library/" + repo
	}

	// The first segment is a registry only when it looks like a hostname.
	first := repo[:slash]
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return repo
	}
	return "docker.io/" + repo
}
