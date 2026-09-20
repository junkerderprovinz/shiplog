// Package engine runs the background poll loop that turns discovered containers
// into stored update statuses: collector → resolver → risk → changelog → store.
package engine

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/cafeed"
	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/resolver"
	"github.com/junkerderprovinz/shiplog/internal/risk"
	"github.com/junkerderprovinz/shiplog/internal/sources"
	"github.com/junkerderprovinz/shiplog/internal/templates"
)

// versionLike matches a tag that starts like a version number ("1.8", "v2.3.1"),
// the same shape the bubble uses to decide a tag is a real version.
var versionLike = regexp.MustCompile(`^v?\d+\.\d+`)

type (
	// Collector lists the containers on the host.
	Collector interface {
		List(ctx context.Context) ([]model.Container, error)
	}
	// Resolver finds the newest tags and digests of an image, as
	// resolver.Resolver does.
	Resolver interface {
		Resolve(ctx context.Context, repo, tag, curDigest string) (newestTag, sameTagDigest, newestVerTag, newestVerDigest string, err error)
	}
	// Changelogger resolves the changelog for a version span.
	Changelogger interface {
		Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool)
	}
	// Storer keeps the status rows and the user's overrides and suppressions.
	Storer interface {
		Upsert(model.UpdateStatus) error
		Get(id string) (model.UpdateStatus, error)
		SourceOverrides() (map[string]string, error)
		SuppressedUnmaintained() (map[string]bool, error)
		Delete(id string) error
		List() ([]model.UpdateStatus, error)
	}
	// Summarizer condenses a raw changelog into an AI summary.
	Summarizer interface {
		Summarize(ctx context.Context, c model.Container, fromTag, toTag, raw string) (*model.AISummary, bool)
	}
	// Notifier announces a newly sighted update.
	Notifier interface {
		Notify(ctx context.Context, st model.UpdateStatus) error
	}
	// caFeedLookuper lets tests fake a *cafeed.Feed.
	caFeedLookuper interface {
		Lookup(name, repo, templateURL string) (cafeed.Result, bool)
	}
)

// Engine wires the collaborators and owns the poll schedule.
type Engine struct {
	collector  Collector
	resolver   Resolver
	changelog  Changelogger
	store      Storer
	summarizer Summarizer
	notifier   Notifier
	interval   time.Duration
	workers    int
	refresh    chan struct{}
	now        func() time.Time

	ignoreUnmanaged bool
	projectPages    func() map[string]string // container name -> template <Project>
	templateURLs    func() map[string]string // container name -> template <TemplateURL>
	// checkURL returns the HTTP status of a GET, or 0 on a transport error.
	checkURL func(ctx context.Context, url string) int
	// caFeed loads the Community Applications catalog once per sweep. Unlike the
	// template URL probe it also sees an app that CA demoted or blacklisted.
	caFeed func(ctx context.Context) (caFeedLookuper, error)
}

// WithCAFeed checks apps against the Community Applications catalog, cached
// under dir.
func (e *Engine) WithCAFeed(dir string) *Engine {
	fetcher := cafeed.NewFetcher(dir)
	e.caFeed = func(ctx context.Context) (caFeedLookuper, error) {
		feed, err := fetcher.Load(ctx)
		if err != nil {
			// A nil *cafeed.Feed inside the interface would pass Sweep's nil
			// check and panic in Lookup.
			return nil, err
		}
		return feed, nil
	}
	return e
}

// WithSummarizer enables AI changelog summaries.
func (e *Engine) WithSummarizer(s Summarizer) *Engine {
	e.summarizer = s
	return e
}

// WithNotifier enables update notifications.
func (e *Engine) WithNotifier(n Notifier) *Engine {
	e.notifier = n
	return e
}

// WithIgnoreUnmanaged makes the sweep skip and forget containers that were not
// created from an Unraid template.
func (e *Engine) WithIgnoreUnmanaged(v bool) *Engine {
	e.ignoreUnmanaged = v
	return e
}

