package notify

import (
	"context"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Sink is one notification channel such as Matrix or native Unraid. SendMessage
// carries the auto-update run summary.
type Sink interface {
	Notify(ctx context.Context, st model.UpdateStatus) error
	SendMessage(ctx context.Context, text, html string) error
}

// Fanout delivers each notification to every sink. A failing sink does not stop
// the others; the first error is returned for logging.
type Fanout struct {
	sinks []Sink
}

// NewFanout returns nil when there are no sinks, so the engine sees no notifier
// at all.
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
