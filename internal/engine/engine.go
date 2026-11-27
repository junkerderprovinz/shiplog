// Package engine runs the background poll loop that turns discovered containers
// into stored update statuses: collector → resolver → risk → changelog → store.
package engine

import (
	"context"
	"errors"
	"fmt"
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
	// resolver.Resolver does. The newest version tag is only the highest one;
	// it names what a rolling tag serves only when its digest matches.
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
	// summaryWriter is a Summarizer that can tell whether a kept summary
	// came from the model it uses now.
	summaryWriter interface {
		Writes(model string) bool
	}
	// Notifier announces a newly sighted update.
	Notifier interface {
		Notify(ctx context.Context, st model.UpdateStatus) error
	}
	// VulnChecker counts the known vulnerabilities of a container's running
	// image and what the image behind newestDigest closes of them, as
	// vulns.Checker does.
	VulnChecker interface {
		Check(ctx context.Context, c model.Container, newestDigest string) *model.Vulns
	}
	// idLister lists the containers without inspecting their images, which is
	// all the check for new containers needs. A collector without it is listed
	// in full.
	idLister interface {
		ListIDs(ctx context.Context) ([]model.Container, error)
	}
	// caFeedLookuper lets tests fake a *cafeed.Feed.
	caFeedLookuper interface {
		Lookup(name, repo, templateURL string) (cafeed.Result, bool)
		LookupImage(name, repo string) (cafeed.Result, bool)
		CrawlsTemplate(templateURL string) bool
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
	vulns      VulnChecker
	interval   time.Duration
	workers    int
	refresh    chan struct{}
	now        func() time.Time
	// batched sends the updates a sweep finds as one message once the sweep is
	// done, when the notifier can, instead of one message per container.
	batched bool
	// minRisk holds back the notice of an update below this risk. Empty sends
	// every one.
	minRisk model.RiskLevel
	// watch is how often Run looks for a container no sweep has checked yet
	// and whose start event it missed.
	watch time.Duration
	// asked holds the unchecked containers that were already checked on their
	// own, so one whose row cannot be stored is not checked again on every look
	// or event. Only Run touches it.
	asked map[string]struct{}
	// started carries the containers Started picked from the event stream to
	// Run. A full queue drops one, and the next look picks it up.
	started chan string

	ignoreUnmanaged bool
	// autoApplies reports whether the scheduled auto-update run will apply an
	// update. Without a run it stays nil.
	autoApplies  func(model.UpdateStatus) bool
	projectPages func() map[string]string // container name -> template <Project>
	templateURLs func() map[string]string // container name -> template <TemplateURL>
	// checkURL returns the HTTP status of a GET, or 0 on a transport error.
	checkURL func(ctx context.Context, url string) int
	// repoStatus is checkURL without following redirects, so a moved
	// repository shows as one.
	repoStatus func(ctx context.Context, url string) int
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

// WithVulns enables the check for known vulnerabilities.
func (e *Engine) WithVulns(v VulnChecker) *Engine {
	e.vulns = v
	return e
}

// WithNotifier enables update notifications.
func (e *Engine) WithNotifier(n Notifier) *Engine {
	e.notifier = n
	return e
}

// WithBatch makes each sweep announce its new updates in one message.
func (e *Engine) WithBatch(on bool) *Engine {
	e.batched = on
	return e
}

// WithMinRisk sends no notice for an update below min. An update whose risk
// is unknown is always announced, since nothing says it is harmless.
func (e *Engine) WithMinRisk(min model.RiskLevel) *Engine {
	e.minRisk = min
	return e
}

// WithAutoApply tells the engine which updates the scheduled auto-update run will
// apply.
func (e *Engine) WithAutoApply(applies func(model.UpdateStatus) bool) *Engine {
	e.autoApplies = applies
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
		watch:    time.Minute,
		asked:    map[string]struct{}{},
		started:  make(chan string, 16),
		projectPages: func() map[string]string {
			return templates.ProjectPages(templates.Dir)
		},
		templateURLs: func() map[string]string {
			return templates.TemplateURLs(templates.Dir)
		},
		checkURL:   defaultCheckURL,
		repoStatus: defaultRepoStatus,
	}
}

// availClient has a short timeout so a slow host cannot hold up a sweep.
var availClient = &http.Client{Timeout: 8 * time.Second}

// repoClient answers a redirect itself instead of following it.
var repoClient = &http.Client{
	Timeout:       8 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func defaultCheckURL(ctx context.Context, url string) int { return getStatus(ctx, availClient, url) }

func defaultRepoStatus(ctx context.Context, url string) int { return getStatus(ctx, repoClient, url) }

// getStatus uses GET because not every host answers HEAD, and template files
// are small. A transport error reads as 0.
func getStatus(ctx context.Context, client *http.Client, url string) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "shiplog")
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// Sweep checks every container and stores its status. A failing container ends
// up in its row's Error; only a failure to list the containers is returned.
func (e *Engine) Sweep(ctx context.Context) error {
	return e.sweep(ctx, nil)
}

// sweep checks the containers whose ID is in only, or all of them when only is
// nil. Either way it drops stale rows, which costs no registry request.
func (e *Engine) sweep(ctx context.Context, only map[string]struct{}) error {
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
	// Without the feed nothing tells a CA app from the user's own, so this sweep
	// gives no CA verdict at all; the archived-repo check still runs.
	var caFeed caFeedLookuper
	if e.caFeed != nil {
		caFeed, _ = e.caFeed(ctx)
	}
	resolve := memoResolve(e.resolver)
	var found *batch
	if d, ok := e.notifier.(batcher); ok && e.batched {
		found = &batch{}
		ctx = context.WithValue(ctx, batchKey{}, found)
		defer func() {
			sts := found.take()
			if len(sts) == 0 {
				return
			}
			// A settings save restarts the engine and cancels ctx mid-sweep.
			// The rows are stored by now, so a later sweep would not announce
			// these updates again: send them anyway, within a short bound.
			fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), batchFlushTimeout)
			defer cancel()
			if err := d.NotifyBatch(fctx, sts); err != nil {
				log.Printf("shiplog: update batch notify failed: %v", err)
			}
		}()
	}
	sem := make(chan struct{}, e.workers)
	var wg sync.WaitGroup
	var tally vulnTally
	for _, c := range containers {
		if e.ignoreUnmanaged && !c.Managed {
			_ = e.store.Delete(c.ID)
			continue
		}
		if _, ok := only[c.ID]; only != nil && !ok {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c model.Container) {
			defer wg.Done()
			defer func() { <-sem }()
			st := e.check(ctx, c, resolve, overrides, suppressed, projects, templateURLs, caFeed)
			tally.add(st.Vulns)
			// A row that fails to store is retried by the next sweep.
			_ = e.store.Upsert(st)
		}(c)
	}
	wg.Wait()
	if e.vulns != nil {
		log.Printf("shiplog: %s", tally.String())
	}

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