// New builds an Engine. Its worker pool stays small, because most containers
// of a sweep share one registry host, which answers a wide fan-out with 429s.
func New(c Collector, r Resolver, cl Changelogger, s Storer, interval time.Duration) *Engine {
	return &Engine{
		collector: c, resolver: r, changelog: cl, store: s,
		interval: interval,
		workers:  3,
		refresh:  make(chan struct{}, 1),
		now:      time.Now,
		projectPages: func() map[string]string {
			return templates.ProjectPages(templates.Dir)
		},
		templateURLs: func() map[string]string {
			return templates.TemplateURLs(templates.Dir)
		},
		checkURL: defaultCheckURL,
	}
}

// availClient has a short timeout so a slow host cannot hold up a sweep.
var availClient = &http.Client{Timeout: 8 * time.Second}

// defaultCheckURL uses GET because not every host answers HEAD, and template
// files are small.
func defaultCheckURL(ctx context.Context, url string) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "shiplog")
	resp, err := availClient.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// Sweep checks every container and stores its status. A failing container ends
// up in its row's Error; only a failure to list the containers is returned.
func (e *Engine) Sweep(ctx context.Context) error {
	containers, err := e.collector.List(ctx)
	if err != nil {
		return err
	}
	// A failed read of these settings must not abort the sweep; without them a
	// muted warning reappears for one sweep at worst.
	overrides, oerr := e.store.SourceOverrides()
	if oerr != nil {
		overrides = nil
	}
	suppressed, serr := e.store.SuppressedUnmaintained()
	if serr != nil {
		suppressed = nil
	}
	var projects map[string]string
	if e.projectPages != nil {
		projects = e.projectPages()
	}
	var templateURLs map[string]string
	if e.templateURLs != nil {
		templateURLs = e.templateURLs()
	}
	// Without the feed this sweep skips the CA check; the other signals still run.
	var caFeed caFeedLookuper
	if e.caFeed != nil {
		caFeed, _ = e.caFeed(ctx)
	}
	resolve := memoResolve(e.resolver)
	sem := make(chan struct{}, e.workers)
	var wg sync.WaitGroup
	for _, c := range containers {
		if e.ignoreUnmanaged && !c.Managed {
			_ = e.store.Delete(c.ID)
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c model.Container) {
			defer wg.Done()
			defer func() { <-sem }()
			st := e.check(ctx, c, resolve, overrides, suppressed, projects, templateURLs, caFeed)
			// A row that fails to store is retried by the next sweep.
			_ = e.store.Upsert(st)
		}(c)
	}
	wg.Wait()

	// A container that was removed or recreated under a new ID would otherwise
	// leave its old row behind for good.
	live := make(map[string]struct{}, len(containers))
	for _, c := range containers {
		live[c.ID] = struct{}{}
	}
	if rows, lerr := e.store.List(); lerr == nil {
		for _, r := range rows {
			if _, ok := live[r.Container.ID]; !ok {
				_ = e.store.Delete(r.Container.ID)
			}
		}
	}
	return nil
}

type resolveFunc func(ctx context.Context, repo, tag, curDigest string) (newestTag, sameTagDigest, newestVerTag, newestVerDigest string, err error)

// memoResolve makes containers that share an image share one lookup per sweep.
// The key leaves out curDigest, which Resolve ignores.
func memoResolve(r Resolver) resolveFunc {
	type result struct {
		newestTag, sameTagDigest, newestVerTag, newestVerDigest string
		err                                                     error
	}
	type entry struct {
		once sync.Once
		res  result
	}
	var mu sync.Mutex
	memo := map[string]*entry{}
	return func(ctx context.Context, repo, tag, curDigest string) (string, string, string, string, error) {
		key := repo + "|" + tag
		mu.Lock()
		en, ok := memo[key]
		if !ok {
			en = &entry{}
			memo[key] = en
		}
		mu.Unlock()
		en.once.Do(func() {
			var r_ result
			r_.newestTag, r_.sameTagDigest, r_.newestVerTag, r_.newestVerDigest, r_.err = r.Resolve(ctx, repo, tag, curDigest)
			en.res = r_
		})
		res := en.res
		return res.newestTag, res.sameTagDigest, res.newestVerTag, res.newestVerDigest, res.err
	}
}

