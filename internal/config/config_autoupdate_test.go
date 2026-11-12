package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/config"
)

func TestLoadAutoUpdateDefaults(t *testing.T) {
	for _, k := range []string{
		"AUTOUPDATE_ENABLED", "AUTOUPDATE_LEVEL", "AUTOUPDATE_DIGEST", "AUTOUPDATE_DRYRUN",
		"AUTOUPDATE_SCHED_MODE", "AUTOUPDATE_SCHED_TIME", "AUTOUPDATE_SCHED_EVERY",
		"AUTOUPDATE_EXCLUDE_CONTAINERS",
	} {
		t.Setenv(k, "")
	}
	au := config.Load().AutoUpdate
	if au.Enabled || au.Level != "off" || au.Digest || au.DryRun || au.SchedMode != "off" || au.SchedEvery != 6 || au.ExcludeContainers != "" {
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
	t.Setenv("AUTOUPDATE_EXCLUDE_CONTAINERS", "gitea,plex")
	au := config.Load().AutoUpdate
	if !au.Enabled || au.Level != "minor" || !au.Digest || !au.DryRun || au.SchedMode != "hours" || au.SchedEvery != 6 || au.ExcludeContainers != "gitea,plex" {
		t.Fatalf("parse failed: %+v", au)
	}
}

func TestLoadBackupSettings(t *testing.T) {
	t.Setenv("AUTOUPDATE_BACKUP_MINUTES", "")
	t.Setenv("AUTOUPDATE_WITHOUT_BACKUP", "")
	t.Setenv("FORTRESS_DIR", "")
	cfg := config.Load()
	if cfg.AutoUpdate.BackupTimeout != 30*time.Minute || cfg.AutoUpdate.WithoutBackup != "" || cfg.FortressDir != "" {
		t.Fatalf("defaults = %s %q %q, want 30m and nothing else", cfg.AutoUpdate.BackupTimeout, cfg.AutoUpdate.WithoutBackup, cfg.FortressDir)
	}

	t.Setenv("AUTOUPDATE_BACKUP_MINUTES", "90")
	t.Setenv("AUTOUPDATE_WITHOUT_BACKUP", "redis,dozzle")
	t.Setenv("FORTRESS_DIR", "/boot/config/plugins/shiplog/fortress")
	cfg = config.Load()
	if cfg.AutoUpdate.BackupTimeout != 90*time.Minute || cfg.AutoUpdate.WithoutBackup != "redis,dozzle" || cfg.FortressDir != "/boot/config/plugins/shiplog/fortress" {
		t.Fatalf("parsed = %s %q %q", cfg.AutoUpdate.BackupTimeout, cfg.AutoUpdate.WithoutBackup, cfg.FortressDir)
	}

	t.Setenv("AUTOUPDATE_BACKUP_MINUTES", "0")
	if got := config.Load().AutoUpdate.BackupTimeout; got != 30*time.Minute {
		t.Errorf("a timeout below a minute = %s, want the 30m default", got)
	}
}

func TestRunsFileDefaultsToTheDataDir(t *testing.T) {
	t.Setenv("DATA_DIR", "/config")
	t.Setenv("RUNS_FILE", "")
	if got := config.Load().RunsFile; got != filepath.Join("/config", "runs.json") {
		t.Fatalf("RunsFile = %q", got)
	}
	t.Setenv("RUNS_FILE", "/boot/config/plugins/shiplog/runs.json")
	if got := config.Load().RunsFile; got != "/boot/config/plugins/shiplog/runs.json" {
		t.Fatalf("RunsFile = %q", got)
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
