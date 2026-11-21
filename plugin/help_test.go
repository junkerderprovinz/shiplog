package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTheHelpCardOffersGitHubAndDiscord keeps the Help card to the two routes
// its sentence names, in every language.
func TestTheHelpCardOffersGitHubAndDiscord(t *testing.T) {
	page := read(t, "shiplog.page")
	card := regexp.MustCompile(`(?s)<section class="sl-card" aria-labelledby="sl-h-help">(.*?)</section>`).FindStringSubmatch(page)
	if card == nil {
		t.Fatal("shiplog.page has no Help card")
	}
	var routes []string
	for _, m := range regexp.MustCompile(`\$slRbtn\('(\w+)', '[^']*', '([^']*)'\)`).FindAllStringSubmatch(card[1], -1) {
		routes = append(routes, m[1]+" "+m[2])
	}
	want := []string{"github https://github.com/junkerderprovinz/shiplog/issues", "discord https://discord.gg/SyU7QUsbFR"}
	if strings.Join(routes, ", ") != strings.Join(want, ", ") {
		t.Errorf("routes = %v, want %v", routes, want)
	}

	if !strings.Contains(read(t, "include/marks.php"), "'discord' => '<svg") {
		t.Error("marks.php has no Discord mark")
	}
	css := read(t, "styles/settings.css")
	for _, rule := range []string{".sl-discord-mark { color: var(--sl-brand-discord); }", ".sl-tile-discord { --sl-give: #5865f2; --sl-give-ink: #fff; }"} {
		if !strings.Contains(css, rule) {
			t.Errorf("settings.css lacks %s", rule)
		}
	}

	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	other := regexp.MustCompile(`(?i)forum|e-?mail`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]any
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		body, _ := texts["s_help_body"].(string)
		if !strings.Contains(body, "GitHub") || !strings.Contains(body, "Discord") || other.MatchString(body) {
			t.Errorf("%s: s_help_body = %q", filepath.Base(f), body)
		}
	}
}

// TestTheVersionCardNamesTheGlimStoneItPorts keeps the pins and the card on one
// GlimStone version, linked to the newest release, which stays when a patch
// tag folds into the next minor.
func TestTheVersionCardNamesTheGlimStoneItPorts(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "glimstone.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Targets []struct{ Version string } `json:"targets"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, target := range cfg.Targets {
		if target.Version != "4.8.1" {
			t.Errorf("glimstone.json pins %s, want 4.8.1", target.Version)
		}
	}
	page := read(t, "shiplog.page")
	if !strings.Contains(page, "$slGlimstoneVer = '4.8.1';") {
		t.Error("the Version card does not name GlimStone 4.8.1")
	}
	row := regexp.MustCompile(`(?s)<span>GlimStone</span></div>\s*<div class="sl-ctl">(.*?)</div>`).FindStringSubmatch(page)
	if row == nil || !strings.Contains(row[1], `href="https://github.com/junkerderprovinz/glimstone/releases/latest"`) || !strings.Contains(row[1], `$slE($slGlimstoneVer)`) {
		t.Errorf("the GlimStone row does not open the newest release: %v", row)
	}
}

// TestAReadmeButtonNameKeepsItsDescenders lets the name's clip reach a little
// past its line, as on GlimStone's README button, so a p or y stays whole.
func TestAReadmeButtonNameKeepsItsDescenders(t *testing.T) {
	css := read(t, "styles/settings.css")
	if !regexp.MustCompile(`#sl-cfg \.sl-rbtn-name \{[^}]*overflow: hidden;[^}]*padding-block: \.15em; margin-block: -\.15em;`).MatchString(css) {
		t.Error("the clip of a README button's name cuts its descenders")
	}
}

// TestAVersionNeverLooksLikeAButton shows every version on the Info tab as
// plain text, linked quietly to its release notes, as in every app of the house.
func TestAVersionNeverLooksLikeAButton(t *testing.T) {
	page := read(t, "shiplog.page")
	if n := strings.Count(page, `class="sl-ver`); n < 3 {
		t.Fatalf("found %d version displays, want the two rows and the plain fallback", n)
	}
	if regexp.MustCompile(`class="[^"]*button[^"]*sl-ver|class="[^"]*sl-ver[^"]*button`).MatchString(page) {
		t.Error("a version wears Unraid's button class")
	}
	css := read(t, "styles/settings.css")
	rule := regexp.MustCompile(`(?s)\.sl-cfg a\.sl-ver \{([^}]*)\}`).FindStringSubmatch(css)
	if rule == nil {
		t.Fatal("settings.css has no rule for a linked version")
	}
	for _, want := range []string{"background: none", "border: 0", "padding: 0", "color: inherit", "text-decoration: none"} {
		if !strings.Contains(rule[1], want) {
			t.Errorf("a linked version lacks %q, so it can look like a button", want)
		}
	}
	if !strings.Contains(css, "#sl-cfg a.sl-ver { color: inherit !important; }") {
		t.Error("the theme plugin's accent colours a linked version like a control")
	}
}

