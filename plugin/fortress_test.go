package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestEveryLanguageNamesTheFortressTab keeps the tab's name and the backup
// warning in every language. The card brings its own texts, so the language
// files carry no other fortress text.
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
		for _, k := range []string{"s_fortress", "s_fortress_no_backup"} {
			if v := texts[k]; v == "" || banned.MatchString(v) {
				t.Errorf("%s: %s = %q", filepath.Base(f), k, v)
			}
		}
		for k := range texts {
			if strings.HasPrefix(k, "s_fortress_") && k != "s_fortress_no_backup" {
				t.Errorf("%s still carries %s, which the card words itself", filepath.Base(f), k)
			}
		}
	}
}

func TestTheFortressHasItsOwnTabJustAboveInfo(t *testing.T) {
	page := read(t, "shiplog.page")
	var tabs []string
	for _, m := range regexp.MustCompile(`data-sl-tab="([a-z]+)"`).FindAllStringSubmatch(page, -1) {
		tabs = append(tabs, m[1])
	}
	if n := len(tabs); n < 3 || tabs[0] != "general" || tabs[n-2] != "fortress" || tabs[n-1] != "info" {
		t.Errorf("tabs = %v, want general first and fortress just above info", tabs)
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

// TestTheFortressCardIsGlimStonesOwnUnchanged holds the copy to GlimStone's
// release tag, or to its release branch while the tag is still to come.
func TestTheFortressCardIsGlimStonesOwnUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "glimstone.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Targets []struct {
			Name    string `json:"name"`
			Mode    string `json:"mode"`
			Version string `json:"version"`
			Files   []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"files"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	var version string
	sources := map[string]string{}
	for _, target := range cfg.Targets {
		if target.Mode != "copy" || target.Name != "fortress card" {
			continue
		}
		version = target.Version
		for _, f := range target.Files {
			sources[f.To] = f.From
		}
	}
	if version == "" || len(sources) != 2 {
		t.Fatalf("glimstone.json has no fortress card copy target with its two files: %+v", cfg.Targets)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	for copied, source := range sources {
		mine, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(copied)))
		if err != nil {
			t.Errorf("%s: %v", copied, err)
			continue
		}
		theirs, ref, err := glimstoneFile(client, version, source)
		if err != nil {
			t.Errorf("%s: %v", copied, err)
			continue
		}
		if !bytes.Equal(mine, theirs) {
			t.Errorf("%s is not GlimStone's %s at %s; copy it again unchanged", copied, source, ref)
		}
	}
}

// glimstoneFile reads path from GlimStone's tag v<version>, falling back to
// the branch rel/<version> until that release is tagged.
func glimstoneFile(client *http.Client, version, path string) ([]byte, string, error) {
	for _, ref := range []string{"v" + version, "rel/" + version} {
		url := "https://raw.githubusercontent.com/junkerderprovinz/glimstone/" + ref + "/" + path
		resp, err := client.Get(url)
		if err != nil {
			return nil, ref, err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if err != nil || resp.StatusCode != http.StatusOK {
			return nil, ref, fmt.Errorf("%s: %s, %v", url, resp.Status, err)
		}
		return body, ref, nil
	}
	return nil, "", fmt.Errorf("GlimStone has %s neither at v%s nor on rel/%s", path, version, version)
}