// vulnTally counts how a sweep checked the images, so the log shows that
// the check ran even when no count changed.
type vulnTally struct {
	mu                                 sync.Mutex
	all, image, system, unread, failed int
}

func (t *vulnTally) add(v *model.Vulns) {
	if v == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.all++
	switch {
	case v.Scope == model.VulnScopeImage:
		t.image++
	case v.Scope == model.VulnScopeSystem:
		t.system++
	case v.Reason == model.VulnNoPackages:
		t.unread++
	default:
		t.failed++
	}
}

func (t *vulnTally) String() string {
	return fmt.Sprintf("vulnerability check: %d containers, %d by the image's package list, %d by the system packages, %d without a package list, %d failed",
		t.all, t.image, t.system, t.unread, t.failed)
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
func noUpstreamReason(c model.Container) (model.Reason, bool) {
	switch {
	case c.PinnedDigest != "":
		// Docker runs the pinned digest even when the ref also names a tag, so an
		// update could not be applied until the pin changes.
		return model.Why(model.ReasonDigestPinned), true
	case c.IsLocal:
		// A locally built or loaded image may share its name with an unrelated
		// registry repository.
		return model.Why(model.ReasonLocal), true
	case c.Repo == "":
		return model.Why(model.ReasonImageID), true
	}
	return model.Reason{}, false
}

func (e *Engine) check(ctx context.Context, c model.Container, resolve resolveFunc, overrides map[string]string, suppressed map[string]bool, projects map[string]string, templateURLs map[string]string, caFeed caFeedLookuper) model.UpdateStatus {
	// A suppression silences every Unmaintained signal below. It is keyed by
	// repo like the source overrides, so it works for containers without a
	// template too.
	silenced := suppressed[c.Repo]
	src, srcKind := sources.Resolve(c.Repo, c.Source, overrides, projects[strings.ToLower(c.Name)])
	if srcKind == sources.KindFile {
		c.ChangelogFile = src
	} else {
		c.Source = src
	}

	st := model.UpdateStatus{Container: c, CheckedAt: e.now()}

	if reason, ok := noUpstreamReason(c); ok {
		st.Kind, st.Risk = model.KindNone, model.RiskNone
		st.SetReason(reason)
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
		if e.vulns != nil {
			st.Vulns = e.vulns.Check(ctx, c, "")
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
			st.Kind, st.Risk = model.KindNone, model.RiskNone
			st.SetReason(model.Why(model.ReasonRemoved))
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
			// The running image is still known, so its check need not wait
			// for the registry.
			if e.vulns != nil {
				newest := ""
				if prior.HasUpdate() {
					newest = prior.NewestDigest
				}
				prior.Vulns = e.vulns.Check(ctx, c, newest)
			}
			return prior
		}
		st.Error = "could not resolve upstream: " + err.Error()
		st.Kind = model.KindUnknown
		st.Risk = model.RiskUnknown
		st.SetReason(model.Why(model.ReasonLookupFailed))
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

	v := classify(c, st.RunningVersion, newestTag, newestDigest, newestVerTag, newestVerDigest)
	st.Kind, st.Risk, st.NewerVersion = v.kind, v.risk, v.newer
	st.SetReason(v.reason)
	if v.newest != "" {
		st.NewestTag = v.newest
	}
	if e.vulns != nil {
		newest := ""
		if st.HasUpdate() {
			newest = st.NewestDigest
		}
		st.Vulns = e.vulns.Check(ctx, c, newest)
		raiseForFixes(&st)
	}

	// Every container gets a changelog, the running version's notes when it is up
	// to date. Resolved versions replace rolling tags, so the span includes the
	// intermediate releases where breaking notes tend to hide; classify only
	// names an end version it can vouch for.
	clFrom, clTo := c.Tag, newestTag
	if isVersion(st.RunningVersion) {
		clFrom = st.RunningVersion
	}
	if v.to != "" {
		clTo = v.to
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
			if reason, breaking := scanBreaking(cl, st.RunningVersion, v.to); breaking && model.RiskCritical.MoreSevere(st.Risk) {
				st.Risk = model.RiskCritical
				st.SetReason(reason)
			}
		}
		if st.HasUpdate() {
			if e.summarizer != nil && cl.Raw != "" {
				if sum := priorSummary(prior, hasPrior, cl, e.summarizer); sum != nil {
					// The notes and the model are unchanged since the last
					// check, so is the summary. Asking again would spend a free model's small
					// daily allowance on every poll.
					cl.Summary = sum
				} else if sum, ok := e.summarizer.Summarize(ctx, c, c.Tag, newestTag, releaseNotes(cl)); ok {
					cl.Summary = sum
				} else {
					log.Printf("shiplog: no AI summary for %s (summaries configured but nothing came back)", c.Name)
				}
			}
			e.maybeNotify(ctx, st, prior, hasPrior)
		}
	}
	e.maybeResolve(ctx, st, prior, hasPrior)
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
			// An app without a template URL or with a source override is the
			// user's own and would otherwise always read as removed.
			_, selfSourced := overrides[c.Repo]
			if !c.Managed || u == "" || selfSourced || caFeed == nil {
				break
			}
			if caFeed.CrawlsTemplate(u) {
				// A conclusive feed answer beats the template URL probe, because
				// a template file also disappears when its repo or branch moves
				// while the app stays listed.
				if res, ok := caFeed.Lookup(c.Name, c.Repo, u); ok {
					applyCAResult(&st, res)
				} else if e.checkURL != nil && e.checkURL(ctx, u) == http.StatusNotFound && e.repoGone(ctx, u) {
					st.Unmaintained, st.UnmaintainedReason = true, removedFromCA
				}
				break
			}
			// A template from a repository CA does not crawl is the user's own or
			// an old copy of a repository that moved. The image still finds a
			// listed app. An unlisted one counts as removed only when its
			// repository moved, because the user's own stays where it is.
			if res, found := caFeed.LookupImage(c.Name, c.Repo); found {
				applyCAResult(&st, res)
			} else if e.repoMoved(ctx, u) {
				st.Unmaintained, st.UnmaintainedReason = true, removedFromCA
			}
		}
	}
	e.maybeNotifyUnmaintained(ctx, st, prior, hasPrior)
	return st
}

