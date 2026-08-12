// Package cafeed checks whether an installed app is still listed in Unraid's
// Community Applications catalog — the same feed CA's own plugin consults
// (Squidly271/AppFeed, mirrored to assets.ca.unraid.net), not the raw
// template-URL reachability proxy engine.repoGone corroborates. That proxy
// only ever catches a genuinely deleted/moved template file; it cannot see
// an app editorially demoted (CA's own "Deprecated" flag: hidden from
// default search, still installable, still updated by its maintainer) or
// pulled from the feed while its template file happens to still sit
// untouched in its source repo.
//
// Fetches are cheap by design: a tiny "-lastUpdated.json" ping is checked
// first, and the ~24MB full feed is only re-downloaded when its timestamp
// actually changed (observed cadence: every 4-8h). Both the current and the
// PREVIOUS successfully-fetched feed are kept on disk — a container is only
// ever reported absent when it is missing from both, since the feed is
// itself crowd-crawled and can have a transient gap for a renamed/moved
// template, the same false-positive shape already fixed once for the raw
// template-URL proxy (see engine.repoGone). Any fetch failure falls back to
// whatever is cached; with nothing cached at all, every lookup is
// inconclusive rather than guessed — fail open, never fail closed.
package cafeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultFeedURL        = "https://raw.githubusercontent.com/Squidly271/AppFeed/master/applicationFeed.json"
	defaultLastUpdatedURL = "https://raw.githubusercontent.com/Squidly271/AppFeed/master/applicationFeed-lastUpdated.json"
	cacheFile             = "ca-feed.json"
	prevCacheFile         = "ca-feed.prev.json"
)

// Entry is one app listing from the feed — only the fields ShipLog needs.
type Entry struct {
	Name             string `json:"Name"`
	Repository       string `json:"Repository"`
	TemplateURL      string `json:"TemplateURL"`
	Deprecated       bool   `json:"Deprecated"`
	ModeratorComment string `json:"ModeratorComment"`
}

// rawFeed mirrors only the top-level fields ShipLog reads out of the ~24MB
// feed; encoding/json silently ignores every field it doesn't know about.
type rawFeed struct {
	AppList              []Entry           `json:"applist"`
	Blacklisted          map[string]string `json:"blacklisted"`
	LastUpdatedTimestamp int64             `json:"last_updated_timestamp"`
}

type rawLastUpdated struct {
	LastUpdatedTimestamp int64 `json:"last_updated_timestamp"`
}

// Feed is a parsed, matchable snapshot of the catalog.
type Feed struct {
	byName      map[string][]Entry
	blacklisted map[string]string // normalized "owner/repo" -> reason
	previous    *Feed             // the crawl before this one, if one was cached; nil otherwise
}

// Result is what the feed says about one container, after cross-crawl
// confirmation of any absence.
type Result struct {
	Listed     bool   // false only once confirmed absent across two consecutive crawls
	Deprecated bool   // CA's own "hidden from default search" flag; the app is still listed and updated
	Note       string // moderator comment (deprecated) or blacklist reason (not listed), for display
}

// Fetcher fetches and caches the feed under dir (typically the engine's
// DATA_DIR — small JSON files, no reason to separate it from the SQLite DB).
type Fetcher struct {
	dir    string
	client *http.Client
	// feedURL/lastUpdatedURL are injectable so tests run against an
	// httptest.Server instead of the real internet.
	feedURL        string
	lastUpdatedURL string
}

// NewFetcher builds a Fetcher caching under dir.
func NewFetcher(dir string) *Fetcher {
	return &Fetcher{
		dir:            dir,
		client:         &http.Client{Timeout: 30 * time.Second},
		feedURL:        defaultFeedURL,
		lastUpdatedURL: defaultLastUpdatedURL,
	}
}

// Load returns the current feed, refreshing the on-disk cache only when the
// upstream feed has actually changed since the last successful fetch. It
// returns a nil Feed and an error only when there is truly nothing to check
// against — no fresh fetch succeeded AND nothing is cached; callers must
// skip CA-based checks entirely for that sweep rather than guess.
func (f *Fetcher) Load(ctx context.Context) (*Feed, error) {
	curPath := filepath.Join(f.dir, cacheFile)
	prevPath := filepath.Join(f.dir, prevCacheFile)

	cached, cacheErr := os.ReadFile(curPath)
	cachedTS := int64(-1)
	if cacheErr == nil {
		var lu rawLastUpdated
		if json.Unmarshal(cached, &lu) == nil {
			cachedTS = lu.LastUpdatedTimestamp
		}
	}

	freshTS, pingErr := f.fetchTimestamp(ctx)
	if pingErr == nil && cacheErr == nil && freshTS == cachedTS {
		// Nothing changed upstream since our last fetch — cheap path, no 24MB
		// re-download just to confirm that.
		return parse(cached, prevPath), nil
	}

	fresh, fetchErr := f.fetchFull(ctx)
	if fetchErr != nil {
		if cacheErr != nil {
			return nil, fmt.Errorf("cafeed: fetch failed and nothing cached: %w", fetchErr)
		}
		return parse(cached, prevPath), nil // degrade to whatever we had
	}
	// Rotate the about-to-be-replaced "current" into "previous" BEFORE
	// overwriting it, so the next Load can cross-check an absence against a
	// genuinely earlier crawl. Write failures are best-effort: fresh is
	// already in hand for THIS call either way, only the next sweep's cache
	// benefit is at stake.
	if cacheErr == nil {
		_ = os.WriteFile(prevPath, cached, 0o644)
	}
	_ = os.WriteFile(curPath, fresh, 0o644)
	return parse(fresh, prevPath), nil
}

