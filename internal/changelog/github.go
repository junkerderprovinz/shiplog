package changelog

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// GitHub resolves a changelog from a repo's GitHub Releases, identified by the
// container's OCI source label.
type GitHub struct {
	httpClient *http.Client
	baseURL    string // GitHub REST API base, injectable for tests.
	token      string // optional PAT; sent as a Bearer header when non-empty.
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

// Get mines GitHub releases for the span (fromTag, toTag]. It returns
// (nil, false) when the container source is not a github.com repo or the API
// call fails, so the chain can fall through. When the repo is on GitHub it
// returns (changelog, true) even if no releases matched the span — a github
// repo with zero matches is still a better answer than a worse source.
func (g *GitHub) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	owner, repo, ok := parseGitHubRepo(c.Source)
	if !ok {
		return nil, false
	}

	rels, ok := g.fetchReleases(ctx, owner, repo)
	if !ok {
		return nil, false // API failure → fall through.
	}

	entries := selectSpan(rels, fromTag, toTag)

	// Up to date (running == newest): there's no span, but we still want to show
	// the release notes of the version actually in use — the exact release for the
	// tag, else the latest release (e.g. when the tag is "latest").
	if fromTag == toTag {
		if e := releaseByTag(rels, toTag); e != nil {
			entries = []model.ReleaseEntry{*e}
		} else if e := latestRelease(rels); e != nil {
			entries = []model.ReleaseEntry{*e}
		}
	}

	skipped := 0
	if len(entries) > 1 {
		skipped = len(entries) - 1
	}

	bodies := make([]string, 0, len(entries))
	for _, e := range entries {
		bodies = append(bodies, e.Body)
	}

	cl := &model.Changelog{
		FromTag:      fromTag,
		ToTag:        toTag,
		SkippedCount: skipped,
		Entries:      entries,
		Raw:          strings.Join(bodies, "\n\n"),
		Source:       "GitHub releases (OCI label)",
		Provider:     "github",
		URL:          "https://github.com/" + owner + "/" + repo + "/compare/" + fromTag + "..." + toTag,
	}
	return cl, true
}

// fetchReleases pulls up to 100 releases. The bool is false on any transport or
// non-200 error, signalling the chain to fall through.
func (g *GitHub) fetchReleases(ctx context.Context, owner, repo string) ([]ghRelease, bool) {
	url := g.baseURL + "/repos/" + owner + "/" + repo + "/releases?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	var rels []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return nil, false
	}
	return rels, true
}

// selectSpan keeps releases with fromTag < tag <= toTag (semver, v-prefix
// tolerant) and returns them mapped to ReleaseEntry, sorted newest-first.
// Non-semver tags and tags outside the span are dropped.
func selectSpan(rels []ghRelease, fromTag, toTag string) []model.ReleaseEntry {
	from, fromOK := parseSemver(fromTag)
	to, toOK := parseSemver(toTag)

	kept := make([]struct {
		v semver
		e model.ReleaseEntry
	}, 0, len(rels))

	for _, r := range rels {
		v, ok := parseSemver(r.TagName)
		if !ok {
			continue
		}
		if fromOK && v.compare(from) <= 0 {
			continue // not strictly above fromTag
		}
		if toOK && v.compare(to) > 0 {
			continue // above toTag
		}
		kept = append(kept, struct {
			v semver
			e model.ReleaseEntry
		}{
			v: v,
			e: model.ReleaseEntry{
				Tag:         r.TagName,
				Body:        r.Body,
				URL:         r.HTMLURL,
				PublishedAt: r.PublishedAt,
			},
		})
	}

	sort.SliceStable(kept, func(i, j int) bool {
		return kept[i].v.compare(kept[j].v) > 0 // newest first
	})

	entries := make([]model.ReleaseEntry, 0, len(kept))
	for _, k := range kept {
		entries = append(entries, k.e)
	}
	return entries
}

// toEntry maps a GitHub release to a ReleaseEntry.
func toEntry(r ghRelease) *model.ReleaseEntry {
	return &model.ReleaseEntry{Tag: r.TagName, Body: r.Body, URL: r.HTMLURL, PublishedAt: r.PublishedAt}
}

// releaseByTag returns the release whose tag equals tag (v-prefix tolerant).
func releaseByTag(rels []ghRelease, tag string) *model.ReleaseEntry {
	want := strings.TrimPrefix(tag, "v")
	for i := range rels {
		if rels[i].TagName == tag || strings.TrimPrefix(rels[i].TagName, "v") == want {
			return toEntry(rels[i])
		}
	}
	return nil
}

// latestRelease returns the highest-semver release, or the first one if none
// parse as semver (the API returns newest-first).
func latestRelease(rels []ghRelease) *model.ReleaseEntry {
	best := -1
	var bestV semver
	for i := range rels {
		v, ok := parseSemver(rels[i].TagName)
		if !ok {
			continue
		}
		if best < 0 || v.compare(bestV) > 0 {
			best, bestV = i, v
		}
	}
	if best < 0 {
		if len(rels) == 0 {
			return nil
		}
		best = 0
	}
	return toEntry(rels[best])
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
