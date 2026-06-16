// Package engine runs the background poll loop that turns discovered containers
// into stored update statuses: collector → resolver → risk → changelog → store.
package engine

import (
	"context"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/risk"
)

// The collaborators are interfaces so the engine stays unit-testable; the real
// dockercli.Client / resolver.Resolver / changelog.Chain / store.Store satisfy them.
type (
	// Collector lists the containers on the host (read-only).
	Collector interface {
		List(ctx context.Context) ([]model.Container, error)
	}
	// Resolver returns the newest tag and the same-tag digest for an image ref.
	Resolver interface {
		Resolve(ctx context.Context, repo, tag, curDigest string) (newestTag, sameTagDigest string, err error)
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

// New builds an Engine. workers caps the per-sweep concurrency.
func New(c Collector, r Resolver, cl Changelogger, s Storer, interval time.Duration) *Engine {
	return &Engine{
		collector: c, resolver: r, changelog: cl, store: s,
		interval: interval,
		workers:  6,
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

	newestTag, newestDigest, err := e.resolver.Resolve(ctx, c.Repo, c.Tag, c.Digest)
	if err != nil {
		st.Error = "could not resolve upstream: " + err.Error()
		st.Kind = model.KindUnknown
		st.Risk = model.RiskUnknown
		st.RiskReason = "upstream lookup failed"
		return st
	}
	st.NewestTag = newestTag
	st.NewestDigest = newestDigest

	kind, level, reason := risk.Classify(c.Tag, newestTag, c.Digest, newestDigest)
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
				}
			}
			e.maybeNotify(ctx, st)
		}
	}
	return st
}

// maybeNotify pushes a notification only when a *new* update appears for a
// container we've already seen: the previously stored row must exist (so we
// don't flood on first run) and must not already represent this same update
// target (so we don't repeat every poll).
func (e *Engine) maybeNotify(ctx context.Context, st model.UpdateStatus) {
	if e.notifier == nil {
		return
	}
	prior, err := e.store.Get(st.Container.ID)
	if err != nil {
		return // unknown container (first sight) → seed silently
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