func (f *Fetcher) fetchTimestamp(ctx context.Context) (int64, error) {
	b, err := f.fetchBytes(ctx, f.lastUpdatedURL)
	if err != nil {
		return 0, err
	}
	var lu rawLastUpdated
	if err := json.Unmarshal(b, &lu); err != nil {
		return 0, err
	}
	return lu.LastUpdatedTimestamp, nil
}

func (f *Fetcher) fetchFull(ctx context.Context) ([]byte, error) {
	return f.fetchBytes(ctx, f.feedURL)
}

func (f *Fetcher) fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "shiplog")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cafeed: %s: unexpected status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// parse builds a Feed from raw feed bytes, best-effort attaching the
// previous crawl (from prevPath) for absence cross-checking. A malformed or
// missing previous cache simply leaves Feed.previous nil — Lookup already
// treats that as "not yet confirmable", never as a false positive.
func parse(b []byte, prevPath string) *Feed {
	feed := parseOne(b)
	if feed == nil {
		return nil
	}
	if pb, err := os.ReadFile(prevPath); err == nil {
		feed.previous = parseOne(pb)
	}
	return feed
}

func parseOne(b []byte) *Feed {
	var raw rawFeed
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	byName := make(map[string][]Entry, len(raw.AppList))
	for _, e := range raw.AppList {
		name := normalizeName(e.Name)
		if name == "" {
			continue
		}
		byName[name] = append(byName[name], e)
	}
	blacklisted := make(map[string]string, len(raw.Blacklisted))
	for repo, reason := range raw.Blacklisted {
		blacklisted[normalizeRepo(repo)] = reason
	}
	return &Feed{byName: byName, blacklisted: blacklisted}
}

// Lookup reports what the feed says about a container. ok=false means
// inconclusive (an ambiguous name match with no repo/templateURL able to
// narrow it, or an apparent absence with no previous crawl yet to confirm it
// against) — callers must leave the container's CA state alone, never treat
// ok=false as any kind of verdict.
func (f *Feed) Lookup(name, repo, templateURL string) (Result, bool) {
	n := normalizeName(name)
	matches := f.byName[n]
	var e Entry
	switch len(matches) {
	case 0:
		if f.previous == nil {
			return Result{}, false // nothing to confirm an absence against yet
		}
		if len(f.previous.byName[n]) > 0 {
			return Result{}, false // present last crawl; today's gap unconfirmed
		}
		return Result{Listed: false}, true // absent from two consecutive crawls
	case 1:
		e = matches[0]
	default:
		var found bool
		e, found = disambiguate(matches, repo, templateURL)
		if !found {
			return Result{}, false
		}
	}
	if reason, blacklisted := f.blacklisted[normalizeRepo(e.Repository)]; blacklisted {
		return Result{Listed: false, Note: reason}, true
	}
	return Result{Listed: true, Deprecated: e.Deprecated, Note: e.ModeratorComment}, true
}

// disambiguate picks the one entry among several same-named candidates that
// actually matches this container, mirroring Community Applications' own
// join order (exec.php): repository prefix first, then the exact template
// URL ShipLog already trusts from the installed template.
func disambiguate(candidates []Entry, repo, templateURL string) (Entry, bool) {
	if repo != "" {
		nr := normalizeRepo(repo)
		for _, e := range candidates {
			if er := normalizeRepo(e.Repository); er != "" && strings.HasPrefix(nr, er) {
				return e, true
			}
		}
	}
	if templateURL != "" {
		for _, e := range candidates {
			if e.TemplateURL == templateURL {
				return e, true
			}
		}
	}
	return Entry{}, false
}

func normalizeName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normalizeRepo strips a docker.io registry prefix and the default
// "library/" namespace, matching CA's own Docker-Hub-style "owner/repo"
// Repository field. A non-Docker-Hub repo (ghcr.io, lscr.io, ...) passes
// through unchanged and simply won't prefix-match any CA entry — expected,
// since CA only lists Docker Hub images this way.
func normalizeRepo(repo string) string {
	repo = strings.TrimPrefix(repo, "docker.io/")
	repo = strings.TrimPrefix(repo, "library/")
	return repo
}
