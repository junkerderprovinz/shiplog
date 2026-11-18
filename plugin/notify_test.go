package plugin

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runPHP runs body after including include/notify.php as $N and decodes what
// it echoes as JSON.
func runPHP(t *testing.T, body string, out any) {
	t.Helper()
	php, err := exec.LookPath("php")
	if err != nil {
		// The runners have php, so CI never skips the page's half of the move.
		if os.Getenv("CI") != "" {
			t.Fatal("php is not installed")
		}
		t.Skip("php is not installed")
	}
	inc, err := filepath.Abs(filepath.Join(pluginDir, "include", "notify.php"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "t.php")
	src := "<?php\n$N = include " + phpString(inc) + ";\n" + body
	if err := os.WriteFile(script, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command(php, script).CombinedOutput()
	if err != nil {
		t.Fatalf("php: %v\n%s", err, b)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("php printed %s: %v", b, err)
	}
}

func phpString(s string) string { return "'" + strings.ReplaceAll(s, `'`, `\'`) + "'" }

// TestTheOldKeysGoOnceTheChannelsAreKept keeps the secrets of the old
// settings in notify.json alone, once the engine has built that file.
func TestTheOldKeysGoOnceTheChannelsAreKept(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "shiplog.cfg")
	channels := filepath.Join(dir, "notify.json")
	if err := os.WriteFile(cfg, []byte("ENABLE=\"true\"\nMATRIX_TOKEN=\"syt_x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var none string
	runPHP(t, `$N['tidy'](`+phpString(channels)+`, `+phpString(cfg)+`); echo json_encode(file_get_contents(`+phpString(cfg)+`));`, &none)
	if !strings.Contains(none, "MATRIX_TOKEN") {
		t.Error("the old keys went before notify.json existed")
	}
	if err := os.WriteFile(channels, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var after string
	runPHP(t, `$N['tidy'](`+phpString(channels)+`, `+phpString(cfg)+`); echo json_encode(file_get_contents(`+phpString(cfg)+`));`, &after)
	if strings.Contains(after, "MATRIX_TOKEN") || !strings.Contains(after, `ENABLE="true"`) {
		t.Errorf("cfg after the move = %q", after)
	}
}

// TestTheNotificationsTabCarriesTheSharedCard keeps the channel card on the
// page with its files, the engine's routes behind the proxy and every text
// of the card in every language.
func TestTheNotificationsTabCarriesTheSharedCard(t *testing.T) {
	page := read(t, "shiplog.page")
	for _, want := range []string{`<div id="sl-notify"></div>`, "/plugins/shiplog/scripts/notify-card.js", "/plugins/shiplog/scripts/notify-marks.js", "/plugins/shiplog/styles/notify-card.css"} {
		if !strings.Contains(page, want) {
			t.Errorf("shiplog.page lacks %s", want)
		}
	}
	for _, f := range []string{"scripts/notify-card.js", "scripts/notify-marks.js", "styles/notify-card.css", "marks/README.md", "marks/LICENSE-dashboard-icons.txt"} {
		if _, err := os.Stat(filepath.Join(pluginDir, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	proxy := read(t, filepath.Join("server", "status.php"))
	for _, want := range []string{`'notify' => '/api/notify'`, `'notify' => ['PUT']`, `'notify/test' => ['POST']`} {
		if !strings.Contains(proxy, want) {
			t.Errorf("status.php lacks %s", want)
		}
	}
	if !strings.Contains(read(t, filepath.Join("scripts", "settings.js")), "window.GlimNotifyCard.mount(byId(\"sl-notify\")") {
		t.Error("settings.js does not mount the card")
	}
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	var en map[string]any
	if err := json.Unmarshal([]byte(read(t, filepath.Join("lang", "en.json"))), &en); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range en {
		if strings.HasPrefix(k, "nc_") {
			keys = append(keys, k)
		}
	}
	if len(keys) < 70 {
		t.Fatalf("en.json has %d card texts", len(keys))
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]any
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, k := range keys {
			if v, _ := texts[k].(string); v == "" {
				t.Errorf("%s lacks %s", filepath.Base(f), k)
			}
		}
	}
}

func TestThePageTakesOnlyTheOldKeysOutOfTheCfg(t *testing.T) {
	cfg, err := os.ReadFile(filepath.Join("..", "internal", "notify", "testdata", "shiplog.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	var got string
	runPHP(t, `echo json_encode($N['strip'](`+phpString(string(cfg))+`, $N['keys']));`, &got)
	for _, k := range []string{"MATRIX_HOMESERVER=", "MATRIX_TOKEN=", "MATRIX_ROOM=", "APPRISE_URL=", "UNRAID_NOTIFY="} {
		if strings.Contains(got, k) {
			t.Errorf("%s is still in the cfg", k)
		}
	}
	if want := strings.Count(string(cfg), "\n") - 5; strings.Count(got, "\n") != want || !strings.Contains(got, `AUTOUPDATE_EXCLUDE_CONTAINERS="nextcloud,mariadb"`) {
		t.Errorf("the other settings changed:\n%s", got)
	}
}

func TestTheProxyReachesTheStatusContract(t *testing.T) {
	proxy := read(t, "server/status.php")
	for _, path := range []string{"'fortress-status' => '/api/fortress/v1/status'", "'fortress-log' => '/api/fortress/v1/log'"} {
		if !strings.Contains(proxy, path) {
			t.Errorf("status.php does not map %s", path)
		}
	}
}

// TestTheNotificationsTabSaysWhenTollRollSendsThem keeps the hint about the
// hub on the page, hidden until the engine says TollRoll takes the messages,
// and in every language.
func TestTheNotificationsTabSaysWhenTollRollSendsThem(t *testing.T) {
	page := read(t, "shiplog.page")
	if !strings.Contains(page, `<p class="sl-hubnote" id="sl-hub-note" role="note" hidden>`) || !strings.Contains(page, "$L['s_notify_hub']") {
		t.Error("shiplog.page lacks the hidden hub note")
	}
	if !strings.Contains(read(t, filepath.Join("server", "status.php")), `'notify-hub' => '/api/notify-hub'`) {
		t.Error("status.php does not pass the hub's state on")
	}
	js := read(t, filepath.Join("scripts", "settings.js"))
	if !strings.Contains(js, `status.php?path=notify-hub`) || !strings.Contains(js, `byId("sl-hub-note").hidden = !(d && d.hub === true)`) {
		t.Error("settings.js does not show the note from the engine's answer")
	}
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]any
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if v, _ := texts["s_notify_hub"].(string); !strings.Contains(v, "TollRoll") {
			t.Errorf("%s: s_notify_hub = %q", filepath.Base(f), v)
		}
	}
}
