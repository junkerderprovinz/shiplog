package config_test

import (
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/config"
)

func TestLoadAutoUpdateDefaults(t *testing.T) {
	for _, k := range []string{
		"AUTOUPDATE_ENABLED", "AUTOUPDATE_LEVEL", "AUTOUPDATE_DIGEST", "AUTOUPDATE_DRYRUN",
		"AUTOUPDATE_SCHED_MODE", "AUTOUPDATE_SCHED_TIME", "AUTOUPDATE_SCHED_EVERY",
	} {
		t.Setenv(k, "")
	}
	au := config.Load().AutoUpdate
	if au.Enabled || au.Level != "off" || au.Digest || au.DryRun || au.SchedMode != "off" || au.SchedEvery != 6 {
		t.Fatalf("defaults must be safe/off, got %+v", au)
	}
}

func TestLoadAutoUpdateParsed(t *testing.T) {
	t.Setenv("AUTOUPDATE_ENABLED", "yes")
	t.Setenv("AUTOUPDATE_LEVEL", "minor")
	t.Setenv("AUTOUPDATE_DIGEST", "yes")
	t.Setenv("AUTOUPDATE_DRYRUN", "yes")
	t.Setenv("AUTOUPDATE_SCHED_MODE", "hours")
	t.Setenv("AUTOUPDATE_SCHED_EVERY", "6")
	au := config.Load().AutoUpdate
	if !au.Enabled || au.Level != "minor" || !au.Digest || !au.DryRun || au.SchedMode != "hours" || au.SchedEvery != 6 {
		t.Fatalf("parse failed: %+v", au)
	}
}

func TestLoadAutoUpdateInvalidLevelAndEveryFallBack(t *testing.T) {
	t.Setenv("AUTOUPDATE_LEVEL", "bogus")
	t.Setenv("AUTOUPDATE_SCHED_EVERY", "0")
	au := config.Load().AutoUpdate
	if au.Level != "off" {
		t.Errorf("invalid level must fall back to off, got %q", au.Level)
	}
	if au.SchedEvery != 6 {
		t.Errorf("every<1 must fall back to 6, got %d", au.SchedEvery)
	}
}
