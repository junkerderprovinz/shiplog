// Package engine runs the background poll loop that turns discovered containers
// into stored update statuses: collector → resolver → risk → changelog → store.
package engine

import (
	"context"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/risk"
)

// versionLike matches a tag that starts like a version number ("1.8", "v2.3.1"),
// the same shape the bubble uses to decide a tag is a real version.
var versionLike = regexp.MustCompile(`^v?\d+\.\d+`)

// The collaborators are interfaces so the engine stays unit-testable; the real
// dockercli.Client / resolver.Resolver / changelog.Chain / store.Store satisfy them.
type (
	// Collector lists the containers on the host (read-only).
	Collector interface {
		List(ctx context.Context) ([]model.Container, error)
	}
	// Resolver returns, for an image ref: the newest tag (requested tag echoed if
	// non-semver) and its same-tag digest for risk/digest-drift, plus the newest
	// SEMVER version tag and that version's digest (to prove a rolling container's
	// running version).
	Resolver interface {
		Resolve(ctx context.Context, repo, tag, curDigest string) (newestTag, sameTagDigest, newestVerTag, newestVerDigest string, err error)
	}
	// Changelogger resolves the changelog for a version span (handled=false → none).
	Changelogger interface {
		Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool)
	}
	// Storer persists one status row and reads the prior one (for notify dedupe).
	Storer interface {
		Upsert(model.UpdateStatus) error
		Get(id string) (model.UpdateStatus, error)
	}
	// Summarizer optionally condenses a raw changelog into an AI summary.
	Summarizer interface {
		Summarize(ctx context.Context, c model.Container, fromTag, toTag, raw string) (*model.AISummary, bool)
	}
	// Notifier optionally pushes a message when a new update is first sighted.
	Notifier interface {
		Notify(ctx context.Context, st model.UpdateStatus) error
	}
)

// Engine wires the collaborators and owns the poll schedule.
type Engine struct {
	collector  Collector
	resolver   Resolver
	changelog  Changelogger
	store      Storer
	summarizer Summarizer // optional; nil → no AI summaries
	notifier   Notifier   // optional; nil → no notifications
	interval   time.Duration
	workers    int
	refresh    chan struct{}
	now        func() time.Time // injectable clock for tests
}

// WithSummarizer enables AI changelog summaries (returns e for chaining). Pass a
// non-nil summariser only when Ollama is configured.
func (e *Engine) WithSummarizer(s Summarizer) *Engine {
	e.summarizer = s
	return e
}

// WithNotifier enables update notifications (returns e for chaining). Pass a
// non-nil notifier only when Matrix is configured.
func (e *Engine) WithNotifier(n Notifier) *Engine {
	e.notifier = n
	return e
}

// New builds an Engine. workers caps the per-sweep concurrency. It is kept low
// on purpose: a sweep over ~55 containers mostly hits the same registry host
// (ghcr.io / lscr.io), and a high fan-out triggers anonymous 429s. The resolver
// also gates per host, so a small pool keeps the request rate civil.
func New(c Collector, r Resolver, cl Changelogger, s Storer, interval time.Duration) *Engine {
	return &Engine{
		collector: c, resolver: r, changelog: cl, store: s,
		interval: interval,
		workers:  3,
		refresh:  make(chan struct{}, 1),
		now:      time.Now,
	}
}

// Sweep discovers every container and stores its update status. A failure for
// one container is captured in that row's Error and never aborts the sweep;
// only a collector failure (can't list at all) is returned.
func (e *Engine) Sweep(ctx context.Context) error {
	containers, err := e.collector.List(ctx)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, e.workers)
	var wg sync.WaitGroup
	for _, c := range containers {
		wg.Add(1)
		sem <- struct{}{}
		go func(c model.Container) {
			defer wg.Done()
			defer func() { <-sem }()
			st := e.check(ctx, c)
			if uerr := e.store.Upsert(st); uerr != nil {
				// Store failure for one row must not crash the sweep; the next
				// sweep retries. (No logger dependency here on purpose.)
				_ = uerr
			}
		}(c)
	}
	wg.Wait()
	return nil
}

