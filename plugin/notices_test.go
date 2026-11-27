package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var noticeKeys = []string{
	"s_notices", "s_notices_sub", "s_notify_batch", "s_notify_batch_desc", "s_notify_min_risk",
	"s_notify_min_risk_desc", "s_nmr_all", "s_nmr_medium", "s_nmr_high", "s_nmr_critical",
}

func TestEveryLanguageNamesTheUpdateNoticeSettings(t *testing.T) {
	page := read(t, "shiplog.page")
	for _, k := range noticeKeys {
		if !strings.Contains(page, "$L['"+k+"']") {
			t.Errorf("shiplog.page does not show %s", k)
		}
	}
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(b, &texts); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		for _, k := range noticeKeys {
			if strings.TrimSpace(texts[k]) == "" {
				t.Errorf("%s: %s is missing", filepath.Base(f), k)
			}
		}
	}
}

// Without a saved setting every update gets its own message, as before.
func TestTheUpdateNoticesStayAsTheyWereUntilChosen(t *testing.T) {
	page := read(t, "shiplog.page")
	for _, want := range []string{
		`$notifyBatch = (isset($cfg['NOTIFY_BATCH']) && $cfg['NOTIFY_BATCH'] === 'true');`,
		`$slSwitch('sl-notify-batch', 'NOTIFY_BATCH', $notifyBatch)`,
		`name="NOTIFY_MIN_RISK"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("shiplog.page lacks %s", want)
		}
	}
	rc := read(t, filepath.Join("scripts", "rc.shiplog"))
	for _, want := range []string{"\nNOTIFY_BATCH=\"false\"\n", "\nNOTIFY_MIN_RISK=\"\"\n", " NOTIFY_BATCH NOTIFY_MIN_RISK "} {
		if !strings.Contains(rc, want) {
			t.Errorf("rc.shiplog lacks %q", want)
		}
	}
}
