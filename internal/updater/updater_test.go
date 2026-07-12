package updater

import (
	"context"
	"errors"
	"testing"
)

func TestNoop(t *testing.T) {
	var u Updater = Noop{}
	if u.Supported() {
		t.Error("Noop is never supported")
	}
	if !errors.Is(u.Update(context.Background(), "x"), ErrUnsupported) {
		t.Error("Noop.Update must return ErrUnsupported")
	}
}

func TestUnraidOffBox(t *testing.T) {
	if (Unraid{}).Supported() {
		t.Skip("host has the Unraid template dir; off-box assertion skipped")
	}
	// Absent the template dir, Unraid reports unsupported and Update is the
	// spike stub until plan Task 1 wires the real hook.
	if !errors.Is((Unraid{}).Update(context.Background(), "x"), ErrSpikePending) {
		t.Error("Unraid.Update must return ErrSpikePending until the on-box spike")
	}
}