// raiseForFixes ranks an update one step higher when it is known to close a
// critical vulnerability. Critical stays reserved for breaking notes, which ask
// for a step before the pull.
func raiseForFixes(st *model.UpdateStatus) {
	if !st.HasUpdate() || !st.Vulns.ClosesCritical() {
		return
	}
	switch st.Risk {
	case model.RiskLow:
		st.Risk, st.VulnRaised = model.RiskMedium, true
	case model.RiskMedium:
		st.Risk, st.VulnRaised = model.RiskHigh, true
	}
}

// scanBreaking reads the release notes that are still ahead of the container.
// Without a running version that is only the release classify vouched for.
func scanBreaking(cl *model.Changelog, running, vouched string) (model.Reason, bool) {
	if running == "" {
		return risk.ScanRelease(cl, vouched)
	}
	return risk.ScanBreaking(cl, running)
}

// priorSummary is the summary the last check kept for the same release notes,
// or nil when the span or its notes changed since, or when s no longer uses the
// model that wrote it.
func priorSummary(prior model.UpdateStatus, hasPrior bool, cl *model.Changelog, s Summarizer) *model.AISummary {
	if !hasPrior || prior.Changelog == nil || prior.Changelog.Summary == nil {
		return nil
	}
	p := prior.Changelog
	if p.FromTag != cl.FromTag || p.ToTag != cl.ToTag || releaseNotes(p) != releaseNotes(cl) {
		return nil
	}
	if w, ok := s.(summaryWriter); ok && !w.Writes(p.Summary.Model) {
		return nil
	}
	return p.Summary
}

