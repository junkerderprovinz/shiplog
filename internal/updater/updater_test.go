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
	if err := (Unraid{}).Update(context.Background(), "plex"); err == nil {
		t.Error("Unraid.Update must error off-box (no template)")
	}
}
