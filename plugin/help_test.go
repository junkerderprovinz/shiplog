package plugin

import (
	"encoding/json"
	"os"
	"os/exec"
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

// TestTheComponentsCardNamesTheGlimStoneItPorts keeps the pins and the card
// on one GlimStone version, linked to the newest release, which stays when a
// patch tag folds into the next minor.
func TestTheComponentsCardNamesTheGlimStoneItPorts(t *testing.T) {
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
		if target.Version != "4.9.0" {
			t.Errorf("glimstone.json pins %s, want 4.9.0", target.Version)
		}
	}
	page := read(t, "shiplog.page")
	if !strings.Contains(page, "$slGlimstoneVer = '4.9.0';") {
		t.Error("the Components card does not name GlimStone 4.9.0")
	}
	card := regexp.MustCompile(`(?s)aria-labelledby="sl-h-components">(.*?)</section>`).FindStringSubmatch(page)
	if card == nil || !strings.Contains(card[1], `<span>GlimStone</span><?=$slInfo($L['s_cc_design'])?>`) ||
		!strings.Contains(card[1], `$slNum($slGlimstoneVer, 'https://github.com/junkerderprovinz/glimstone')`) {
		t.Errorf("the Components card has no GlimStone row with its (i) and number: %v", card)
	}
	if !strings.Contains(page, `$repo.'/releases/latest'`) {
		t.Error("a version does not open its project's newest release")
	}
}

// TestTheInfoTabHoldsVersionComponentsHelpAndAbout keeps the four cards of
// every app in their order, and no other.
func TestTheInfoTabHoldsVersionComponentsHelpAndAbout(t *testing.T) {
	page := read(t, "shiplog.page")
	pane := page[strings.Index(page, `id="sl-pane-info"`):]
	pane = pane[:strings.Index(pane, "<script>")]
	var cards []string
	for _, m := range regexp.MustCompile(`<section class="sl-card" aria-labelledby="sl-h-(\w+)">`).FindAllStringSubmatch(pane, -1) {
		cards = append(cards, m[1])
	}
	if got := strings.Join(cards, " "); got != "version components help about" {
		t.Errorf("the Info tab holds %q", got)
	}
	en := map[string]string{}
	if err := json.Unmarshal([]byte(read(t, "lang/en.json")), &en); err != nil {
		t.Fatal(err)
	}
	if en["s_about"] != "About" {
		t.Errorf("the About card is called %q, want About alone", en["s_about"])
	}
}

// TestTheVersionCardSpeaksEveryLanguage gives each card's words, from
// GlimStone, to every language with the blanks the page fills.
func TestTheVersionCardSpeaksEveryLanguage(t *testing.T) {
	blanks := map[string]string{"s_vc_hint": "{app}", "s_vc_checked": "{when}", "s_vc_available": "{version}", "s_vc_unreleased": "{version}", "s_cc_hint": "{app}", "s_notes_title": "{version}"}
	keys := []string{"s_vc_title", "s_vc_hint", "s_vc_whats_new", "s_vc_check", "s_vc_checking", "s_vc_unchecked", "s_vc_current", "s_vc_checked",
		"s_vc_available", "s_vc_failed", "s_vc_plugins", "s_vc_unreleased", "s_cc_title", "s_cc_hint", "s_cc_design",
		"s_notes_title", "s_notes_loading", "s_notes_none", "s_notes_failed", "s_notes_github"}
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		texts := map[string]any{}
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, k := range keys {
			s, _ := texts[k].(string)
			if s == "" || (blanks[k] != "" && !strings.Contains(s, blanks[k])) {
				t.Errorf("%s: %s = %q", filepath.Base(f), k, s)
			}
		}
	}
}

