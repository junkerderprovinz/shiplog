package notify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestNewUnraid_Gating(t *testing.T) {
	if NewUnraid(false) != nil {
		t.Error("disabled → nil")
	}

	t.Setenv("UNRAID_NOTIFY_SCRIPT", filepath.Join(t.TempDir(), "does-not-exist"))
	if NewUnraid(true) != nil {
		t.Error("enabled but notify tool missing → nil")
	}

	script := filepath.Join(t.TempDir(), "notify")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNRAID_NOTIFY_SCRIPT", script)
	if NewUnraid(true) == nil {
		t.Error("enabled + notify tool present → non-nil")
	}
}

func TestUpdateArgs(t *testing.T) {
	st := model.UpdateStatus{
		Container: model.Container{Name: "immich", Tag: "1.0", Source: "https://github.com/x/immich"},
		NewestTag: "1.2", Risk: model.RiskMedium,
	}
	args := updateArgs(st)

	if got := argValue(args, "-e"); got != "ShipLog" {
		t.Errorf("event = %q, want ShipLog", got)
	}
	if got := argValue(args, "-s"); !strings.Contains(got, "immich") {
		t.Errorf("subject missing name: %q", got)
	}
	if got := argValue(args, "-d"); !strings.Contains(got, "1.0") || !strings.Contains(got, "1.2") || !strings.Contains(got, "MEDIUM") {
		t.Errorf("description = %q", got)
	}
	if got := argValue(args, "-i"); got != "normal" {
		t.Errorf("medium risk → normal importance, got %q", got)
	}
	if got := argValue(args, "-l"); got != "https://github.com/x/immich" {
		t.Errorf("link = %q", got)
	}
}

func TestUpdateArgs_HighRiskIsWarning(t *testing.T) {
	st := model.UpdateStatus{Container: model.Container{Name: "app", Tag: "1"}, NewestTag: "2", Risk: model.RiskHigh}
	if got := argValue(updateArgs(st), "-i"); got != "warning" {
		t.Errorf("high risk → warning importance, got %q", got)
	}
}

func TestUnraid_NilSafe(t *testing.T) {
	var u *Unraid
	if err := u.Notify(context.Background(), model.UpdateStatus{}); err != nil {
		t.Errorf("nil Notify: %v", err)
	}
	if err := u.SendMessage(context.Background(), "x", "y"); err != nil {
		t.Errorf("nil SendMessage: %v", err)
	}
}
