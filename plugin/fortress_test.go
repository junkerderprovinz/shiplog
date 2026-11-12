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
		"s_fortress_changed": false,
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

// TestARemovedServerIsSentToTheCurrentPhrase holds the removed note to the
// current phrase, since the one ShipLog knew may have changed since.
func TestARemovedServerIsSentToTheCurrentPhrase(t *testing.T) {
	for lang, want := range map[string]string{"en": "the current phrase", "de": "der aktuellen Phrase"} {
		raw, err := os.ReadFile(filepath.Join(pluginDir, "lang", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(texts["s_fortress_removed"], want) {
			t.Errorf("%s: s_fortress_removed = %q, want %q in it", lang, texts["s_fortress_removed"], want)
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
	if !strings.Contains(page, "s_fortress_changed") {
		t.Error("the page never says that the fortress phrase changed")
	}
	js := read(t, "scripts/settings.js")
	if !strings.Contains(js, `"fortress/join"`) {
		t.Error("settings.js does not post the phrase to fortress/join")
	}
	if !strings.Contains(js, "s.phraseChanged") {
		t.Error("settings.js never offers the new words after a phrase change")
	}
}

// TestEveryLanguageCanRemoveAMember keeps the texts around removing a member
// in every language, each with its placeholders and without a long dash or a
// semicolon.
func TestEveryLanguageCanRemoveAMember(t *testing.T) {
	keys := map[string][]string{
		"s_fortress_others": nil, "s_fortress_online": nil, "s_fortress_away": nil,
		"s_fortress_remove": nil, "s_fortress_member_on": {"{app}", "{device}"},
		"s_fortress_remove_title": {"{device}"}, "s_fortress_remove_body": {"{device}"},
		"s_fortress_removed_title": nil, "s_fortress_removed_body": {"{device}"},
		"s_fortress_later": nil, "s_fortress_removed_note": {"{devices}"}, "s_fortress_change": nil,
		"s_fortress_change_in": {"{app}"}, "s_fortress_change_any": nil, "s_fortress_cancel": nil,
		"s_fortress_remove_failed": nil,
	}
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	banned := regexp.MustCompile(`[\x{2013}\x{2014};]|%s`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for k, holes := range keys {
			v := texts[k]
			if v == "" {
				t.Errorf("%s lacks %s", filepath.Base(f), k)
				continue
			}
			// Greek ends a question with the semicolon's sign.
			prose := v
			if filepath.Base(f) == "el.json" {
				prose = strings.TrimSuffix(v, ";")
			}
			if banned.MatchString(prose) {
				t.Errorf("%s: %s carries a long dash, a semicolon or %%s: %q", filepath.Base(f), k, v)
			}
			if got := len(regexp.MustCompile(`\{[a-z]+\}`).FindAllString(v, -1)); got != len(holes) {
				t.Errorf("%s: %s has %d placeholders, want %v: %q", filepath.Base(f), k, got, holes, v)
			}
			for _, h := range holes {
				if !strings.Contains(v, h) {
					t.Errorf("%s: %s lacks %s: %q", filepath.Base(f), k, h, v)
				}
			}
		}
	}
}

func TestTheSettingsPageRemovesAMemberThroughTheProxy(t *testing.T) {
	proxy := read(t, "server/status.php")
	if !strings.Contains(proxy, "'fortress/member' => ['DELETE']") {
		t.Error("status.php does not pass a removal on")
	}
	if !strings.Contains(proxy, "preg_replace('/[^a-z0-9-]/', '', $_POST['id'])") {
		t.Error("status.php passes the member id on unchecked")
	}
	page := read(t, "shiplog.page")
	for _, id := range []string{"sl-fortress-others", "sl-fortress-list", "sl-fortress-out", "sl-win-remove", "sl-rm-go", "sl-rm-later", "sl-rm-change"} {
		if !strings.Contains(page, `id="`+id+`"`) {
			t.Errorf("the page lacks #%s", id)
		}
	}
	js := read(t, "scripts/settings.js")
	for _, want := range []string{`path: "fortress/member"`, `method: "DELETE"`, "csrf_token", "s.removedMembers", "T.s_fortress_removed_title", "T.s_fortress_removed_note"} {
		if !strings.Contains(js, want) {
			t.Errorf("settings.js lacks %s", want)
		}
	}
}

// TestTheRemoveWindowPaintsNothingRed keeps Remove in the colours of every
// other button, as GlimStone asks of anything that deletes.
func TestTheRemoveWindowPaintsNothingRed(t *testing.T) {
	page := read(t, "shiplog.page")
	win := regexp.MustCompile(`(?s)<div class="sl-win sl-win-remove".*?</div>\s*</div>`).FindString(page)
	if win == "" {
		t.Fatal("no remove window in shiplog.page")
	}
	if strings.Contains(win, "sl-err") || strings.Contains(win, "danger") {
		t.Errorf("the remove window paints a button red: %s", win)
	}
}
