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
}

// New returns a GitHub provider with a sane HTTP client and the public API base.
// Pass an empty token for unauthenticated (rate-limited) access.
func New(token string) *GitHub {
	return &GitHub{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		token:      token,
	}
}

// ghRelease is the slice of the GitHub release object we consume.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// Get resolves the TARGET update's release notes — the notes for toTag (the
// newest tag the engine resolved), or the repo's latest release when toTag is a
// rolling tag like "latest". This is a single GitHub call (plus the optional
// archived check), versus the old "mine the whole from→to span" which made many;
// the smaller request volume eases GitHub's rate limit, and showing the pending
// update's own notes is what the user asked for.
//
// It returns (nil, false) so the chain falls through to the version-delta
// Fallback when: the source is not a github.com repo, the API call fails (incl.
// rate limit), OR the repo has no usable release (404 on both the exact tag and
// the latest release). When a release is found it returns (changelog, true) with
// that single release as entries[0] and its body in Raw.
func (g *GitHub) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	owner, repo, ok := parseGitHubRepo(c.Source)
	if !ok {
		return nil, false
	}

	// The target release: the exact release for toTag, else (rolling tag, or no
	// release named after it) the repo's latest release. nil → no usable release.
	rel := g.fetchTargetRelease(ctx, owner, repo, toTag)
	if rel == nil {
		return nil, false // no release notes → let the Fallback's version-delta win
	}

	// The archived ("deprecated/EOL") check is a second GitHub call; spend it now
	// that we have a release worth annotating.
	cl := &model.Changelog{
		FromTag:      fromTag,
		ToTag:        toTag,
		SkippedCount: 0, // we show only the target release now, never a span
		Entries:      []model.ReleaseEntry{*rel},
		Raw:          rel.Body,
		Source:       "GitHub releases (OCI label)",
		Provider:     "github",
		URL:          "https://github.com/" + owner + "/" + repo + "/releases",
		Deprecated:   g.repoArchived(ctx, owner, repo),
	}
	return cl, true
}

// fetchTargetRelease returns the release to show: the exact release named after
// toTag (v-prefix tolerant), else the repo's latest release. It returns nil when
// the repo has no usable release (404) OR on a transport/rate-limit failure — in
// both cases Get falls through so the version-delta Fallback handles it.
func (g *GitHub) fetchTargetRelease(ctx context.Context, owner, repo, toTag string) *model.ReleaseEntry {
	// A real version tag: ask for that exact release directly (one call). A 404
	// just means the project didn't cut a GitHub release for it — fall back to
	// the latest release below. Any other non-200 (e.g. a 429/403 rate limit) is
	// a real failure → give up (nil) so the chain falls through.
	if isVersionTag(toTag) {
		rel, status, ok := g.fetchRelease(ctx, owner, repo, "/releases/tags/"+toTag)
		switch {
		case ok:
			return rel
		case status != http.StatusNotFound:
			return nil
		}
	}
	// Rolling tag, or no release for the exact tag: the repo's latest release.
	rel, _, _ := g.fetchRelease(ctx, owner, repo, "/releases/latest")
	return rel
}

// isVersionTag reports whether a tag looks like a real version (e.g. "1.8",
// "v2.3.1") rather than a rolling tag like "latest" — used to decide whether to
// request a specific release vs. the repo's latest.
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

// fetchRelease GETs a single release endpoint (e.g. "/releases/latest" or
// "/releases/tags/v1.2.3") and decodes it into a ReleaseEntry. It returns the
// HTTP status alongside ok so the caller can distinguish a 404 (no such release,
// keep going) from a transport/rate-limit failure (fall through). status is 0
// when the request couldn't be issued at all.
func (g *GitHub) fetchRelease(ctx context.Context, owner, repo, path string) (*model.ReleaseEntry, int, bool) {
	url := g.baseURL + "/repos/" + owner + "/" + repo + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, 0, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// A rate-limited anonymous client (60 req/h, shared per source IP) is the
		// usual reason a github-sourced container shows no changelog. It's silent
		// otherwise — the chain just falls to the empty Fallback — so say it once,
		// loudly, with the remedy. 403/429 + Remaining:0 is GitHub's rate-limit shape.
		if isRateLimited(resp) {
			g.warnRateLimit()
		}
		return nil, resp.StatusCode, false
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, resp.StatusCode, false
	}
	return toEntry(rel), resp.StatusCode, true
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
