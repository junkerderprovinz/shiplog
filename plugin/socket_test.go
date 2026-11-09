package plugin

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestEngineAndProxyShareTheSocket keeps the engine off the network: the rc
// script starts it on a unix socket, the proxy dials that socket and the
// removal script cleans it up.
func TestEngineAndProxyShareTheSocket(t *testing.T) {
	rc := read(t, "scripts/rc.shiplog")
	m := regexp.MustCompile(`(?m)^SOCKET="([^"]+)"`).FindStringSubmatch(rc)
	if m == nil {
		t.Fatal("rc.shiplog sets no SOCKET for the engine")
	}
	sock := m[1]
	if !regexp.MustCompile(`(?m)^\s*export\b[^\n]*(\\\n[^\n]*)*\bSOCKET\b`).MatchString(rc) {
		t.Error("rc.shiplog does not export SOCKET to the engine")
	}

	proxy := read(t, "server/status.php")
	if !strings.Contains(proxy, "CURLOPT_UNIX_SOCKET_PATH") || !strings.Contains(proxy, "'"+sock+"'") {
		t.Errorf("status.php does not dial %s", sock)
	}
	if strings.Contains(proxy, "127.0.0.1") {
		t.Error("status.php still reaches the engine over TCP")
	}

	plg, err := os.ReadFile("shiplog.plg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plg), strings.Replace(sock, "shiplog", "&slug;", 1)) {
		t.Errorf("shiplog.plg does not remove %s", sock)
	}
}

func TestStatusPageOpensThroughTheProxy(t *testing.T) {
	page := read(t, "shiplog.page")
	if !strings.Contains(page, `href="/plugins/shiplog/server/status.php?path=page"`) {
		t.Error("the settings page does not open the status page through the proxy")
	}
	for _, src := range []string{page, read(t, "scripts/settings.js")} {
		if strings.Contains(src, "data-sl-port") || strings.Contains(src, "SL_STATUS_HOST") {
			t.Error("the settings page still points at the engine's port")
		}
	}
}
