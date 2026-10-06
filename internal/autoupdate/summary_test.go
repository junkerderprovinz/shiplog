package autoupdate

import (
	"errors"
	"strings"
	"testing"
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
	for _, want := range []string{"Auto-update run", "1 skipped (excluded from auto-update)", "gitea ?→? (minor)"} {
		if !strings.Contains(dry, want) {
			t.Errorf("skipped-only dry run missing %q: %s", want, dry)
		}
	}
	if strings.Contains(dry, "Would auto-update") {
		t.Errorf("a skipped-only dry run must not claim anything would be updated: %s", dry)
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