// check runs the per-container pipeline and returns the status to store.
func (e *Engine) check(ctx context.Context, c model.Container) model.UpdateStatus {
	st := model.UpdateStatus{Container: c, CheckedAt: e.now()}

	// The prior row is the memory: it tells us the version we last believed was
	// running and lets us dedupe notifications.
	prior, hasPrior := model.UpdateStatus{}, false
	if p, err := e.store.Get(c.ID); err == nil {
		prior, hasPrior = p, true
	}

	newestTag, newestDigest, newestVerTag, newestVerDigest, err := e.resolver.Resolve(ctx, c.Repo, c.Tag, c.Digest)
	if err != nil {
		// A transient lookup failure (e.g. a 429 burst against the registry) must
		// NOT blank a container we already resolved. When we have a usable prior
		// row, carry its verdict + changelog forward unchanged and stay silent —
		// the next sweep refreshes it. Only when there's no prior data do we fall
		// back to a bare "unknown" with an honest error.
		if hasPrior && prior.Kind != "" && prior.Kind != model.KindUnknown {
			prior.Container = c // refresh metadata (rename/tag); keep the verdict
			prior.CheckedAt = st.CheckedAt
			prior.Error = ""
			return prior
		}
		st.Error = "could not resolve upstream: " + err.Error()
		st.Kind = model.KindUnknown
		st.Risk = model.RiskUnknown
		st.RiskReason = "upstream lookup failed"
		// Keep what we remembered so a transient lookup failure doesn't blank it,
		// falling back to the image's self-declared version (or a pinned tag) so a
		// container still shows its version when the registry is unreachable.
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

	kind, level, reason := risk.Classify(c.Tag, newestTag, c.Digest, newestDigest)
	// A rolling tag (":latest") makes the raw tag comparison blind to the real
	// jump — latest vs latest looks like a flat "digest, low risk". When we
	// resolved two real versions, classify by that delta instead, so an actual
	// minor/major update shows the right risk (orange/red) rather than "digest".
	if isVersion(st.RunningVersion) && isVersion(newestVerTag) {
		if vk, vl, vr := risk.Classify(st.RunningVersion, newestVerTag, "", ""); vk != model.KindUnknown && vk != model.KindNone {
			kind, level, reason = vk, vl, vr
		}
	}
	st.Kind, st.Risk, st.RiskReason = kind, level, reason

	// Resolve a changelog for EVERY container, so each can show one: the update
	// span when there's an update, or the running version's release notes when it
	// is up to date. Summaries and notifications stay update-only.
	if cl, ok := e.changelog.Get(ctx, c, c.Tag, newestTag); ok {
		st.Changelog = cl
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
	return st
}

// decideRunningVersion determines the version we believe is currently running.
//
//   - A pinned version tag IS the running version.
//   - For a rolling tag (":latest") the tag carries no version, so we report the
//     best version we can stand behind, strongest evidence first:
//     1. if the running image's digest equals the newest version's digest, the
//     running image IS that version — proven, even if ":latest" itself lags a
//     newer published tag;
//     2. otherwise, if nothing changed since last sight, keep the remembered
//     version (the memory that lets a later update show "1.7 -> 1.8");
//     3. otherwise, the version the image declares about itself via its OCI
//     version label — so a labelled image shows its version immediately, before
//     ShipLog has ever witnessed an update;
//     4. otherwise we don't know it yet (an unlabelled, out-of-date image).
//
// newestVerTag/newestVerDigest are the newest SEMVER tag and its digest.
func decideRunningVersion(c model.Container, newestVerTag, newestVerDigest string, prior model.UpdateStatus, hasPrior bool) string {
	if isVersion(c.Tag) {
		return c.Tag
	}
	if c.Digest != "" && newestVerDigest != "" && c.Digest == newestVerDigest {
		return newestVerTag // running image proven to be the newest version
	}
	if hasPrior && c.Digest != "" && prior.Container.Digest == c.Digest && prior.RunningVersion != "" {
		return prior.RunningVersion // unchanged since last sight → remembered
	}
	if isVersion(c.ImageVersion) {
		return c.ImageVersion // the image's self-declared version (OCI label)
	}
	return ""
}

// isVersion reports whether a tag looks like a version number (e.g. "1.8", "v2.3.1").
func isVersion(tag string) bool { return versionLike.MatchString(tag) }

// maybeNotify pushes a notification only when a *new* update appears for a
// container we've already seen: the prior row must exist (so we don't flood on
// first run) and must not already represent this same update target (so we
// don't repeat every poll). prior is the row read at the start of the check.
func (e *Engine) maybeNotify(ctx context.Context, st model.UpdateStatus, prior model.UpdateStatus, hasPrior bool) {
	if e.notifier == nil || !hasPrior {
		return // no notifier, or first sight → seed silently
	}
	if prior.HasUpdate() && prior.NewestDigest == st.NewestDigest {
		return // already notified for this exact update
	}
	_ = e.notifier.Notify(ctx, st)
}

// Refresh triggers an out-of-band sweep (non-blocking; coalesces).
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
