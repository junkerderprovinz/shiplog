package notify

import (
	"context"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Sink is one notification channel (Matrix, native Unraid, ...): a per-update
// Notify plus a free-text SendMessage used for the auto-update run summary.
// Both *Matrix and *Unraid satisfy it.
type Sink interface {
	Notify(ctx context.Context, st model.UpdateStatus) error
	SendMessage(ctx context.Context, text, html string) error
}

// Fanout delivers each notification to every configured sink, best-effort: one
// sink failing never stops the others. It returns the first error encountered so
// the caller can log that something went wrong.
type Fanout struct {
	sinks []Sink
}

// NewFanout returns a Fanout over the given sinks, or nil when there are none —
// so the engine treats "no channels configured" as no notifier at all.
func NewFanout(sinks ...Sink) *Fanout {
	if len(sinks) == 0 {
		return nil
	}
	return &Fanout{sinks: sinks}
}

// Notify fans a per-update notification out to every sink. nil → no-op.
func (f *Fanout) Notify(ctx context.Context, st model.UpdateStatus) error {
	if f == nil {
		return nil
	}
	var firstErr error
	for _, s := range f.sinks {
		if err := s.Notify(ctx, st); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SendMessage fans a free-text message out to every sink. nil → no-op.
func (f *Fanout) SendMessage(ctx context.Context, text, html string) error {
	if f == nil {
		return nil
	}
	var firstErr error
	for _, s := range f.sinks {
		if err := s.SendMessage(ctx, text, html); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
