// Package plugin holds checks on the Unraid plugin's page sources.
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

var pluginDir = filepath.Join("src", "shiplog", "usr", "local", "emhttp", "plugins", "shiplog")

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(pluginDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestWindowFootButtonsAreFilled keeps Close and every other answer in a
// window's foot filled (GlimStone rules 3 and 15), since without a ground of
// its own a button there reads as a word.
func TestWindowFootButtonsAreFilled(t *testing.T) {
	page := read(t, "shiplog.page")
	feet := regexp.MustCompile(`<div class="sl-win-act">(.*?)</div>`).FindAllStringSubmatch(page, -1)
	if len(feet) == 0 {
		t.Fatal("no window foot found in shiplog.page")
	}
	button := regexp.MustCompile(`<(button|a)\b[^>]*>`)
	for _, foot := range feet {
		for _, tag := range button.FindAllString(foot[1], -1) {
			if !strings.Contains(tag, "sl-win-btn") {
				t.Errorf("window foot button without sl-win-btn: %s", tag)
			}
		}
	}

	css := regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(read(t, filepath.Join("styles", "settings.css")), "")
	rule := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	ground := regexp.MustCompile(`(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)`)
	clear := regexp.MustCompile(`^(transparent|none)\b`)
	filled := false
	for _, m := range rule.FindAllStringSubmatch(css, -1) {
		if !strings.Contains(m[1], "sl-win-btn") {
			continue
		}
		for _, d := range ground.FindAllStringSubmatch(m[2], -1) {
			v := strings.TrimSpace(d[1])
			if clear.MatchString(v) {
				t.Errorf("%s clears the window button's fill", strings.TrimSpace(m[1]))
			} else if strings.Contains(m[1], "tt-popups-on") && !strings.Contains(m[1], ":hover") {
				filled = true
			}
		}
	}
	if !filled {
		t.Error("settings.css gives sl-win-btn no fill of its own under TowerTorch")
	}
}

// TestEveryLanguageExplainsTheBackupBeforeAnUpdate keeps the backup and
// rollback texts of the Docker tab in every language, and its English fallback
// equal to en.json.
func TestEveryLanguageExplainsTheBackupBeforeAnUpdate(t *testing.T) {
	keys := []string{"wobLabel", "wobHintOn", "wobHintOff", "heldTitle", "heldNote", "rolledTitle", "rolledNote", "rolledUnpin", "guardHint"}
	js := read(t, filepath.Join("scripts", "docker.js"))
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, k := range keys {
			v := texts["d_"+k]
			switch {
			case v == "":
				t.Errorf("%s lacks d_%s", filepath.Base(f), k)
			case strings.HasSuffix(k, "Note") && !strings.Contains(v, "%s"):
				t.Errorf("%s: d_%s drops the reason (%%s): %q", filepath.Base(f), k, v)
			case filepath.Base(f) == "en.json":
				want, _ := json.Marshal(v)
				if !strings.Contains(js, k+": "+string(want)) {
					t.Errorf("docker.js falls back to another English d_%s than en.json's %s", k, want)
				}
			}
		}
	}
}

// TestEveryLanguageCountsKnownVulnerabilities keeps the texts of the
// vulnerability check in every language with their counts, and the Docker
// tab's English fallback equal to en.json.
func TestEveryLanguageCountsKnownVulnerabilities(t *testing.T) {
	counts := map[string][]string{
		"vulnTitle": nil, "vulnBadgeHint": nil, "vulnAnd": nil,
		"vulnCrit_one": {"%n"}, "vulnCrit_other": {"%n"}, "vulnHigh_one": {"%n"}, "vulnHigh_other": {"%n"},
		"vulnRunning": {"%v"}, "vulnInstalled": {"%v"}, "vulnNone": nil, "vulnNoneInstalled": nil,
		"vulnCloses": {"%v"}, "vulnClosesNone": nil, "vulnNewestUnknown": nil, "vulnRaised": {"%v"},
		"vulnSystem": nil, "vulnNoPackages": nil, "vulnFailed": nil, "vulnFixed": nil, "vulnMore": {"%n"},
	}
	js := read(t, filepath.Join("scripts", "docker.js"))
	page := read(t, "shiplog.page")
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, k := range []string{"s_vulns", "s_vulns_sub", "s_vuln_check", "s_vuln_check_desc"} {
		if !strings.Contains(page, "$L['"+k+"']") {
			t.Errorf("shiplog.page does not show %s", k)
		}
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, k := range []string{"s_vulns", "s_vulns_sub", "s_vuln_check", "s_vuln_check_desc"} {
			if texts[k] == "" {
				t.Errorf("%s lacks %s", filepath.Base(f), k)
			}
		}
		if !strings.Contains(texts["s_vuln_check_desc"], "api.osv.dev") {
			t.Errorf("%s: s_vuln_check_desc does not say where the package names go", filepath.Base(f))
		}
		for k, want := range counts {
			v := texts["d_"+k]
			if v == "" {
				t.Errorf("%s lacks d_%s", filepath.Base(f), k)
				continue
			}
			for _, ph := range want {
				if !strings.Contains(v, ph) {
					t.Errorf("%s: d_%s drops %s: %q", filepath.Base(f), k, ph, v)
				}
			}
			if filepath.Base(f) == "en.json" {
				quoted, _ := json.Marshal(v)
				if !strings.Contains(js, k+": "+string(quoted)) {
					t.Errorf("docker.js falls back to another English d_%s than en.json's %s", k, quoted)
				}
			}
		}
	}
}