// TestTheUpdateStateIsWhatThePluginsTabShows reads the plg that Unraid's
// check fetched against the installed one and compares them as Unraid does,
// so the Info tab never offers an update the Plugins tab would not.
func TestTheUpdateStateIsWhatThePluginsTabShows(t *testing.T) {
	dir := t.TempDir()
	plg := func(name, version string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("<!DOCTYPE PLUGIN [\n  <!ENTITY version   \""+version+"\">\n]>\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	installed := plg("installed.plg", "3.9.3")
	state := func(fetched string) map[string]any {
		out := map[string]any{}
		runPHP(t, "U", "update.php", `echo json_encode($U(`+phpString(installed)+`, `+phpString(fetched)+`));`, &out)
		return out
	}
	if s := state(filepath.Join(dir, "missing.plg")); s["kind"] != "unchecked" {
		t.Errorf("before the first check the state is %v", s)
	}
	if s := state(plg("newer.plg", "3.9.4")); s["kind"] != "available" || s["version"] != "3.9.4" {
		t.Errorf("a newer plg gives %v", s)
	}
	if s := state(plg("same.plg", "3.9.3")); s["kind"] != "current" || s["checkedAt"] == nil {
		t.Errorf("the same plg gives %v", s)
	}
	if s := state(plg("ten.plg", "3.10.0")); s["kind"] != "current" {
		t.Errorf("3.10.0 over 3.9.3 gives %v, and Unraid's strcmp offers no update there", s)
	}
}

// TestTheCheckIsTheOnePluginsTabRuns runs Unraid's own check for ShipLog alone,
// on a POST only, and leads an update to the Plugins tab.
func TestTheCheckIsTheOnePluginsTabRuns(t *testing.T) {
	src := read(t, "server/update-check.php")
	if !strings.Contains(src, "exec('/usr/local/sbin/plugin check shiplog.plg 2>&1', $out, $rc);") {
		t.Error("update-check.php does not run plugin check for shiplog.plg")
	}
	if !strings.Contains(src, "if ($_SERVER['REQUEST_METHOD'] !== 'POST') {") {
		t.Error("update-check.php runs on a GET, which Unraid does not check for its CSRF token")
	}
	page := read(t, "shiplog.page")
	if !regexp.MustCompile(`<a class="button sl-btn tt-fm-primary" id="sl-plugins" href="/Plugins" hidden>`).MatchString(page) {
		t.Error("the update action does not lead to the Plugins tab")
	}
	js := read(t, "scripts/settings.js")
	if !strings.Contains(js, `fetch("/plugins/shiplog/server/update-check.php", { method: "POST"`) {
		t.Error("Check for updates does not post to update-check.php")
	}
}

// TestWhatsNewReadsTheNotesOfThePlg takes the running version's block from
// the installed plg's CHANGES, or the new version's from the plg the check
// fetched, with the category lines as the headings of the release page.
func TestWhatsNewReadsTheNotesOfThePlg(t *testing.T) {
	dir := t.TempDir()
	plg := func(name, version, changes string) string {
		p := filepath.Join(dir, name)
		src := "<?xml version='1.0' standalone='yes'?>\n<!DOCTYPE PLUGIN [\n  <!ENTITY name      \"shiplog\">\n  <!ENTITY version   \"" + version + "\">\n]>\n" +
			"<PLUGIN name=\"&name;\">\n<CHANGES>\nWhat ShipLog does.\n\n" + changes + "</CHANGES>\n</PLUGIN>\n"
		if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	changes := "### 3.9.4\nThe newer one.\n\n🐛 **Fixed**\n- **A fix.** Under Settings &gt; ShipLog.\n\n" +
		"### 3.9.3\nThe running one.\n\n✨ **Added**\n- **A feature.** With &amp; and `code`.\n- **Bold alone.**\n\n### 3.9.2\nAn older one.\n"
	notes := func(installed, fetched string) map[string]string {
		out := map[string]string{}
		runPHP(t, "N", "notes.php", `echo json_encode($N(`+phpString(installed)+`, `+phpString(fetched)+`));`, &out)
		return out
	}
	missing := filepath.Join(dir, "missing.plg")
	running := notes(plg("installed.plg", "3.9.3", changes), missing)
	want := "The running one.\n\n## ✨ Added\n- **A feature.** With & and `code`.\n- **Bold alone.**"
	if running["version"] != "3.9.3" || running["notes"] != want {
		t.Errorf("the running release reads %q: %q, want %q", running["version"], running["notes"], want)
	}
	newer := notes(plg("installed.plg", "3.9.3", changes), plg("fetched.plg", "3.9.4", changes))
	if newer["version"] != "3.9.4" || !strings.HasPrefix(newer["notes"], "The newer one.\n\n## 🐛 Fixed\n- **A fix.** Under Settings > ShipLog.") || strings.Contains(newer["notes"], "running") {
		t.Errorf("while 3.9.4 waits the window reads %q: %q", newer["version"], newer["notes"])
	}
	if b := notes(plg("build.plg", "3.9.3.fix.c8dbae6", changes), missing); b["version"] != "3.9.4" || !strings.HasPrefix(b["notes"], "The newer one.") {
		t.Errorf("a build that is not a release reads %q: %q, want the newest block", b["version"], b["notes"])
	}
	if b := notes(plg("bare.plg", "3.9.5", changes), missing); b["version"] != "3.9.5" || b["notes"] != "" {
		t.Errorf("a release without a block reads %q: %q, want no notes", b["version"], b["notes"])
	}
}

// TestWhatsNewOpensAWindowOfThePlugin keeps the release notes in a window of
// the plugin, never a tab on GitHub: the link to the release page opens its
// foot and Close ends it, and the notes come from ShipLog's own server.
func TestWhatsNewOpensAWindowOfThePlugin(t *testing.T) {
	page := read(t, "shiplog.page")
	if !strings.Contains(page, `<button type="button" class="sl-btn" id="sl-whatsnew" aria-haspopup="dialog"><span class="sl-vdot" aria-hidden="true"></span>`) {
		t.Error("What's new is not a button with the dot that opens a window")
	}
	win := regexp.MustCompile(`(?s)<div class="sl-win sl-win-notes" id="sl-win-notes" role="dialog" aria-modal="true" aria-labelledby="sl-win-notes-t" tabindex="-1" hidden>(.*?)\n</div>`).FindStringSubmatch(page)
	if win == nil {
		t.Fatal("shiplog.page has no window for the release notes")
	}
	foot := regexp.MustCompile(`<div class="sl-win-act">(.*)</div>`).FindStringSubmatch(win[1])
	if foot == nil || !regexp.MustCompile(`^<button type="button" class="sl-win-btn sl-win-link" id="sl-notes-github">.*s_notes_github.*</button><button type="button" class="sl-win-btn" data-sl-close>.*s_close.*</button>$`).MatchString(foot[1]) {
		t.Errorf("the notes window's foot is not the link to the release page, then Close: %v", foot)
	}
	if !strings.Contains(win[1], `<div class="sl-notes" id="sl-notes" tabindex="0"></div>`) {
		t.Error("long notes scroll, and the keyboard has no way into them")
	}
	notesAt, pageAt := strings.Index(page, "<script src=\"<?autov('/plugins/shiplog/scripts/release-notes.js')?>\">"), strings.Index(page, "<script src=\"<?autov('/plugins/shiplog/scripts/settings.js')?>\">")
	if notesAt < 0 || notesAt > pageAt {
		t.Error("the notes are drawn by a script that loads after the page's")
	}
	js := read(t, "scripts/settings.js")
	if !strings.Contains(js, `fetch("/plugins/shiplog/server/release-notes.php", { credentials: "same-origin" })`) {
		t.Error("What's new does not ask ShipLog's server for the notes")
	}
	if strings.Contains(js, "api.github.com") || strings.Contains(js, `sl-whatsnew").href`) {
		t.Error("What's new still leads the browser to GitHub")
	}
	if !strings.Contains(js, `news.addEventListener("click", function () { showWin("notes", news); });`) {
		t.Error("What's new does not open the notes window")
	}
	src := read(t, "server/release-notes.php")
	if !strings.Contains(src, "$notes = include '/usr/local/emhttp/plugins/shiplog/include/notes.php';") || strings.Contains(src, "curl") {
		t.Error("release-notes.php does not answer from the plg on the server")
	}
	css := read(t, "styles/settings.css")
	if !strings.Contains(css, ".sl-win.sl-win-notes { width: min(760px, calc(100vw - 32px)); }") {
		t.Error("the notes window is not wide enough to read")
	}
	if !strings.Contains(css, ".sl-win-notes .sl-win-act { justify-content: space-between; }") {
		t.Error("the link to the release page leans on a margin TowerTorch overrides")
	}
}

// TestTheDotBeforeWhatsNewStaysGrey has nothing to mark: ShipLog marks no
// changes, so nothing is ever unseen.
func TestTheDotBeforeWhatsNewStaysGrey(t *testing.T) {
	css := read(t, "styles/settings.css")
	if !strings.Contains(css, `/* The dot before "What's new", grey while nothing is unseen. */`+"\n.sl-vdot { flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--sl-dim); }") {
		t.Error("the dot before What's new is not grey")
	}
	if strings.Contains(css, "data-unseen") {
		t.Error("settings.css keeps a rule for an unseen change ShipLog never marks")
	}
}

// TestTheReleaseNotesPassTheirNodeTests runs the suite that draws notes into a
// small document of its own.
func TestTheReleaseNotesPassTheirNodeTests(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("node is not installed")
		}
		t.Skip("node is not installed")
	}
	if out, err := exec.Command(node, "--test", filepath.Join("test", "release-notes.test.js")).CombinedOutput(); err != nil {
		t.Fatalf("node --test: %v\n%s", err, out)
	}
}

// TestTheVersionCardPassesItsNodeTests runs the suite for the words and tone
// of each update state, which the page loads before the script that draws it.
func TestTheVersionCardPassesItsNodeTests(t *testing.T) {
	page := read(t, "shiplog.page")
	cardAt := strings.Index(page, "<script src=\"<?autov('/plugins/shiplog/scripts/version-card.js')?>\">")
	pageAt := strings.Index(page, "<script src=\"<?autov('/plugins/shiplog/scripts/settings.js')?>\">")
	if cardAt < 0 || pageAt < cardAt {
		t.Error("the page does not load version-card.js before settings.js")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("node is not installed")
		}
		t.Skip("node is not installed")
	}
	if out, err := exec.Command(node, "--test", filepath.Join("test", "version-card.test.js")).CombinedOutput(); err != nil {
		t.Fatalf("node --test: %v\n%s", err, out)
	}
}