// releaseNotes is the summary's input: every release of a version span, since
// Raw holds only the newest one. A rolling tag's recent releases can belong to
// other channels, so it keeps Raw.
func releaseNotes(cl *model.Changelog) string {
	if len(cl.Entries) < 2 || !isVersion(cl.FromTag) || !isVersion(cl.ToTag) {
		return cl.Raw
	}
	var b strings.Builder
	for _, en := range cl.Entries {
		fmt.Fprintf(&b, "## %s\n%s\n\n", en.Tag, strings.TrimSpace(en.Body))
	}
	return b.String()
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

// repoMoved reports whether the GitHub repository of templateURL answers with a
// redirect, which is how GitHub serves a renamed or transferred repository. A
// 404 proves nothing here, since a private repository answers 404 as well.
func (e *Engine) repoMoved(ctx context.Context, templateURL string) bool {
	repoURL, ok := githubRepoURL(templateURL)
	if !ok || e.repoStatus == nil {
		return false
	}
	switch e.repoStatus(ctx, repoURL) {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

const removedFromCA = "Removed from Community Applications"

// applyCAResult records a conclusive answer from the CA feed.
func applyCAResult(st *model.UpdateStatus, res cafeed.Result) {
	switch {
	case !res.Listed && res.Note != "":
		st.Unmaintained, st.UnmaintainedReason = true, removedFromCA+": "+res.Note
	case !res.Listed:
		st.Unmaintained, st.UnmaintainedReason = true, removedFromCA
	case res.Deprecated:
		st.CADeprecated, st.CADeprecatedNote = true, res.Note
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
// image's version label. An unlabelled, outdated image stays unknown, and so
// does a version remembered from a label the image inherited from its base.
func decideRunningVersion(c model.Container, newestVerTag, newestVerDigest string, prior model.UpdateStatus, hasPrior bool) string {
	if isVersion(c.Tag) {
		return c.Tag
	}
	if c.HasDigest(newestVerDigest) {
		return newestVerTag
	}
	if hasPrior && c.Digest != "" && prior.Container.Digest == c.Digest && prior.RunningVersion != "" && prior.RunningVersion != c.BaseVersion {
		return prior.RunningVersion
	}
	if isVersion(c.ImageVersion) {
		return c.ImageVersion
	}
	return ""
}

func isVersion(tag string) bool { return versionLike.MatchString(tag) }

// verdict is what pulling a container's own tag would deliver.
type verdict struct {
	kind   model.Kind
	risk   model.RiskLevel
	reason model.Reason
	newer  string // advisory for a pinned tag; set only with KindNone
	newest string // replaces the resolved newest tag when set
	to     string // end of the changelog span; empty means the newest tag
}

// classify lets only the digest behind the container's own tag decide whether
// there is an update, because that is all a pull can fetch. Versions only size
// a real move or advise about a newer tag: the registry's highest version tag
// is often an old date tag or another release channel.
func classify(c model.Container, running, newestTag, newestDigest, newestVerTag, newestVerDigest string) verdict {
	// A mirror pull carries one digest per registry, and matching any of them
	// means the image is current.
	curDigest := c.Digest
	if c.HasDigest(newestDigest) {
		curDigest = newestDigest
	}
	kind, level, reason := risk.Classify(c.Tag, newestTag, curDigest, newestDigest)
	v := verdict{kind: kind, risk: level, reason: reason}

	switch kind {
	case model.KindNone:
		if isVersion(running) {
			v.to = running
		}

	case model.KindDigest:
		// A rolling tag moved. Its size is known only when the newest version
		// tag is the very image the rolling tag now serves.
		if trusted := trustedVersion(newestDigest, newestVerTag, newestVerDigest); trusted != "" {
			v.to = trusted
			if isVersion(running) {
				if vk, vl, vr := risk.Classify(running, trusted, "", ""); vk != model.KindUnknown && vk != model.KindNone {
					v.kind, v.risk, v.reason = vk, vl, vr
				}
			}
		}

	case model.KindPatch, model.KindMinor, model.KindMajor:
		// The container is pinned to a version tag and a newer one exists, but a
		// pull of the pinned tag cannot reach it.
		if tagMoved(c, newestDigest) {
			v.kind, v.risk, v.reason = model.KindDigest, model.RiskLow, model.Why(model.ReasonDigest)
			v.newest, v.to = c.Tag, c.Tag
			break
		}
		v.kind, v.risk = model.KindNone, model.RiskNone
		v.reason = model.Why(model.ReasonPinnedNewer, c.Tag, newestTag)
		v.newer = newestTag
	}
	return v
}

// trustedVersion returns the newest version tag when it serves the same image
// as the rolling tag, else "".
func trustedVersion(newestDigest, newestVerTag, newestVerDigest string) string {
	if newestVerTag != "" && newestVerDigest != "" && newestVerDigest == newestDigest {
		return newestVerTag
	}
	return ""
}

// tagMoved reports whether the container's own tag now serves another image
// than the one running. With either digest unknown it claims nothing.
func tagMoved(c model.Container, tagDigest string) bool {
	return c.Digest != "" && tagDigest != "" && !c.HasDigest(tagDigest)
}

// closer hears that an update a message offered is applied, as
// notify.Notifier does.
type closer interface {
	Resolve(ctx context.Context, prior model.UpdateStatus)
}

// maybeResolve closes the message about the update prior offered once the
// container runs it, so the notification hub stops counting it as open.
func (e *Engine) maybeResolve(ctx context.Context, st, prior model.UpdateStatus, hasPrior bool) {
	r, ok := e.notifier.(closer)
	if !ok || !hasPrior || !prior.HasUpdate() || st.HasUpdate() {
		return
	}
	r.Resolve(ctx, prior)
}

// batchFlushTimeout bounds the batch a cancelled sweep still sends.
const batchFlushTimeout = 30 * time.Second

// batcher announces several updates in one message, as notify.Notifier does.
type batcher interface {
	NotifyBatch(ctx context.Context, sts []model.UpdateStatus) error
}

// batch collects the updates the workers of one sweep found.
type batch struct {
	mu  sync.Mutex
	sts []model.UpdateStatus
}

func (b *batch) add(st model.UpdateStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sts = append(b.sts, st)
}

func (b *batch) take() []model.UpdateStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	sts := b.sts
	b.sts = nil
	return sts
}

type batchKey struct{}

// heldBack reports whether the minimum risk keeps st's notice back. A held
// back update is announced once a later check finds it at the minimum.
func (e *Engine) heldBack(st model.UpdateStatus) bool {
	return e.minRisk != "" && st.Risk != model.RiskUnknown && e.minRisk.MoreSevere(st.Risk)
}

// maybeNotify notifies when a known container gets an update target it did not
// have before. The first sighting only seeds the row.
func (e *Engine) maybeNotify(ctx context.Context, st model.UpdateStatus, prior model.UpdateStatus, hasPrior bool) {
	if e.notifier == nil || !hasPrior {
		return
	}
	if prior.HasUpdate() && prior.NewestDigest == st.NewestDigest && !e.heldBack(prior) {
		return
	}
	// The run applies it and its summary reports the result, while a notice sent
	// now would sit unread after the update is gone. Breaking notes ask for a step
	// before the pull, so they still go out ahead of the run.
	if e.autoApplies != nil && e.autoApplies(st) && st.Risk != model.RiskCritical {
		return
	}
	if e.heldBack(st) {
		return
	}
	if b, ok := ctx.Value(batchKey{}).(*batch); ok {
		b.add(st)
		return
	}
	_ = e.notifier.Notify(ctx, st)
}

// Started hears that container c started and has Run check it on its own when
// it has no row and is worth checking. It never waits for Run.
func (e *Engine) Started(c model.Container) {
	if _, err := e.store.Get(c.ID); err == nil {
		return
	}
	var names map[string]struct{}
	if !c.Managed {
		rows, err := e.store.List()
		if err != nil {
			return
		}
		names = rowNames(rows)
	}
	if !e.worthChecking(c, names) {
		return
	}
	select {
	case e.started <- c.ID:
	default:
	}
}

// worthChecking reports whether c is checked before the next sweep: Unraid
// manages it, or it runs under a name ShipLog knows, which is how an update
// recreates it. Builds and one-off runs start under made-up names all day and
// wait for the sweep.
func (e *Engine) worthChecking(c model.Container, names map[string]struct{}) bool {
	if c.Managed {
		return true
	}
	if e.ignoreUnmanaged {
		return false
	}
	_, ok := names[c.Name]
	return ok
}

func rowNames(rows []model.UpdateStatus) map[string]struct{} {
	names := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		names[r.Container.Name] = struct{}{}
	}
	return names
}

// Refresh asks for an extra sweep without blocking; pending requests coalesce.
func (e *Engine) Refresh() {
	select {
	case e.refresh <- struct{}{}:
	default:
	}
}

// Run sweeps once immediately, then on the interval, plus whenever Refresh is
// called. A new container that starts in between is checked on its own when
// Started hears of it, or at the next look if its event went missing. It
// returns when ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	_ = e.Sweep(ctx)
	t := time.NewTicker(e.interval)
	defer t.Stop()
	watch := time.NewTicker(e.watch)
	defer watch.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = e.Sweep(ctx)
		case <-e.refresh:
			_ = e.Sweep(ctx)
		case id := <-e.started:
			if _, ok := e.asked[id]; !ok {
				e.asked[id] = struct{}{}
				_ = e.sweep(ctx, map[string]struct{}{id: {}})
			}
		case <-watch.C:
			if ids := e.unchecked(ctx); len(ids) > 0 {
				_ = e.sweep(ctx, ids)
			}
		}
	}
}

// unchecked returns the IDs of the containers worth checking that Docker lists
// without a stored row and that were not checked on their own before. Without
// it a container created or recreated after a sweep whose event went missing has
// no status, and no chip in the Docker tab, until the next interval. It only
// reads the container list and the store.
func (e *Engine) unchecked(ctx context.Context) map[string]struct{} {
	var containers []model.Container
	var err error
	if l, ok := e.collector.(idLister); ok {
		containers, err = l.ListIDs(ctx)
	} else {
		containers, err = e.collector.List(ctx)
	}
	if err != nil {
		return nil
	}
	rows, err := e.store.List()
	if err != nil {
		return nil
	}
	known := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		known[r.Container.ID] = struct{}{}
	}
	names := rowNames(rows)
	fresh := map[string]struct{}{}
	asked := map[string]struct{}{}
	for _, c := range containers {
		if _, ok := known[c.ID]; ok || !e.worthChecking(c, names) {
			continue
		}
		if _, ok := e.asked[c.ID]; !ok {
			fresh[c.ID] = struct{}{}
		}
		asked[c.ID] = struct{}{}
	}
	// Containers that got a row or went away drop out of the set.
	e.asked = asked
	return fresh
}