// TestEveryLanguageGivesTheRiskReasons keeps a text for every reason the
// engine sends as a code, with its values, and the English one equal to the
// engine's own.
func TestEveryLanguageGivesTheRiskReasons(t *testing.T) {
	js := read(t, filepath.Join("scripts", "docker.js"))
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for code, en := range model.ReasonCodes {
			v := texts["d_reason_"+code]
			if v == "" {
				t.Errorf("%s lacks d_reason_%s", filepath.Base(f), code)
				continue
			}
			for _, ph := range []string{"%1", "%2"} {
				if strings.Contains(en, ph) != strings.Contains(v, ph) {
					t.Errorf("%s: d_reason_%s and English differ in %s: %q", filepath.Base(f), code, ph, v)
				}
			}
			if filepath.Base(f) == "en.json" {
				if v != en {
					t.Errorf("en.json d_reason_%s = %q, the engine says %q", code, v, en)
				}
				quoted, _ := json.Marshal(v)
				if !strings.Contains(js, "reason_"+code+": "+string(quoted)) {
					t.Errorf("docker.js falls back to another English reason_%s than en.json's %s", code, quoted)
				}
			}
		}
	}
	if !strings.Contains(js, "riskReason(st)") {
		t.Error("the window does not say the reason from its code")
	}
}

// TestStylesCarryNoControlCharacters keeps escapes such as \25B8 in the CSS
// as text, since a control character in their place shows as stray letters.
func TestStylesCarryNoControlCharacters(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(pluginDir, "styles", "*.css"))
	if err != nil || len(files) == 0 {
		t.Fatalf("css files = %v, %v", files, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range raw {
			if b < 0x20 && b != '\n' && b != '\t' && b != '\r' {
				t.Errorf("%s: control character 0x%02x at byte %d", filepath.Base(f), b, i)
			}
		}
	}
}

// TestTheWindowHeaderShowsTheWholeReason keeps the risk reason in full on a
// line of its own under the title, without a marquee and without cutting it.
func TestTheWindowHeaderShowsTheWholeReason(t *testing.T) {
	js := read(t, filepath.Join("scripts", "docker.js"))
	css := read(t, filepath.Join("styles", "docker.css"))
	for _, gone := range []string{"armJumpMarquee", "sl-jump-marquee", "sl-jump-scroll"} {
		if strings.Contains(js, gone) || strings.Contains(css, gone) {
			t.Errorf("the header still scrolls through %s", gone)
		}
	}
	rule := regexp.MustCompile(`\.sl-bubble \.sl-jump\{([^}]*)\}`).FindStringSubmatch(css)
	if rule == nil {
		t.Fatal("docker.css has no rule for the reason")
	}
	for _, want := range []string{"flex:1 0 100%", "white-space:normal", "order:9"} {
		if !strings.Contains(rule[1], want) {
			t.Errorf("the reason's rule lacks %s: %s", want, rule[1])
		}
	}
	if strings.Contains(rule[1], "ellipsis") || strings.Contains(rule[1], "nowrap") {
		t.Errorf("the reason is still cut short: %s", rule[1])
	}
	if !regexp.MustCompile(`\.sl-bubble \.sl-bh\{[^}]*flex-wrap:wrap`).MatchString(css) {
		t.Error("the header row does not wrap the reason onto its own line")
	}
}

// TestTheVulnerabilityListStaysFolded keeps the window's list of
// vulnerabilities behind its counts line, so the release notes stay in view,
// and says how many the list leaves out.
func TestTheVulnerabilityListStaysFolded(t *testing.T) {
	js := read(t, filepath.Join("scripts", "docker.js"))
	for _, want := range []string{`<details class="sl-sec sl-vulns"><summary>`, `T("vulnMore")`, `vulnCount(st) - list.length`} {
		if !strings.Contains(js, want) {
			t.Errorf("docker.js lacks %s", want)
		}
	}
	if strings.Contains(js, "<details open") {
		t.Error("the vulnerability list opens by itself")
	}
}

// TestTheVulnerabilityCheckIsOnUntilSwitchedOff keeps the settings page and
// the start script on the engine's default.
func TestTheVulnerabilityCheckIsOnUntilSwitchedOff(t *testing.T) {
	if page := read(t, "shiplog.page"); !strings.Contains(page, `$vulnCheck = (!isset($cfg['VULN_CHECK']) || $cfg['VULN_CHECK'] !== 'false');`) {
		t.Error("shiplog.page does not show the check on without a saved setting")
	}
	if rc := read(t, filepath.Join("scripts", "rc.shiplog")); !strings.Contains(rc, "\nVULN_CHECK=\"true\"\n") {
		t.Error("rc.shiplog does not start the engine with the check on")
	}
	var en map[string]string
	if err := json.Unmarshal([]byte(read(t, filepath.Join("lang", "en.json"))), &en); err != nil {
		t.Fatal(err)
	}
	if d := en["s_vuln_check_desc"]; !strings.HasPrefix(d, "On by default.") || !strings.Contains(d, "Turning it off saves") {
		t.Errorf("s_vuln_check_desc = %q, want it on by default and what switching it off saves", d)
	}
}

// TestBubbleNamesOnlyAVouchedVersion keeps the window's "running → next" header
// to tags the engine could prove. A rolling tag's newest changelog entry says
// nothing about the image it serves.
func TestBubbleNamesOnlyAVouchedVersion(t *testing.T) {
	js := read(t, filepath.Join("scripts", "docker.js"))
	m := regexp.MustCompile(`(?s)const next = (.*?);`).FindStringSubmatch(js[strings.Index(js, "function bubbleHTML"):])
	if m == nil {
		t.Fatal("no next version in bubbleHTML")
	}
	if strings.Contains(m[1], "newestRel") || strings.Contains(m[1], "entries") {
		t.Errorf("the next version reads a changelog entry: %s", m[1])
	}
}
