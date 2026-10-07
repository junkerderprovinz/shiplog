// Package cafeed checks whether an installed app is still listed in the
// Community Applications feed that CA's own plugin reads. Unlike the template
// URL check in engine.repoGone, the feed also shows an app that CA demoted or
// pulled while its template file still exists.
//
// A small timestamp file is checked first, so the 24MB feed is downloaded only
// when it changed. The feed is crawled and can briefly miss a moved template,
// so an app counts as absent only when the previous crawl lacks it too. With
// nothing cached, every lookup is inconclusive.
//
// Absence means "removed" only for an app CA once carried. A template from a
// repository CA does not crawl, such as the user's own, is absent by nature,
// so CrawlsTemplate decides between Lookup and LookupImage, which finds such an
// app only by its image.
package cafeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	defaultFeedURL        = "https://raw.githubusercontent.com/Squidly271/AppFeed/master/applicationFeed.json"
	defaultLastUpdatedURL = "https://raw.githubusercontent.com/Squidly271/AppFeed/master/applicationFeed-lastUpdated.json"
	cacheFile             = "ca-feed.json"
	prevCacheFile         = "ca-feed.prev.json"
)

// Entry is one app listing from the feed.
type Entry struct {
	Name             string `json:"Name"`
	Repository       string `json:"Repository"`
	TemplateURL      string `json:"TemplateURL"`
	Deprecated       bool   `json:"Deprecated"`
	ModeratorComment string `json:"ModeratorComment"`
}

type rawFeed struct {
	AppList     []Entry           `json:"applist"`
	Blacklisted map[string]string `json:"blacklisted"`
	// Repositories stays raw, so a change in its shape loses only this list
	// and not the whole feed.
	Repositories         json.RawMessage `json:"repositories"`
	LastUpdatedTimestamp int64           `json:"last_updated_timestamp"`
}

type rawLastUpdated struct {
	LastUpdatedTimestamp int64 `json:"last_updated_timestamp"`
}

// Feed is a parsed, matchable snapshot of the catalog.
type Feed struct {
	byName        map[string][]Entry
	byRepo        map[string][]Entry // normalized "owner/repo"; several templates can wrap one image
	byTemplateURL map[string]Entry
	templateRepos map[string]struct{} // templateRepoKey of every repository CA crawls
	blacklisted   map[string]string   // normalized "owner/repo" -> reason
	previous      *Feed               // the crawl before this one, if cached
}

// Result is what the feed says about one container, after cross-crawl
// confirmation of any absence.
type Result struct {
	Listed     bool   // false only once absent from two consecutive crawls
	Deprecated bool   // hidden from CA's default search, still listed and updated
	Note       string // moderator comment or blacklist reason
}

// Fetcher fetches the feed and caches it under dir.
type Fetcher struct {
	dir            string
	client         *http.Client
	feedURL        string
	lastUpdatedURL string
}

// NewFetcher returns a Fetcher for the public feed.
func NewFetcher(dir string) *Fetcher {
	return &Fetcher{
		dir:            dir,
		client:         &http.Client{Timeout: 30 * time.Second},
		feedURL:        defaultFeedURL,
		lastUpdatedURL: defaultLastUpdatedURL,
	}
}

// Load returns the current feed and refreshes the cache when upstream changed.
// It fails only when the fetch failed and nothing is cached; callers then skip
// the CA checks for that sweep.
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
		return parse(cached, prevPath), nil
	}

	fresh, fetchErr := f.fetchBytes(ctx, f.feedURL)
	if fetchErr != nil {
		if cacheErr != nil {
			return nil, fmt.Errorf("cafeed: fetch failed and nothing cached: %w", fetchErr)
		}
		return parse(cached, prevPath), nil
	}
	// The replaced crawl becomes the previous one that confirms an absence. A
	// failed write only costs the next sweep its cache.
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

// parse builds a Feed and attaches the previous crawl when one is readable.
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
	byName, byRepo, byTemplateURL := indexEntries(raw.AppList)
	blacklisted := make(map[string]string, len(raw.Blacklisted))
	for repo, reason := range raw.Blacklisted {
		blacklisted[normalizeRepo(repo)] = reason
	}
	return &Feed{
		byName: byName, byRepo: byRepo, byTemplateURL: byTemplateURL,
		templateRepos: indexTemplateRepos(raw.AppList, raw.Repositories),
		blacklisted:   blacklisted,
	}
}

func indexEntries(entries []Entry) (byName, byRepo map[string][]Entry, byTemplateURL map[string]Entry) {
	byName = make(map[string][]Entry, len(entries))
	byRepo = make(map[string][]Entry, len(entries))
	byTemplateURL = make(map[string]Entry, len(entries))
	for _, e := range entries {
		if n := normalizeName(e.Name); n != "" {
			byName[n] = append(byName[n], e)
		}
		if r := normalizeRepo(e.Repository); r != "" {
			byRepo[r] = append(byRepo[r], e)
		}
		if e.TemplateURL != "" {
			byTemplateURL[e.TemplateURL] = e
		}
	}
	return byName, byRepo, byTemplateURL
}