// TestAVersionNumberIsAFingersTarget gives a linked number GlimStone's 24px
// target and takes the room back with the margin, so the line keeps its height.
func TestAVersionNumberIsAFingersTarget(t *testing.T) {
	css := read(t, "styles/settings.css")
	rule := regexp.MustCompile(`(?s)\.sl-cfg a\.sl-ver \{([^}]*)\}`).FindStringSubmatch(css)
	if rule == nil || !strings.Contains(rule[1], "padding: 2px 0;") || !strings.Contains(rule[1], "margin: -2px 0;") {
		t.Errorf("a linked version is not a 24px target: %v", rule)
	}
}

// TestALongNumberDropsUnderTheName lets a branch build's number leave the
// app's name its line on a phone and break where it has to.
func TestALongNumberDropsUnderTheName(t *testing.T) {
	css := read(t, "styles/settings.css")
	if !regexp.MustCompile(`\.sl-vhead \{[^}]*flex-wrap: wrap;[^}]*gap: 2px 8px;`).MatchString(css) {
		t.Error("the Version card's head does not wrap")
	}
	if !regexp.MustCompile(`\.sl-cfg \.sl-vhead \.sl-ver \{[^}]*min-width: 0;[^}]*margin-inline-start: auto;[^}]*overflow-wrap: anywhere;`).MatchString(css) {
		t.Error("a long number in the head neither keeps to the end nor breaks")
	}
}

