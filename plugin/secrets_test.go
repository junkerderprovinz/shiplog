package plugin

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestACheckRefusesATokenInItsURL keeps the GitHub and Docker Hub tokens out
// of nginx's access log, which records every URL with its query: a check that
// gets its token as a GET parameter answers without using it.
func TestACheckRefusesATokenInItsURL(t *testing.T) {
	for _, check := range []string{"github-check.php", "dockerhub-check.php"} {
		path, err := filepath.Abs(filepath.Join(pluginDir, "server", check))
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			OK      bool   `json:"ok"`
			Message string `json:"message"`
		}
		runPHP(t, "N", "notify.php", `$_SERVER['REQUEST_METHOD'] = 'GET'; $_GET = ['user' => 'someone', 'token' => 'ghp_leaked']; include `+phpString(path)+`;`, &got)
		if got.OK || strings.Contains(got.Message, "ghp_leaked") {
			t.Errorf("%s took a token from its URL: %+v", check, got)
		}
		src := read(t, filepath.Join("server", check))
		if strings.Contains(src, "$_GET") {
			t.Errorf("%s still reads the query string", check)
		}
	}
}

// TestTheChecksPostTheirTokens sends the tokens to the checks in the body of
// a POST, which Unraid checks for its CSRF token.
func TestTheChecksPostTheirTokens(t *testing.T) {
	js := read(t, filepath.Join("scripts", "settings.js"))
	if regexp.MustCompile(`[?&](token|user)=`).MatchString(js) {
		t.Error("settings.js still puts a token or a login in a check's URL")
	}
	for _, kind := range []string{"github", "dockerhub"} {
		if !regexp.MustCompile(kind + `: function \(\) \{ return \{ post: \{`).MatchString(js) {
			t.Errorf("the %s check does not post its fields", kind)
		}
	}
	if !strings.Contains(js, `body.set("csrf_token", window.csrf_token || "")`) {
		t.Error("a posted check goes without Unraid's CSRF token")
	}
}

// TestTheSettingsFileIsForRootAlone keeps shiplog.cfg, which holds the GitHub
// and Docker Hub tokens, readable by root only, also when an older install or
// a first save left it 0644.
func TestTheSettingsFileIsForRootAlone(t *testing.T) {
	rc := read(t, filepath.Join("scripts", "rc.shiplog"))
	fix := strings.Index(rc, `[ -f "$CFG" ] && chmod 600 "$CFG"`)
	load := strings.Index(rc, `[ -f "$CFG" ] && . "$CFG"`)
	if fix < 0 || load < 0 || fix > load {
		t.Error("rc.shiplog does not make the cfg root-only before it reads it")
	}
	page := read(t, "shiplog.page")
	if !strings.Contains(page, `@chmod('/boot/config/plugins/shiplog/shiplog.cfg', 0600);`) {
		t.Error("the settings page leaves the cfg as update.php created it")
	}
	raw, err := os.ReadFile("shiplog.plg")
	if err != nil {
		t.Fatal(err)
	}
	plg := string(raw)
	post := regexp.MustCompile(`(?s)<!-- Post-install.*?</FILE>`).FindString(plg)
	if !strings.Contains(post, `chmod 600 &plgPath;/&slug;.cfg`) {
		t.Error("an install or upgrade leaves an existing cfg readable by everyone")
	}
}

// TestUninstallTakesTheOpenRouterKey deletes the key with the plugin and
// keeps the other settings, as before.
func TestUninstallTakesTheOpenRouterKey(t *testing.T) {
	raw, err := os.ReadFile("shiplog.plg")
	if err != nil {
		t.Fatal(err)
	}
	remove := regexp.MustCompile(`(?s)<FILE Run="/bin/bash" Method="remove">(.*?)</FILE>`).FindStringSubmatch(string(raw))
	if remove == nil {
		t.Fatal("shiplog.plg has no remove script")
	}
	if !strings.Contains(remove[1], "rm -f &plgPath;/openrouter.key") {
		t.Error("uninstall leaves the OpenRouter key on the flash")
	}
	if regexp.MustCompile(`rm [^\n]*(\.cfg|notify\.json|&plgPath;/?\s|&plgPath;/\*\s)`).MatchString(remove[1]) {
		t.Error("uninstall removes more of the settings than the key")
	}
}
