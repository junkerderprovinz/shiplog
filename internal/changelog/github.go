package changelog

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// GitHub resolves a changelog from a repo's GitHub Releases, identified by the
// container's OCI source label.
type GitHub struct {
	httpClient *http.Client
	baseURL    string
	token      string // optional, raises the rate limit
	warnOnce   sync.Once

	// mu guards cache, which the concurrent sweep workers share.
	mu    sync.Mutex
	cache map[string]*repoCache // key "owner/repo"
	ttl   time.Duration         // how long a cached list is served without revalidation
}

// repoCache is one repo's releases list with the ETag that revalidates it; a
// 304 answer does not count against the rate limit.
type repoCache struct {
	etag      string
	releases  []model.ReleaseEntry // newest first, as GitHub returns
	archived  bool
	fetchedAt time.Time
}

// New returns a GitHub provider for the public API. An empty token means
// anonymous access.
func New(token string) *GitHub {
	return &GitHub{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		token:      token,
		cache:      make(map[string]*repoCache),
		ttl:        10 * time.Minute,
	}
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// Get resolves the changelog from the releases of the container's GitHub
// source repo. A rolling or unmatched update gets the repo's recent releases
// (Recent), so a ":latest" container still shows what changed. When the rate
// limit leaves no data, it answers with a RateLimited changelog rather than
// letting the Fallback hide the cause.
func (g *GitHub) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	owner, repo, ok := parseGitHubRepo(c.Source)
	if !ok {
		return nil, false
	}
	releasesURL := "https://github.com/" + owner + "/" + repo + "/releases"

	rc, rateLimited := g.repoReleases(ctx, owner, repo)
	if rc == nil {
		if rateLimited {
			return &model.Changelog{
				FromTag:     fromTag,
				ToTag:       toTag,
				Source:      "GitHub releases (OCI label)",
				Provider:    "github",
				URL:         releasesURL,
				RateLimited: true,
			}, true
		}
		return nil, false
	}
	if len(rc.releases) == 0 {
		return nil, false
	}

	// A known version span lists every release across the jump, because a
	// breaking note often hides in an intermediate one. Otherwise the exact
	// toTag release is shown, and failing that the recent releases.
	var entries []model.ReleaseEntry
	recent := false
	fromV, fromOK := parseSemver(fromTag)
	toV, toOK := parseSemver(toTag)
	if fromOK && toOK && toV.compare(fromV) > 0 {
		for i := range rc.releases {
			rv, ok := parseSemver(rc.releases[i].Tag)
			if ok && rv.compare(fromV) > 0 && rv.compare(toV) <= 0 {
				entries = append(entries, rc.releases[i])
			}
		}
		// A lone release in the span stays a plain single-release changelog.
		if len(entries) > 1 {
			recent = true
		}
	}
	if entries == nil {
		if isVersionTag(toTag) {
			want := strings.TrimPrefix(toTag, "v")
			for i := range rc.releases {
				if strings.TrimPrefix(rc.releases[i].Tag, "v") == want {
					entries = rc.releases[i : i+1]
					break
				}
			}
		}
		if entries == nil {
			entries = rc.releases[:min(5, len(rc.releases))]
			recent = true
		}
	}

	return &model.Changelog{
		FromTag:    fromTag,
		ToTag:      toTag,
		Entries:    entries,
		Raw:        entries[0].Body,
		Source:     "GitHub releases (OCI label)",
		Provider:   "github",
		URL:        releasesURL,
		Deprecated: rc.archived,
		Recent:     recent,
	}, true
}

// repoReleases serves a cached list younger than ttl without a request and
// revalidates an older one with its ETag, which keeps a sweep under the
// anonymous limit of 60 requests per hour. Any failure falls back to the stale
// list; the bool reports a rate limit that left no list at all.
func (g *GitHub) repoReleases(ctx context.Context, owner, repo string) (*repoCache, bool) {
	key := owner + "/" + repo

	g.mu.Lock()
	defer g.mu.Unlock()

	cached := g.cache[key]
	if cached != nil && time.Since(cached.fetchedAt) < g.ttl {
		return cached, false
	}

	url := g.baseURL + "/repos/" + owner + "/" + repo + "/releases?per_page=20"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return cached, false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	if cached != nil && cached.etag != "" {
		req.Header.Set("If-None-Match", cached.etag)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return cached, false
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotModified && cached != nil:
		cached.fetchedAt = time.Now()
		return cached, false
	case resp.StatusCode == http.StatusOK:
		var rels []ghRelease
		if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
			return cached, false
		}
		entries := make([]model.ReleaseEntry, 0, len(rels))
		for _, r := range rels {
			entries = append(entries, *toEntry(r))
		}
		rc := &repoCache{
			etag:      resp.Header.Get("ETag"),
			releases:  entries,
			archived:  g.repoArchived(ctx, owner, repo),
			fetchedAt: time.Now(),
		}
		g.cache[key] = rc
		return rc, false
	case isRateLimited(resp):
		if cached != nil {
			return cached, false
		}
		g.warnRateLimit()
		return nil, true
	default:
		return cached, false
	}
}

// isVersionTag tells a version such as "v2.3.1" from a rolling tag like
// "latest".
func isVersionTag(tag string) bool {
	_, ok := parseSemver(tag)
	return ok
}

// repoArchived reports whether the repo is archived, treating any error as not
// archived.
func (g *GitHub) repoArchived(ctx context.Context, owner, repo string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/repos/"+owner+"/"+repo, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body struct {
		Archived bool `json:"archived"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	return body.Archived
}

// isRateLimited reports whether resp is GitHub's rate-limit response: a 403 or
// 429 whose X-RateLimit-Remaining header is "0".
func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return false
	}
	return resp.Header.Get("X-RateLimit-Remaining") == "0"
}

// warnRateLimit logs the cause once per provider, so a sweep over many
// containers does not flood the log.
func (g *GitHub) warnRateLimit() {
	g.warnOnce.Do(func() {
		if g.token != "" {
			log.Print("shiplog: GitHub API rate limit hit despite GITHUB_TOKEN; changelogs will be incomplete until it resets")
			return
		}
		log.Print("shiplog: GitHub API rate limit hit (anonymous, 60 req/h); changelogs are empty until it resets, set GITHUB_TOKEN to raise the limit to 5000/h")
	})
}

func toEntry(r ghRelease) *model.ReleaseEntry {
	return &model.ReleaseEntry{Tag: r.TagName, Body: r.Body, URL: r.HTMLURL, PublishedAt: r.PublishedAt}
}

// parseGitHubRepo extracts owner/repo from a github.com URL or scp-style
// remote, with or without a trailing ".git" or "/".
func parseGitHubRepo(source string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(source)
	if !strings.Contains(s, "github.com") {
		return "", "", false
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "git@github.com:")
	s = strings.TrimPrefix(s, "github.com/")
	s = strings.TrimPrefix(s, "github.com:")
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")

	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
