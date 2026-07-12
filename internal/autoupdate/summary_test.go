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
