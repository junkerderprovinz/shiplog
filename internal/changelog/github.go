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
	baseURL    string // GitHub REST API base, injectable for tests.
	token      string // optional PAT; sent as a Bearer header when non-empty.
	warnOnce   sync.Once

	// mu guards cache. A sweep over many containers hits the same repos and runs
	// concurrently; the cache is the whole point (see repoReleases), so its access
	// must be serialized.
	mu    sync.Mutex
	cache map[string]*repoCache // key "owner/repo"
	ttl   time.Duration         // how long a cached releases list is served without revalidation
}

// repoCache is one repo's releases list plus the metadata needed to revalidate
// it cheaply (an ETag conditional GET → 304 does NOT count against the API rate
// limit) and to annotate the changelog (archived = EOL).
type repoCache struct {
	etag      string
	releases  []model.ReleaseEntry // newest first, as GitHub returns
	archived  bool
	fetchedAt time.Time
}

// New returns a GitHub provider with a sane HTTP client and the public API base.
// Pass an empty token for unauthenticated (rate-limited) access.
func New(token string) *GitHub {
	return &GitHub{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		token:      token,
		cache:      make(map[string]*repoCache),
		ttl:        10 * time.Minute,
	}
}

// ghRelease is the slice of the GitHub release object we consume.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// Get resolves the changelog for a container update from the repo named by its
// OCI source label. It shows the repo's own release notes: the single release
// matching toTag for a pinned version bump, or the recent N releases for a
// digest/rolling/no-exact-match update (Recent=true) so a ":latest" container
// still gets a meaningful "what changed" list.
//
// The repo's releases LIST is cached with an ETag and a short TTL (see
// repoReleases), so a sweep over many containers barely touches the GitHub API —
// the #1 real-world cause of an empty changelog is the anonymous rate limit (60
// req/h shared per IP), and caching is the root fix.
//
// It returns (nil, false) — falling through to the version-delta Fallback — when
// the source is not a github.com repo, the repo has no releases, or a first-time
// fetch fails for a non-rate-limit reason. When the rate limit is hit with no
// cached data it returns a RateLimited changelog with handled=true, so the UI
// shows an honest note rather than letting the Fallback blank it out.
func (g *GitHub) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	owner, repo, ok := parseGitHubRepo(c.Source)
	if !ok {
		return nil, false
	}
	releasesURL := "https://github.com/" + owner + "/" + repo + "/releases"

	rc, rateLimited := g.repoReleases(ctx, owner, repo)
	if rc == nil {
		if rateLimited {
			// No data and rate limited: hand back an honest note instead of a blank.
			// handled=true so the Fallback does NOT overwrite it with a bare delta.
			return &model.Changelog{
				FromTag:     fromTag,
				ToTag:       toTag,
				Source:      "GitHub releases (OCI label)",
				Provider:    "github",
				URL:         releasesURL,
				RateLimited: true,
			}, true
		}
		return nil, false // no data, not rate limited → let the Fallback win
	}
	if len(rc.releases) == 0 {
		return nil, false // repo has no releases → version-delta Fallback
	}

	// A real version bump with a release of its own: show exactly that release.
	// Otherwise (rolling/digest tag, or no release named after toTag): show the
	// repo's recent releases so the user still sees what changed.
	var entries []model.ReleaseEntry
	recent := false
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

// repoReleases returns the cached (or freshly fetched) releases list for a repo,
// plus a rateLimited flag. A cached entry younger than ttl is served with no
// network at all; otherwise a conditional GET (If-None-Match: <etag>) revalidates
// it — a 304 Not-Modified refreshes the cache without counting against the rate
// limit, and is the mechanism that keeps a sweep under the anonymous 60 req/h.
//
// Return contract:
//   - fresh cache hit → (cache, false), no network
//   - 304            → (cache, false), fetchedAt refreshed
//   - 200            → (new cache, false), stored
//   - rate limited   → (stale cache, false) if one exists, else (nil, true)
//   - other failure  → (stale cache, false) if one exists, else (nil, false)
func (g *GitHub) repoReleases(ctx context.Context, owner, repo string) (*repoCache, bool) {
	key := owner + "/" + repo

	g.mu.Lock()
	defer g.mu.Unlock()

	cached := g.cache[key]
	if cached != nil && time.Since(cached.fetchedAt) < g.ttl {
		return cached, false // fresh: no network
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
		return cached, false // transport failure → serve stale cache if any
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotModified && cached != nil:
		cached.fetchedAt = time.Now() // 304: unchanged, extend the TTL for free
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
		// A rate-limited anonymous client (60 req/h, shared per source IP) is the
		// usual reason a github-sourced container shows no changelog. Serve stale
		// data if we have it; otherwise say it once, loudly, and signal upward.
		if cached != nil {
			return cached, false
		}
		g.warnRateLimit()
		return nil, true
	default:
		return cached, false // any other non-200 → serve stale cache if any
	}
}

// isVersionTag reports whether a tag looks like a real version (e.g. "1.8",
// "v2.3.1") rather than a rolling tag like "latest" — used to decide whether to
// look for a specific release vs. show the repo's recent ones.
func isVersionTag(tag string) bool {
	_, ok := parseSemver(tag)
	return ok
}

// repoArchived reports whether the GitHub repo is archived (a good "deprecated /
// EOL" signal). Best-effort: any error → false.
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

// warnRateLimit logs the rate-limit cause and remedy exactly once per provider,
// so a sweep over many github-sourced containers doesn't flood the log.
func (g *GitHub) warnRateLimit() {
	g.warnOnce.Do(func() {
		if g.token != "" {
			log.Print("shiplog: GitHub API rate limit hit despite GITHUB_TOKEN — changelogs will be incomplete until it resets")
			return
		}
		log.Print("shiplog: GitHub API rate limit hit (anonymous, 60 req/h) — changelogs are empty until it resets; set GITHUB_TOKEN to raise the limit to 5000/h")
	})
}

// toEntry maps a GitHub release to a ReleaseEntry.
func toEntry(r ghRelease) *model.ReleaseEntry {
	return &model.ReleaseEntry{Tag: r.TagName, Body: r.Body, URL: r.HTMLURL, PublishedAt: r.PublishedAt}
}

// parseGitHubRepo extracts owner/repo from a github.com URL, tolerating a
// trailing ".git" or "/". It returns false for empty or non-github sources.
//
//	"https://github.com/immich-app/immich"     -> "immich-app", "immich"
//	"https://github.com/o/r.git"               -> "o", "r"
//	"https://github.com/o/r/"                  -> "o", "r"
func parseGitHubRepo(source string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(source)
	if !strings.Contains(s, "github.com") {
		return "", "", false
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "git@github.com:") // scp-style remotes
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