// TestAComponentRowBreaksBeforeItsEnd moves a row's number under its name on
// a narrow card, to the end of the line, instead of squeezing it.
func TestAComponentRowBreaksBeforeItsEnd(t *testing.T) {
	css := read(t, "styles/settings.css")
	if !regexp.MustCompile(`\.sl-vrow \{[^}]*flex-wrap: wrap;[^}]*gap: 2px 8px;`).MatchString(css) {
		t.Error("a component row does not wrap")
	}
	if !regexp.MustCompile(`\.sl-cfg \.sl-vrow \.sl-ver \{[^}]*margin-inline-start: auto;`).MatchString(css) {
		t.Error("a wrapped number leaves the end of the row")
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
// plain text in the text colour, which takes the accent's ink under the
// pointer alone, as in every app of the house.
func TestAVersionNeverLooksLikeAButton(t *testing.T) {
	page := read(t, "shiplog.page")
	if n := strings.Count(page, `class="sl-ver`); n != 2 {
		t.Fatalf("found %d version displays, want the link and the plain build in $slNum", n)
	}
	if strings.Count(page, "<?=$slNum(") != 2 {
		t.Error("the app and GlimStone do not both show their number through $slNum")
	}
	if regexp.MustCompile(`class="[^"]*button[^"]*sl-ver|class="[^"]*sl-ver[^"]*button`).MatchString(page) {
		t.Error("a version wears Unraid's button class")
	}
	css := read(t, "styles/settings.css")
	rule := regexp.MustCompile(`(?s)\.sl-cfg a\.sl-ver \{([^}]*)\}`).FindStringSubmatch(css)
	if rule == nil {
		t.Fatal("settings.css has no rule for a linked version")
	}
	for _, want := range []string{"background: none", "border: 0", "color: inherit", "text-decoration: none"} {
		if !strings.Contains(rule[1], want) {
			t.Errorf("a linked version lacks %q, so it can look like a button", want)
		}
	}
	if !strings.Contains(css, "#sl-cfg a.sl-ver { color: inherit !important; text-decoration: none !important; }") {
		t.Error("the theme plugin's accent colours or underlines a linked version like a control")
	}
	hover := regexp.MustCompile(`(?s)@media \(hover: hover\) \{\s*#sl-cfg a\.sl-ver:hover \{ color: var\(--sl-focus\) !important; \}`)
	if !hover.MatchString(css) || !strings.Contains(css, "#sl-cfg a.sl-ver:focus-visible { color: var(--sl-focus) !important; }") {
		t.Error("a linked version does not take the accent's ink under the pointer and on focus")
	}
	if strings.Contains(css, "a.sl-ver:is(:hover") || regexp.MustCompile(`a\.sl-ver[^{]*\{[^}]*text-decoration: underline`).MatchString(css) {
		t.Error("a linked version is underlined")
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