// TestAButtonLabelKeepsTheTailsOfArabic gives a button's words the room below
// the line that GlimStone's label has, so an Arabic label keeps its tails.
func TestAButtonLabelKeepsTheTailsOfArabic(t *testing.T) {
	css := read(t, "styles/settings.css")
	if !regexp.MustCompile(`\.sl-l \{[^}]*overflow: hidden;[^}]*padding-block: \.15em; margin-block: -\.15em;`).MatchString(css) {
		t.Error("the clip of a button's label cuts what hangs below the line")
	}
}

// TestAReadmeButtonMarkKeepsItsSoftEdge lets a mark that reaches its box draw
// past the SVG's own clip, as GlimStone's README button does.
func TestAReadmeButtonMarkKeepsItsSoftEdge(t *testing.T) {
	css := read(t, "styles/settings.css")
	if !regexp.MustCompile(`#sl-cfg \.sl-rbtn-mark svg \{ overflow: visible; \}`).MatchString(css) {
		t.Error("the SVG of a README button's mark shaves the edge of a shape drawn to its box")
	}
}

// TestTheFocusRingIsOpaqueInTheAccentsInk follows GlimStone's ring: under
// TowerTorch it is the accent's ink, so it reaches 3:1 on every ground, and it
// takes TowerTorch's width, 3px when the system asks for more contrast.
func TestTheFocusRingIsOpaqueInTheAccentsInk(t *testing.T) {
	css := read(t, "styles/settings.css")
	if strings.Contains(css, "--sl-w-focus: var(--tt-accent, #FCC419);") || !strings.Contains(css, "--sl-w-focus: var(--tt-accent-ink, #FCC419);") {
		t.Error("the windows' ring under TowerTorch is not the accent's ink")
	}
	if strings.Contains(css, "45%, transparent)") {
		t.Error("settings.css still draws a translucent halo")
	}
	if !strings.Contains(css, "#sl-cfg .sl-rbtn:focus-visible { outline: none !important; filter: none !important; box-shadow: inset 0 0 0 var(--tt-focus-ring-width, 2px) var(--sl-focus) !important; }") {
		t.Error("a README button does not draw the ring inside its edge")
	}
	for _, m := range regexp.MustCompile(`outline:\s*(\S+) solid var\(--sl-(?:w-)?focus\)`).FindAllStringSubmatch(css, -1) {
		if m[1] != "var(--tt-focus-ring-width," {
			t.Errorf("a ring is %s wide, not TowerTorch's width", m[1])
		}
	}
	if docker := read(t, "styles/docker.css"); !strings.Contains(docker, ".sl-au-cb:focus-visible + .sl-au-track{box-shadow:0 0 0 var(--tt-focus-ring-width,2px) var(--tt-accent-ink,#ff8c2f)}") {
		t.Error("the bubble's switch does not ring in the accent's ink")
	}
}

// TestTheAmountsPlaceholderReadsOnEveryFill takes GlimStone's placeholder
// colour for the dark give windows, which reaches 4.5:1 on every field fill.
func TestTheAmountsPlaceholderReadsOnEveryFill(t *testing.T) {
	if !strings.Contains(read(t, "styles/settings.css"), "html.tt-popups-on .sl-win input.sl-amount::placeholder { color: #c6c6c6; opacity: 1; }") {
		t.Error("the amount's placeholder is not GlimStone's #c6c6c6")
	}
}

// TestEveryHoverLightsInTowerTorchsHover follows GlimStone's --hover: a button,
// a tab, a choice in a strip and a network all light in TowerTorch's one hover
// colour, and only an older TowerTorch without it falls back to the tinted mix.
func TestEveryHoverLightsInTowerTorchsHover(t *testing.T) {
	css := read(t, "styles/settings.css")
	for _, want := range []string{
		"--sl-w-hover: var(--tt-hover-dark);",
		"--sl-w-hover: var(--tt-hover);",
		"html.tt-popups-on .sl-win button.sl-win-btn:is(:hover, :focus-visible) { background: var(--sl-w-hover, color-mix(",
		"html.tt-popups-on .sl-win :is(.sl-seg-opt:not([aria-checked=\"true\"]), .sl-net:not([aria-selected=\"true\"])):hover { background: var(--sl-w-hover, var(--sl-w-chip)); }",
		"html.tt-popups-on .sl-cfg .sl-tab:not([aria-selected=\"true\"]):hover { background: var(--tt-hover, var(--sl-tile)); box-shadow: none; }",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("settings.css is missing %q", want)
		}
	}
	docker := read(t, "styles/docker.css")
	for _, sel := range []string{".sl-gh", ".sl-x"} {
		if !strings.Contains(docker, "html.tt-popups-on .sl-bubble "+sel+":is(:hover,:focus-visible){background:var(--tt-hover,color-mix(") {
			t.Errorf("the bubble's %s does not light in TowerTorch's hover", sel)
		}
	}
}
