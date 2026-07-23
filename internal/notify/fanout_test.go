package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

type recSink struct {
	notifies int
	messages int
	err      error
}

func (r *recSink) Notify(_ context.Context, _ model.UpdateStatus) error { r.notifies++; return r.err }
func (r *recSink) SendMessage(_ context.Context, _, _ string) error     { r.messages++; return r.err }

func TestNewFanout_NilWhenEmpty(t *testing.T) {
	if NewFanout() != nil {
		t.Error("no sinks → nil")
	}
}

func TestFanout_DeliversToAllSinks(t *testing.T) {
	a, b := &recSink{}, &recSink{}
	f := NewFanout(a, b)

	_ = f.Notify(context.Background(), model.UpdateStatus{})
	_ = f.SendMessage(context.Background(), "x", "y")

	if a.notifies != 1 || b.notifies != 1 {
		t.Errorf("Notify not fanned out: a=%d b=%d", a.notifies, b.notifies)
	}
	if a.messages != 1 || b.messages != 1 {
		t.Errorf("SendMessage not fanned out: a=%d b=%d", a.messages, b.messages)
	}
}

func TestFanout_OneFailingDoesNotStopOthers(t *testing.T) {
	bad := &recSink{err: errors.New("boom")}
	good := &recSink{}
	f := NewFanout(bad, good)

	if err := f.Notify(context.Background(), model.UpdateStatus{}); err == nil {
		t.Error("expected the first sink's error to surface")
	}
	if good.notifies != 1 {
		t.Error("the second sink must still be called after the first fails")
	}
}

func TestFanout_NilSafe(t *testing.T) {
	var f *Fanout
	if err := f.Notify(context.Background(), model.UpdateStatus{}); err != nil {
		t.Errorf("nil Notify: %v", err)
	}
	if err := f.SendMessage(context.Background(), "x", "y"); err != nil {
		t.Errorf("nil SendMessage: %v", err)
	}
}
