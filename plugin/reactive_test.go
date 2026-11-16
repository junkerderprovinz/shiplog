package plugin

import (
	"regexp"
	"strings"
	"testing"
)

// TestReactiveButtonsKeepTheirWidth holds the give windows' buttons and the
// changelog bubble's to the width they have with glyph and words in
// TowerTorch's reactive mode, so the words open inside them and a row that
// wraps never moves a button out from under the pointer.
func TestReactiveButtonsKeepTheirWidth(t *testing.T) {
	settings := read(t, "styles/settings.css")
	docker := read(t, "styles/docker.css")
	for _, want := range []string{
		`#sl-wins[data-sl-labels="reactive"] .sl-win-btn { min-width: var(--sl-btn-w, auto); }`,
		`#sl-wins[data-sl-labels="reactive"] .sl-win-btn[data-sl-measure] .sl-l { max-width: 16em; opacity: 1; margin-inline-start: 6px; }`,
		`#sl-wins .sl-win-btn[data-sl-still], #sl-wins .sl-win-btn[data-sl-still] * { transition: none !important; }`,
	} {
		if !strings.Contains(settings, want) {
			t.Errorf("settings.css lacks %s", want)
		}
	}
	for _, want := range []string{
		`html.tt-popups-on[data-tt-labels-buttons="reactive"] .sl-bubble :is(.sl-gh,.sl-upd){min-width:var(--sl-btn-w,auto)}`,
		`html.tt-popups-on[data-tt-labels-buttons="reactive"] .sl-bubble :is(.sl-gh,.sl-upd)[data-sl-fit]{justify-content:center}`,
		`html.tt-popups-on[data-tt-labels-buttons="reactive"] .sl-bubble :is(.sl-gh,.sl-upd)[data-sl-measure]{gap:6px}`,
		`html.tt-popups-on[data-tt-labels-buttons="reactive"] .sl-bubble :is(.sl-gh,.sl-upd)[data-sl-measure] .sl-l{max-width:16em;opacity:1}`,
		`.sl-bubble :is(.sl-gh,.sl-upd)[data-sl-still],.sl-bubble :is(.sl-gh,.sl-upd)[data-sl-still] *{transition:none!important}`,
	} {
		if !strings.Contains(docker, want) {
			t.Errorf("docker.css lacks %s", want)
		}
	}
	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	grows := regexp.MustCompile(`:hover[^{]*\{[^}]*(?:^|[^-\w])width:`)
	for name, css := range map[string]string{"settings.css": settings, "docker.css": docker} {
		if m := grows.FindString(comments.ReplaceAllString(css, "")); m != "" {
			t.Errorf("%s changes a width under the pointer: %s", name, m)
		}
	}

	for _, file := range []string{"scripts/settings.js", "scripts/docker.js"} {
		js := read(t, file)
		for _, want := range []string{
			`function holdWidth(b, reactive)`,
			`setAttribute("data-sl-measure", "")`,
			`style.setProperty("--sl-btn-w", Math.ceil(w) + "px")`,
			`removeAttribute("data-sl-still")`,
			`new ResizeObserver(`,
			`characterData: true`,
		} {
			if !strings.Contains(js, want) {
				t.Errorf("%s lacks %s", file, want)
			}
		}
	}
}