// Lookup reports what the feed says about a container. ok=false means
// inconclusive, such as an ambiguous name, an absence the previous crawl
// cannot confirm yet or an absence from a repository CA does not crawl;
// callers then leave the container's CA state alone.
func (f *Feed) Lookup(name, repo, templateURL string) (Result, bool) {
	n := normalizeName(name)
	matches := f.byName[n]
	var e Entry
	switch len(matches) {
	case 0:
		// The container name is whatever the user typed and often differs from
		// the feed's Name, so the repo and template URL get a chance first.
		found := false
		if e, found = f.matchByIdentity(repo, templateURL); !found {
			if f.previous == nil {
				return Result{}, false
			}
			if len(f.previous.byName[n]) > 0 {
				return Result{}, false
			}
			if _, prevFound := f.previous.matchByIdentity(repo, templateURL); prevFound {
				return Result{}, false
			}
			if !f.CrawlsTemplate(templateURL) {
				return Result{}, false
			}
			return Result{Listed: false}, true
		}
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

// LookupImage finds an app whose template comes from a repository CA does not
// crawl, such as an old copy of a repository that moved. Only the image ties
// such an app to a listing; the name picks among several templates of one
// image. found=false means neither crawl lists the image, which proves nothing
// on its own, because the user's own templates are never listed.
func (f *Feed) LookupImage(name, repo string) (res Result, found bool) {
	nr := normalizeRepo(repo)
	if nr == "" {
		return Result{}, false
	}
	if reason, blacklisted := f.blacklisted[nr]; blacklisted {
		return Result{Listed: false, Note: reason}, true
	}
	candidates := f.byRepo[nr]
	if len(candidates) == 0 {
		if f.previous != nil && len(f.previous.byRepo[nr]) > 0 {
			return Result{Listed: true}, true
		}
		return Result{}, false
	}
	e := candidates[0]
	for _, c := range candidates {
		if normalizeName(c.Name) == normalizeName(name) {
			e = c
			break
		}
	}
	return Result{Listed: true, Deprecated: e.Deprecated, Note: e.ModeratorComment}, true
}

// indexTemplateRepos collects the repositories CA crawls from both the
// repositories list and the template URLs of the listed apps, so either source
// alone is enough.
func indexTemplateRepos(entries []Entry, repositories json.RawMessage) map[string]struct{} {
	repos := make(map[string]struct{}, 2048)
	for _, e := range entries {
		if k, ok := templateRepoKey(e.TemplateURL); ok {
			repos[k] = struct{}{}
		}
	}
	var listed map[string]struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(repositories, &listed) == nil {
		for _, r := range listed {
			if k, ok := templateRepoKey(r.URL); ok {
				repos[k] = struct{}{}
			}
		}
	}
	return repos
}

// templateRepoKey reduces a template or repository URL on GitHub or GitLab, the
// two hosts CA crawls, to the lower-cased "host/owner/repo" it lives in. A
// GitLab project path may nest groups; it ends at the "/-/" marker, at a
// raw/blob/tree segment in older URLs, or at the end of a bare project URL.
func templateRepoKey(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(s, "https://"):
		s = s[len("https://"):]
	case strings.HasPrefix(s, "http://"):
		s = s[len("http://"):]
	default:
		return "", false
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	host, path, _ := strings.Cut(s, "/")
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	segs := strings.FieldsFunc(strings.ToLower(path), func(r rune) bool { return r == '/' })
	switch host {
	case "raw.githubusercontent.com", "github.com":
		host = "github.com"
		segs = segs[:min(len(segs), 2)]
	case "gitlab.com":
		end := len(segs)
		if i := slices.Index(segs, "-"); i >= 0 {
			end = i
		} else if i := slices.IndexFunc(segs[min(len(segs), 2):], func(p string) bool {
			return p == "raw" || p == "blob" || p == "tree"
		}); i >= 0 {
			end = i + 2
		}
		segs = segs[:end]
	default:
		return "", false
	}
	if len(segs) < 2 {
		return "", false
	}
	segs[len(segs)-1] = strings.TrimSuffix(segs[len(segs)-1], ".git")
	return host + "/" + strings.Join(segs, "/"), true
}

// CrawlsTemplate reports whether CA crawls the repository templateURL lives
// in, judged on the current crawl so the answer cannot flap between crawls.
// Without it no CA verdict about the app holds.
func (f *Feed) CrawlsTemplate(templateURL string) bool {
	key, ok := templateRepoKey(templateURL)
	if !ok {
		return false
	}
	_, crawled := f.templateRepos[key]
	return crawled
}

// matchByIdentity finds an entry by repository, then by exact template URL.
func (f *Feed) matchByIdentity(repo, templateURL string) (Entry, bool) {
	if repo != "" {
		if candidates := f.byRepo[normalizeRepo(repo)]; len(candidates) > 0 {
			return candidates[0], true
		}
	}
	if templateURL != "" {
		if e, ok := f.byTemplateURL[templateURL]; ok {
			return e, true
		}
	}
	return Entry{}, false
}

// disambiguate picks among same-named entries in the order CA's exec.php
// uses: repository prefix first, then the exact template URL.
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

// normalizeRepo reduces repo to "owner/repo" without registry host, "library/"
// namespace, tag or digest. CA often lists the ghcr.io mirror of an image
// installed from docker.io, and sometimes bakes a tag into Repository.
func normalizeRepo(repo string) string {
	parts := strings.Split(repo, "/")
	if len(parts) > 2 && strings.Contains(parts[0], ".") {
		parts = parts[1:]
	}
	if len(parts) > 1 && parts[0] == "library" {
		parts = parts[1:]
	}
	if last := len(parts) - 1; last >= 0 {
		if i := strings.IndexAny(parts[last], ":@"); i >= 0 {
			parts[last] = parts[last][:i]
		}
	}
	return strings.Join(parts, "/")
}
