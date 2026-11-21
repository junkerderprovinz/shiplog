package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// openRouterTexts are the strings the OpenRouter rows show, by the placeholders
// each has to keep.
var openRouterTexts = map[string][]string{
	"s_ai_provider": nil, "s_ai_local": nil, "s_or_notice": nil,
	"s_or_model": nil, "s_or_model_hint": nil,
	"s_or_key": nil, "s_or_key_hint": nil, "s_or_key_ph": nil, "s_or_key_saved": {"%s"}, "s_or_key_remove": nil,
	"s_or_check_lbl": nil, "s_or_check_hint": nil, "s_or_check_ok": nil,
	"s_or_try_lbl": nil, "s_or_try_hint": nil, "s_or_try": nil, "s_or_try_ok": nil,
	"s_or_err_no_key": nil, "s_or_err_no_model": nil, "s_or_err_invalid_key": nil, "s_or_err_no_credit": nil,
	"s_or_err_model_unavailable": nil, "s_or_err_rate_limited": nil, "s_or_err_network": nil,
	"s_or_err_bad_answer": nil, "s_or_err_failed": nil, "s_or_err_bad_key": nil, "s_or_err_save_failed": nil,
}

// TestTheKeyFieldIsLeftOutOfTheSavedSettings keeps the key out of shiplog.cfg:
// every save posts the whole form, so the field carries no name, and it
// starts empty, so the page holds no key to show.
func TestTheKeyFieldIsLeftOutOfTheSavedSettings(t *testing.T) {
	page := read(t, "shiplog.page")
	field := regexp.MustCompile(`<input[^>]*id="sl-or-key"[^>]*>`).FindString(page)
	if field == "" {
		t.Fatal("shiplog.page has no key field")
	}
	if strings.Contains(field, "name=") || strings.Contains(field, "value=") {
		t.Errorf("the key field would post or show the key: %s", field)
	}
	if !strings.Contains(field, `type="password"`) {
		t.Errorf("the key field does not mask what is typed: %s", field)
	}
	for _, leak := range []string{"OPENROUTER_KEY'", "openrouter.key"} {
		if strings.Contains(page, leak) {
			t.Errorf("shiplog.page reads the key (%s)", leak)
		}
	}
}

// TestTheKeyTravelsInAPostBodyOnly sends the key to the engine through the
// proxy's POST, which Unraid checks for its CSRF token, and never in a URL.
func TestTheKeyTravelsInAPostBodyOnly(t *testing.T) {
	js := read(t, "scripts/settings.js")
	if !strings.Contains(js, `orEngine("openrouter/key", "PUT", { key: v })`) {
		t.Error("settings.js does not save the key through the engine's POST")
	}
	if regexp.MustCompile(`[?&](key|token)=[^"]*" *\+ *[^;]*orKey`).MatchString(js) || strings.Contains(js, "openrouter-check.php") {
		t.Error("settings.js puts the key in a URL")
	}
	if !regexp.MustCompile(`csrf_token: window\.csrf_token`).MatchString(js) {
		t.Error("the engine's POST goes without Unraid's CSRF token")
	}
	proxy := read(t, filepath.Join("server", "status.php"))
	if !strings.Contains(proxy, `'openrouter/key' => ['PUT', 'DELETE']`) {
		t.Error("status.php does not take the key as a write")
	}
	reads := regexp.MustCompile(`(?s)\$reads\s*=\s*\[(.*?)\];`).FindStringSubmatch(proxy)
	if reads == nil || strings.Contains(reads[1], "openrouter/key") {
		t.Error("the key route must not be readable through status.php")
	}
}

// TestTheEngineReadsTheKeyFromItsFile hands the engine the file's path, so the
// key is in no environment and no cfg.
func TestTheEngineReadsTheKeyFromItsFile(t *testing.T) {
	rc := read(t, filepath.Join("scripts", "rc.shiplog"))
	for _, want := range []string{`AI_PROVIDER="local"`, `OPENROUTER_KEY_FILE="/boot/config/plugins/${NAME}/openrouter.key"`, "AI_PROVIDER OPENROUTER_MODEL OPENROUTER_KEY_FILE"} {
		if !strings.Contains(rc, want) {
			t.Errorf("rc.shiplog lacks %s", want)
		}
	}
	if regexp.MustCompile(`OPENROUTER_KEY[ =]|OPENROUTER_API_KEY`).MatchString(rc) {
		t.Error("rc.shiplog puts the key itself into the environment")
	}
}

// TestLocalStaysTheChoiceUntilOpenRouterIsPicked opens the page on Local for a
// cfg without the setting, with the OpenRouter rows hidden.
func TestLocalStaysTheChoiceUntilOpenRouterIsPicked(t *testing.T) {
	page := read(t, "shiplog.page")
	if !strings.Contains(page, `$aiProvider = (isset($cfg['AI_PROVIDER']) && $cfg['AI_PROVIDER'] === 'openrouter') ? 'openrouter' : 'local';`) {
		t.Error("the page does not fall back to Local")
	}
	rows := regexp.MustCompile(`data-sl-ai="openrouter"<\?=\$aiProvider==='openrouter'\?'':' hidden'\?>`).FindAllString(page, -1)
	if len(rows) != 4 {
		t.Errorf("%d OpenRouter rows hide while Local is chosen, want 4", len(rows))
	}
	js := read(t, "scripts/settings.js")
	if !strings.Contains(js, `if (provider.value === "openrouter") orLoad();`) {
		t.Error("the page asks OpenRouter's model list while Local is chosen")
	}
}

// TestTheNoticeStandsBesideTheChoice shows what leaves the server before
// OpenRouter is picked, not only after.
func TestTheNoticeStandsBesideTheChoice(t *testing.T) {
	page := read(t, "shiplog.page")
	notice := regexp.MustCompile(`<p class="sl-notice" id="sl-or-notice"[^>]*>`).FindString(page)
	if notice == "" || strings.Contains(notice, "hidden") || strings.Contains(notice, "data-sl-ai") {
		t.Errorf("the notice is missing or hides with a provider: %q", notice)
	}
	if !strings.Contains(page, `aria-describedby="sl-or-notice"`) {
		t.Error("the provider choice does not point at its notice")
	}
}

// TestEveryLanguageSaysWhatLeavesTheServer keeps the OpenRouter strings in
// every language with their placeholders, and the notice naming the
// changelog, the image and the versions.
func TestEveryLanguageSaysWhatLeavesTheServer(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	page, js := read(t, "shiplog.page"), read(t, "scripts/settings.js")
	for k := range openRouterTexts {
		short := strings.TrimPrefix(k, "s_or_")
		if !strings.Contains(page, "$L['"+k+"']") && !strings.Contains(js, "T."+k) && !strings.HasPrefix(short, "err_") && !strings.HasSuffix(k, "_ok") {
			t.Errorf("neither the page nor the script shows %s", k)
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
		for k, marks := range openRouterTexts {
			if texts[k] == "" {
				t.Errorf("%s lacks %s", filepath.Base(f), k)
			}
			for _, m := range marks {
				if !strings.Contains(texts[k], m) {
					t.Errorf("%s: %s lacks %s", filepath.Base(f), k, m)
				}
			}
		}
		if !strings.Contains(texts["s_or_notice"], "openrouter.ai") {
			t.Errorf("%s: the notice does not name openrouter.ai", filepath.Base(f))
		}
	}
}
