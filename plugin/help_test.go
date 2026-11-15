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
		if target.Version != "4.5.2" {
			t.Errorf("glimstone.json pins %s, want 4.5.2", target.Version)
		}
	}
	page := read(t, "shiplog.page")
	if !strings.Contains(page, "$slGlimstoneVer = '4.5.2';") {
		t.Error("the Version card does not name GlimStone 4.5.2")
	}
	row := regexp.MustCompile(`(?s)<span>GlimStone</span></div>\s*<div class="sl-ctl">(.*?)</div>`).FindStringSubmatch(page)
	if row == nil || !strings.Contains(row[1], `href="https://github.com/junkerderprovinz/glimstone/releases/latest"`) || !strings.Contains(row[1], `$slE($slGlimstoneVer)`) {
		t.Errorf("the GlimStone row does not open the newest release: %v", row)
	}
}
