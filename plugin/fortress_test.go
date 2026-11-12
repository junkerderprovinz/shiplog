package plugin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestEveryLanguageNamesTheFortressTab keeps the tab's name in every
// language. The card brings its own texts, so the language files carry no
// other fortress text.
func TestEveryLanguageNamesTheFortressTab(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	banned := regexp.MustCompile(`[\x{2013}\x{2014};]`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if v := texts["s_fortress"]; v == "" || banned.MatchString(v) {
			t.Errorf("%s: s_fortress = %q", filepath.Base(f), v)
		}
		for k := range texts {
			if strings.HasPrefix(k, "s_fortress_") {
				t.Errorf("%s still carries %s, which the card words itself", filepath.Base(f), k)
			}
		}
	}
}

func TestTheFortressHasItsOwnTabRightAfterGeneral(t *testing.T) {
	page := read(t, "shiplog.page")
	var tabs []string
	for _, m := range regexp.MustCompile(`data-sl-tab="([a-z]+)"`).FindAllStringSubmatch(page, -1) {
		tabs = append(tabs, m[1])
	}
	if len(tabs) < 2 || tabs[0] != "general" || tabs[1] != "fortress" {
		t.Errorf("tabs = %v, want fortress right after general", tabs)
	}
	tab := regexp.MustCompile(`<button[^>]*data-sl-tab="fortress"[^>]*>.*?</button>`).FindString(page)
	if !strings.Contains(tab, `$L['s_fortress']`) || !strings.Contains(tab, `aria-controls="sl-pane-fortress"`) {
		t.Errorf("the fortress tab is %s", tab)
	}
	pane := regexp.MustCompile(`(?s)<div class="sl-pane" role="tabpanel" id="sl-pane-fortress"[^>]*>(.*?)\n  </div>\n`).FindStringSubmatch(page)
	if pane == nil || !strings.Contains(pane[1], `<div id="sl-fortress"></div>`) {
		t.Fatalf("the fortress pane holds no empty element for the card: %v", pane)
	}
	card := strings.Index(page, "scripts/fortress-card.js")
	adapter := strings.Index(page, "scripts/fortress.js")
	if card < 0 || adapter < card || !strings.Contains(page, "styles/fortress-card.css") {
		t.Error("the page has to load fortress-card.css, fortress-card.js and then fortress.js")
	}
}

func TestTheProxyPassesEveryFortressStepOn(t *testing.T) {
	proxy := read(t, "server/status.php")
	for _, want := range []string{
		"'fortress' => '/api/fortress'",
		"'fortress-member' => '/api/fortress/members/'",
		"'fortress/join' => ['POST']",
		"'fortress/leave' => ['POST']",
		"'fortress/member' => ['DELETE']",
		"preg_replace('/[^a-z0-9-]/', '', $_POST['id'])",
		"preg_replace('/[^a-z0-9-]/', '', $_GET['member'])",
	} {
		if !strings.Contains(proxy, want) {
			t.Errorf("status.php lacks %s", want)
		}
	}
}

// TestTheAdapterHandsTheCardShipLogsFortress pins what the card cannot see:
// which call goes where, the token on every write, and the engine's names
// turned into the card's.
func TestTheAdapterHandsTheCardShipLogsFortress(t *testing.T) {
	js := read(t, "scripts/fortress.js")
	for _, want := range []string{
		`path: "fortress"`,
		`path: "fortress/join", method: "POST"`,
		`path: "fortress/leave", method: "POST"`,
		`path: "fortress/member", method: "DELETE"`,
		`path: "fortress-member", member: id, lang: lang`,
		"fields.csrf_token = window.csrf_token",
		"removedHere: !!s.removed",
		"removed: s.removedMembers",
		`noFortress: "noMember"`,
		`app: { id: "shiplog", name: "ShipLog" }`,
		"window.SL_LANG",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("fortress.js lacks %s", want)
		}
	}
	for _, banned := range []string{".address", "window.open("} {
		if strings.Contains(js, banned) {
			t.Errorf("fortress.js uses %s", banned)
		}
	}
}

func TestTheFortressCardIsGlimStonesOwnUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "glimstone.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Vanilla struct {
			Ref   string            `json:"ref"`
			Files map[string]string `json:"files"`
		} `json:"vanilla"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Vanilla.Ref == "" || len(cfg.Vanilla.Files) != 2 {
		t.Fatalf("glimstone.json names no GlimStone copy: %+v", cfg.Vanilla)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	for copied, source := range cfg.Vanilla.Files {
		mine, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(copied)))
		if err != nil {
			t.Errorf("%s: %v", copied, err)
			continue
		}
		url := "https://raw.githubusercontent.com/junkerderprovinz/glimstone/" + cfg.Vanilla.Ref + "/" + source
		resp, err := client.Get(url)
		if err != nil {
			t.Errorf("%s: %v", url, err)
			continue
		}
		theirs, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("%s: %s, %v", url, resp.Status, err)
			continue
		}
		if !bytes.Equal(mine, theirs) {
			t.Errorf("%s is not GlimStone's %s at %s; copy it again unchanged", copied, source, cfg.Vanilla.Ref)
		}
	}
}