// noUpstreamReason reports why c has no registry counterpart to check.
func noUpstreamReason(c model.Container) (string, bool) {
	switch {
	case c.PinnedDigest != "":
		// Docker runs the pinned digest even when the ref also names a tag, so an
		// update could not be applied until the pin changes.
		return "pinned by digest, updates only when the pin changes", true
	case c.IsLocal:
		// A locally built or loaded image may share its name with an unrelated
		// registry repository.
		return "no registry digest (built or loaded locally), not checked against a registry", true
	case c.Repo == "":
		return "referenced by image ID, no tag to compare", true
	}
	return "", false
}

func (e *Engine) check(ctx context.Context, c model.Container, resolve resolveFunc, overrides map[string]string, suppressed map[string]bool, projects map[string]string, templateURLs map[string]string, caFeed caFeedLookuper) model.UpdateStatus {
	// A suppression silences every Unmaintained signal below. It is keyed by
	// repo like the source overrides, so it works for containers without a
	// template too.
	silenced := suppressed[c.Repo]
	src, srcKind := sources.Resolve(c.Repo, c.Source, overrides, projects[strings.ToLower(c.Name)])
	c.Source = src

	st := model.UpdateStatus{Container: c, CheckedAt: e.now()}

	if reason, ok := noUpstreamReason(c); ok {
		st.Kind, st.Risk, st.RiskReason = model.KindNone, model.RiskNone, reason
		switch {
		case isVersion(c.Tag):
			st.RunningVersion = c.Tag
		case isVersion(c.ImageVersion):
			st.RunningVersion = c.ImageVersion
		}
		// The source label still points at the release notes of the running
		// version. Without an update there is no summary and no notification.
		if cl, ok := e.changelog.Get(ctx, c, st.RunningVersion, st.RunningVersion); ok {
			st.Changelog = cl
		}
		return st
	}

	// The prior row remembers the running version and dedupes notifications.
	prior, hasPrior := model.UpdateStatus{}, false
	if p, err := e.store.Get(c.ID); err == nil {
		prior, hasPrior = p, true
	}

	newestTag, newestDigest, newestVerTag, newestVerDigest, err := resolve(ctx, c.Repo, c.Tag, c.Digest)
	if err != nil {
		if errors.Is(err, resolver.ErrRepoNotFound) {
			st.Kind, st.Risk, st.RiskReason = model.KindNone, model.RiskNone, "image removed from the registry"
			if !silenced {
				st.Unmaintained, st.UnmaintainedReason = true, "Image no longer in the registry"
			}
			switch {
			case hasPrior && prior.RunningVersion != "":
				st.RunningVersion = prior.RunningVersion
			case isVersion(c.Tag):
				st.RunningVersion = c.Tag
			case isVersion(c.ImageVersion):
				st.RunningVersion = c.ImageVersion
			}
			e.maybeNotifyUnmaintained(ctx, st, prior, hasPrior)
			return st
		}
		// A transient failure such as a 429 burst keeps the last verdict. Its
		// CheckedAt stays too, since the data may be an hour old after a backoff.
		if hasPrior && prior.Kind != "" && prior.Kind != model.KindUnknown {
			prior.Container = c
			prior.Error = ""
			return prior
		}
		st.Error = "could not resolve upstream: " + err.Error()
		st.Kind = model.KindUnknown
		st.Risk = model.RiskUnknown
		st.RiskReason = "upstream lookup failed"
		switch {
		case hasPrior && prior.RunningVersion != "":
			st.RunningVersion = prior.RunningVersion
		case isVersion(c.Tag):
			st.RunningVersion = c.Tag
		case isVersion(c.ImageVersion):
			st.RunningVersion = c.ImageVersion
		}
		return st
	}
	st.NewestTag = newestTag
	st.NewestDigest = newestDigest
	st.RunningVersion = decideRunningVersion(c, newestVerTag, newestVerDigest, prior, hasPrior)

	// A mirror pull carries one digest per registry, and matching any of them
	// means the image is current.
	curDigest := c.Digest
	if c.HasDigest(newestDigest) {
		curDigest = newestDigest
	}
	kind, level, reason := risk.Classify(c.Tag, newestTag, curDigest, newestDigest)
	// "latest" against "latest" reads as a low-risk digest move, so two resolved
	// versions classify the real jump instead.
	if isVersion(st.RunningVersion) && isVersion(newestVerTag) {
		if vk, vl, vr := risk.Classify(st.RunningVersion, newestVerTag, "", ""); vk != model.KindUnknown && vk != model.KindNone {
			kind, level, reason = vk, vl, vr
		}
	}
	st.Kind, st.Risk, st.RiskReason = kind, level, reason

	// Every container gets a changelog, the running version's notes when it is up
	// to date. Resolved versions replace rolling tags, so the span includes the
	// intermediate releases where breaking notes tend to hide.
	clFrom, clTo := c.Tag, newestTag
	if isVersion(st.RunningVersion) {
		clFrom = st.RunningVersion
	}
	if isVersion(newestVerTag) {
		clTo = newestVerTag
	}
	if cl, ok := e.changelog.Get(ctx, c, clFrom, clTo); ok {
		st.Changelog = cl
		if cl.Provider == "github" {
			switch srcKind {
			case sources.KindOverride:
				cl.Source = "GitHub releases (manual source)"
			case sources.KindCurated:
				cl.Source = "GitHub releases (curated source)"
			case sources.KindProject:
				cl.Source = "GitHub releases (template project page)"
			}
		}
		// Release notes can demand a migration that the version delta does not
		// show. They only ever raise the verdict.
		if st.HasUpdate() {
			if reason, breaking := risk.ScanBreaking(cl, st.RunningVersion); breaking && model.RiskCritical.MoreSevere(st.Risk) {
				st.Risk, st.RiskReason = model.RiskCritical, reason
			}
		}
		if st.HasUpdate() {
			if e.summarizer != nil && cl.Raw != "" {
				if sum, ok := e.summarizer.Summarize(ctx, c, c.Tag, newestTag, cl.Raw); ok {
					cl.Summary = sum
				} else {
					log.Printf("shiplog: no AI summary for %s (Ollama configured but returned nothing)", c.Name)
				}
			}
			e.maybeNotify(ctx, st, prior, hasPrior)
		}
	}
	// An up-to-date app can still be at a dead end: its template was pulled from
	// Community Applications or its source repo was archived.
	if !st.Unmaintained && !silenced {
		u := templateURLs[strings.ToLower(c.Name)]
		switch {
		case st.Changelog != nil && st.Changelog.Deprecated:
			// An archived repo has no false positives, so it wins over a mere
			// demotion in the feed.
			st.Unmaintained, st.UnmaintainedReason = true, "Source repository archived"
		default:
			// A conclusive feed answer beats the template URL probe, because a
			// template file also disappears when its repo or branch moves while
			// the app stays listed. Both checks apply only to apps that came from
			// CA: one without a template URL, or with a source override, is the
			// user's own and would otherwise always read as removed.
			_, selfSourced := overrides[c.Repo]
			feedConclusive := false
			if c.Managed && caFeed != nil && u != "" && !selfSourced {
				if res, ok := caFeed.Lookup(c.Name, c.Repo, u); ok {
					feedConclusive = true
					switch {
					case !res.Listed && res.Note != "":
						st.Unmaintained, st.UnmaintainedReason = true, "Removed from Community Applications: "+res.Note
					case !res.Listed:
						st.Unmaintained, st.UnmaintainedReason = true, "Removed from Community Applications"
					case res.Deprecated:
						st.CADeprecated, st.CADeprecatedNote = true, res.Note
					}
				}
			}
			if !feedConclusive && c.Managed && u != "" && !selfSourced && e.checkURL != nil && e.checkURL(ctx, u) == http.StatusNotFound && e.repoGone(ctx, u) {
				st.Unmaintained, st.UnmaintainedReason = true, "Removed from Community Applications"
			}
		}
	}
	e.maybeNotifyUnmaintained(ctx, st, prior, hasPrior)
	return st
}

