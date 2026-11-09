package plugin

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/notify/legacy"
)

// runPHP runs body after including include/notify.php as $N and decodes what
// it echoes as JSON.
func runPHP(t *testing.T, body string, out any) {
	t.Helper()
	php, err := exec.LookPath("php")
	if err != nil {
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

var oldSettings = map[string]string{
	"MATRIX_HOMESERVER": "https://matrix.example.org",
	"MATRIX_TOKEN":      "syt_c2hpcGxvZw_NotARealToken_0a1b2c",
	"MATRIX_ROOM":       "#updates:example.org",
	"APPRISE_URL":       "http://apprise.lan:8000/notify/shiplog",
	"UNRAID_NOTIFY":     "true",
}

// TestThePageSavesWhatTheEngineMovesOnItsOwn keeps both ways to notify.json
// alike: the engine's first start and the settings page's first save.
func TestThePageSavesWhatTheEngineMovesOnItsOwn(t *testing.T) {
	var got []kit.Channel
	runPHP(t, `echo json_encode($N['merge']([], ['hs' => 'https://matrix.example.org', 'token' => 'syt_c2hpcGxvZw_NotARealToken_0a1b2c',
		'room' => '#updates:example.org', 'apprise' => 'http://apprise.lan:8000/notify/shiplog', 'unraid' => true]));`, &got)
	if want := legacy.FromEnv(oldSettings); !reflect.DeepEqual(got, want) {
		t.Errorf("page saved\n %+v\nengine moves\n %+v", got, want)
	}
}

func TestThePageKeepsChannelsItDoesNotShow(t *testing.T) {
	var got []kit.Channel
	runPHP(t, `$ch = [
		['id' => 'phone', 'kind' => 'ntfy', 'enabled' => true, 'on' => 'failure', 'ntfy' => ['url' => 'https://ntfy.example.org/updates']],
		['id' => 'matrix', 'kind' => 'matrix', 'name' => 'Room', 'enabled' => true, 'on' => 'failure',
		 'matrix' => ['homeserver' => 'https://hs', 'room' => '!r:hs', 'token' => 't']],
		['id' => 'unraid', 'kind' => 'unraid', 'enabled' => true, 'on' => 'always'],
	];
	echo json_encode($N['merge']($ch, ['hs' => '', 'token' => '', 'room' => '', 'apprise' => 'apprise.lan/notify/x', 'unraid' => false]));`, &got)
	want := []kit.Channel{
		{ID: "phone", Kind: kit.KindNtfy, Enabled: true, On: kit.OnFailure, Ntfy: &kit.Ntfy{URL: "https://ntfy.example.org/updates"}},
		{ID: "unraid", Kind: kit.KindUnraid, Enabled: false, On: kit.OnAlways},
		{ID: "apprise", Kind: kit.KindApprise, Enabled: false, On: kit.OnAlways, Apprise: &kit.Apprise{URL: "apprise.lan/notify/x"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestThePageShowsTheSavedChannels(t *testing.T) {
	var fromCfg, fromFile map[string]any
	runPHP(t, `echo json_encode($N['fields'](null, ['MATRIX_HOMESERVER' => 'https://hs', 'MATRIX_TOKEN' => 't', 'MATRIX_ROOM' => '!r:hs',
		'APPRISE_URL' => 'http://a/notify/k', 'UNRAID_NOTIFY' => 'true']));`, &fromCfg)
	runPHP(t, `echo json_encode($N['fields']($N['merge']([], ['hs' => 'https://hs', 'token' => 't', 'room' => '!r:hs',
		'apprise' => 'http://a/notify/k', 'unraid' => true]), []));`, &fromFile)
	want := map[string]any{"hs": "https://hs", "token": "t", "room": "!r:hs", "apprise": "http://a/notify/k", "unraid": true}
	if !reflect.DeepEqual(fromCfg, want) || !reflect.DeepEqual(fromFile, want) {
		t.Errorf("from shiplog.cfg %v, from notify.json %v, want %v", fromCfg, fromFile, want)
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
