package autoupdate

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/backup"
)

func TestRenderSummary(t *testing.T) {
	if txt, _ := RenderSummary(Result{}); txt != "" {
		t.Fatal("an empty run must render nothing")
	}
	res := Result{Outcomes: []Outcome{
		{Name: "plex", From: "1.2.3", To: "1.2.4", Level: "patch", Updated: true},
		{Name: "radarr", From: "4.0", To: "4.1", Level: "minor", Err: errors.New("pull error")},
	}}
	text, html := RenderSummary(res)
	for _, want := range []string{"Auto-updated 1", "plex", "1.2.3→1.2.4", "patch", "1 failed", "radarr", "pull error"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(html, "<b>plex</b>") {
		t.Errorf("html missing bold name: %s", html)
	}
	dt, _ := RenderSummary(Result{DryRun: true, Outcomes: []Outcome{{Name: "plex", Level: "patch", Updated: true}}})
	if !strings.Contains(dt, "Would auto-update") {
		t.Errorf("dry-run wording missing: %s", dt)
	}
}

func TestRenderSummaryTellsWhatTheBackupDid(t *testing.T) {
	snap := backup.Snapshot{ID: "4f2a9c1e", Link: "https://tower.lan/snap/4f2a9c1e"}
	res := Result{Outcomes: []Outcome{
		{Name: "plex", From: "1.2.3", To: "1.2.4", Level: "patch", Updated: true, Snapshot: snap},
		{Name: "sonarr", From: "4.0", To: "4.1", Level: "minor", Updated: true, WithoutBackup: true},
		{Name: "radarr", From: "5.0", To: "5.1", Level: "minor", Err: errors.New("unhealthy after the update"), RolledBack: true, Snapshot: snap},
		{Name: "mariadb", From: "11.4", To: "11.5", Level: "minor", NoBackup: true, Err: fmt.Errorf("held back without a backup: %w", errors.New("the backup failed: repository locked"))},
	}}
	text, html := RenderSummary(res)
	for _, want := range []string{
		"plex 1.2.3→1.2.4 (patch, backed up first)",
		"sonarr 4.0→4.1 (minor, without a backup)",
		"1 failed: radarr (unhealthy after the update, back on its old image and pinned to it in the template, backup 4f2a9c1e)",
		"1 held back without a backup: mariadb (the backup failed: repository locked)",
		`"Update without a backup"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(html, `<a href="https://tower.lan/snap/4f2a9c1e">backup 4f2a9c1e</a>`) {
		t.Errorf("html does not link the snapshot:\n%s", html)
	}
	if strings.Count(text, "mariadb") != 1 {
		t.Errorf("a held update must not count as failed too:\n%s", text)
	}
}

func TestRenderSummary_Blocked(t *testing.T) {
	res := Result{Outcomes: []Outcome{
		{Name: "dozzle", Blocked: true, BlockedWord: "breaking"},
	}}
	text, html := RenderSummary(res)
	for _, want := range []string{"1 blocked", "dozzle", "breaking"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(html, "dozzle") {
		t.Errorf("html missing name: %s", html)
	}
	if strings.Contains(text, "failed") {
		t.Errorf("a blocked outcome must not be reported as failed: %s", text)
	}

	dryText, _ := RenderSummary(Result{DryRun: true, Outcomes: []Outcome{
		{Name: "immich", Blocked: true, BlockedWord: "BREAKING CHANGE"},
	}})
	for _, want := range []string{"1 blocked", "immich", "BREAKING CHANGE"} {
		if !strings.Contains(dryText, want) {
			t.Errorf("dry-run blocked text missing %q: %s", want, dryText)
		}
	}
}

func TestRenderSummary_Skipped(t *testing.T) {
	res := Result{Outcomes: []Outcome{
		{Name: "plex", From: "1.2.3", To: "1.2.4", Level: "patch", Updated: true},
		{Name: "gitea", From: "0.42.1", To: "0.43.0", Level: "minor", Skipped: true},
	}}
	text, html := RenderSummary(res)
	for _, want := range []string{"Auto-updated 1", "plex", "1 skipped (excluded from auto-update)", "gitea 0.42.1→0.43.0 (minor)"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(html, "1 skipped (excluded from auto-update)") || !strings.Contains(html, "gitea") {
		t.Errorf("html missing the skipped clause: %s", html)
	}
	for _, bad := range []string{"failed", "blocked"} {
		if strings.Contains(text, bad) {
			t.Errorf("a skipped outcome must not read as %q: %s", bad, text)
		}
	}

	dry, _ := RenderSummary(Result{DryRun: true, Outcomes: []Outcome{{Name: "gitea", Level: "minor", Skipped: true}}})
	for _, want := range []string{"Auto-update dry run", "1 skipped (excluded from auto-update)", "gitea ?→? (minor)"} {
		if !strings.Contains(dry, want) {
			t.Errorf("skipped-only dry run missing %q: %s", want, dry)
		}
	}
	if strings.Contains(dry, "Would auto-update") {
		t.Errorf("a skipped-only dry run must not claim anything would be updated: %s", dry)
	}
	live, _ := RenderSummary(Result{Outcomes: []Outcome{{Name: "gitea", Level: "minor", Skipped: true}}})
	if !strings.Contains(live, "Auto-update run") || strings.Contains(live, "dry") {
		t.Errorf("a skipped-only real run must read as a run, not a dry run: %s", live)
	}
}

func TestRenderSummary_UpToDate(t *testing.T) {
	res := Result{Outcomes: []Outcome{
		{Name: "plex", From: "1.2.3", To: "1.2.4", Level: "patch", Updated: true},
		{Name: "sonarr", From: "4.0.20", To: "latest", Level: "major", UpToDate: true},
		{Name: "nzbget", Level: "major", UpToDate: true},
	}}
	text, html := RenderSummary(res)
	for _, want := range []string{"Auto-updated 1: plex", "2 already up to date: sonarr, nzbget"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(html, "2 already up to date: sonarr, nzbget") {
		t.Errorf("html missing the up-to-date clause: %s", html)
	}
	for _, bad := range []string{"failed", "blocked", "skipped", "sonarr 4.0.20"} {
		if strings.Contains(text, bad) {
			t.Errorf("an up-to-date outcome must not read as %q: %s", bad, text)
		}
	}

	dry, _ := RenderSummary(Result{DryRun: true, Outcomes: []Outcome{{Name: "sonarr", Level: "major", UpToDate: true}}})
	if strings.Contains(dry, "Would auto-update") || !strings.Contains(dry, "1 already up to date: sonarr") {
		t.Errorf("dry run with nothing to do reads wrong: %s", dry)
	}
}

func TestRenderSummary_KeptContainers(t *testing.T) {
	res := Result{Outcomes: []Outcome{
		{Name: "Immich", From: "1.1", To: "1.2", Level: "minor", Updated: true, Recreated: true, Started: true},
		{Name: "Repo-Watch", Level: "digest", Err: ErrNoChange, Started: true},
		{Name: "OpenCloud", From: "2.0", To: "2.1", Level: "minor", Updated: true, Recreated: true},
	}}
	text, html := RenderSummary(res)
	for _, want := range []string{
		"Immich 1.1→1.2 (minor, recreated from its template and started)",
		"Repo-Watch (still on the old image after the update, started again)",
		"OpenCloud 2.0→2.1 (minor, recreated from its template)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(html, "(minor, recreated from its template and started)") {
		t.Errorf("html missing what was done to keep Immich: %s", html)
	}
}

func TestResultFailedOnlyWhenAnUpdateFailed(t *testing.T) {
	ok := Result{Outcomes: []Outcome{{Name: "a", Updated: true}, {Name: "b", Skipped: true}}}
	if ok.Failed() {
		t.Error("a clean run counts as failed")
	}
	bad := Result{Outcomes: []Outcome{{Name: "a", Updated: true}, {Name: "b", Err: errors.New("pull error")}}}
	if !bad.Failed() {
		t.Error("a run with a failed update does not count as failed")
	}
}