// repoGone confirms a template URL 404. A raw.githubusercontent.com URL also
// 404s when its repo is renamed (raw URLs do not follow the redirect) or the
// file moved, so the repo page has to answer 404 or 410 as well. Any other
// answer, including a rate limit or a failed request, keeps the app listed. A
// 404 from another host is trusted as it is.
func (e *Engine) repoGone(ctx context.Context, templateURL string) bool {
	repoURL, ok := githubRepoURL(templateURL)
	if !ok || e.checkURL == nil {
		return true
	}
	switch e.checkURL(ctx, repoURL) {
	case http.StatusNotFound, http.StatusGone:
		return true
	default:
		return false
	}
}

// githubRepoURL maps a raw.githubusercontent.com URL to its github.com repo.
func githubRepoURL(u string) (string, bool) {
	const prefix = "https://raw.githubusercontent.com/"
	if !strings.HasPrefix(u, prefix) {
		return "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(u, prefix), "/", 4)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return "https://github.com/" + parts[0] + "/" + parts[1], true
}

// maybeNotifyUnmaintained notifies once when a known container becomes
// unmaintained. The first sighting only seeds the row.
func (e *Engine) maybeNotifyUnmaintained(ctx context.Context, st, prior model.UpdateStatus, hasPrior bool) {
	if e.notifier == nil || !hasPrior || !st.Unmaintained {
		return
	}
	if prior.Unmaintained {
		return
	}
	if err := e.notifier.Notify(ctx, st); err != nil {
		log.Printf("shiplog: unmaintained notify failed for %s: %v", st.Container.Name, err)
	}
}

// decideRunningVersion returns a pinned version tag as it is. For a rolling tag
// it takes, in order of evidence: the newest version when the running digest
// matches it, the remembered version while the digest is unchanged, and the
// image's version label. An unlabelled, outdated image stays unknown.
func decideRunningVersion(c model.Container, newestVerTag, newestVerDigest string, prior model.UpdateStatus, hasPrior bool) string {
	if isVersion(c.Tag) {
		return c.Tag
	}
	if c.HasDigest(newestVerDigest) {
		return newestVerTag
	}
	if hasPrior && c.Digest != "" && prior.Container.Digest == c.Digest && prior.RunningVersion != "" {
		return prior.RunningVersion
	}
	if isVersion(c.ImageVersion) {
		return c.ImageVersion
	}
	return ""
}

func isVersion(tag string) bool { return versionLike.MatchString(tag) }

// maybeNotify notifies when a known container gets an update target it did not
// have before. The first sighting only seeds the row.
func (e *Engine) maybeNotify(ctx context.Context, st model.UpdateStatus, prior model.UpdateStatus, hasPrior bool) {
	if e.notifier == nil || !hasPrior {
		return
	}
	if prior.HasUpdate() && prior.NewestDigest == st.NewestDigest {
		return
	}
	_ = e.notifier.Notify(ctx, st)
}

// Refresh asks for an extra sweep without blocking; pending requests coalesce.
func (e *Engine) Refresh() {
	select {
	case e.refresh <- struct{}{}:
	default:
	}
}

// Run sweeps once immediately, then on the interval, plus whenever Refresh is
// called. It returns when ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	_ = e.Sweep(ctx)
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = e.Sweep(ctx)
		case <-e.refresh:
			_ = e.Sweep(ctx)
		}
	}
}
