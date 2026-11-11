package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEveryLanguageOffersTheFortressAgain keeps the fortress card's texts in
// every language, with their placeholder and without a long dash or a
// semicolon.
func TestEveryLanguageOffersTheFortressAgain(t *testing.T) {
	keys := map[string]bool{
		"s_fortress": false, "s_fortress_member": false, "s_fortress_in": true, "s_fortress_out": false,
		"s_fortress_removed": false, "s_fortress_join": false, "s_fortress_join_desc": false, "s_fortress_join_ph": false,
		"s_fortress_join_btn": false, "s_fortress_joined": false, "s_fortress_bad_phrase": false, "s_fortress_failed": true,
	}
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
		for k, arg := range keys {
			v := texts[k]
			switch {
			case v == "":
				t.Errorf("%s lacks %s", filepath.Base(f), k)
			case arg != strings.Contains(v, "%s"):
				t.Errorf("%s: %s has the wrong placeholder: %q", filepath.Base(f), k, v)
			case banned.MatchString(v):
				t.Errorf("%s: %s carries a long dash or a semicolon: %q", filepath.Base(f), k, v)
			}
		}
	}
}

func TestTheSettingsPageJoinsThroughTheProxy(t *testing.T) {
	proxy := read(t, "server/status.php")
	for _, want := range []string{"'fortress' => '/api/fortress'", "'fortress/join' => ['POST']"} {
		if !strings.Contains(proxy, want) {
			t.Errorf("status.php lacks %s", want)
		}
	}
	page := read(t, "shiplog.page")
	field := regexp.MustCompile(`<input[^>]*id="sl-fortress-phrase"[^>]*>`).FindString(page)
	if field == "" || !strings.Contains(field, `type="password"`) || strings.Contains(field, "name=") {
		t.Errorf("the phrase field %q has to hide the words and stay out of the saved form", field)
	}
	if !strings.Contains(page, "s_fortress_removed") {
		t.Error("the page never says that ShipLog was taken out")
	}
	if !strings.Contains(read(t, "scripts/settings.js"), `"fortress/join"`) {
		t.Error("settings.js does not post the phrase to fortress/join")
	}
}
