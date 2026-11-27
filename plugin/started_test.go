package plugin

import (
	"regexp"
	"strings"
	"testing"
)

// TestTheStartedEventLeavesTheWaitToTheRcScript keeps the check for the data
// folder in one place.
func TestTheStartedEventLeavesTheWaitToTheRcScript(t *testing.T) {
	ev := read(t, "event/started")
	if !regexp.MustCompile(`(?m)^/usr/local/emhttp/plugins/shiplog/scripts/rc\.shiplog started$`).MatchString(ev) {
		t.Error("event/started does not hand the start to rc.shiplog started")
	}
	if strings.Contains(ev, "stat ") || strings.Contains(ev, "DATA_DIR=") || strings.Contains(ev, "/dev/null") {
		t.Error("event/started still probes the data folder itself")
	}
	if strings.Contains(ev, "shfs") {
		t.Error("event/started still says shfs may be mounting, which Unraid finishes before started")
	}
}

// TestTheRcScriptProbesTheDataFolderOnce shares the probe between a start and
// the wait after array start.
func TestTheRcScriptProbesTheDataFolderOnce(t *testing.T) {
	rc := read(t, "scripts/rc.shiplog")
	if n := strings.Count(rc, `stat -c %d "$probe"`); n != 1 {
		t.Errorf("rc.shiplog probes the data folder %d times", n)
	}
	if !regexp.MustCompile(`(?m)^\s*started\)\s+start_when_mounted\s*;;`).MatchString(rc) {
		t.Error("rc.shiplog has no started command that waits for the data folder")
	}
}

// TestTheWaitForTheDataFolderLogsAndEndsWithTheArray leaves a trace of a start
// that waits and gives up once the array stops.
func TestTheWaitForTheDataFolderLogsAndEndsWithTheArray(t *testing.T) {
	rc := read(t, "scripts/rc.shiplog")
	body := regexp.MustCompile(`(?s)start_when_mounted\(\) \{.*?\n\}`).FindString(rc)
	if body == "" {
		t.Fatal("rc.shiplog has no start_when_mounted")
	}
	if strings.Contains(body, "/dev/null 2>&1 &") || !strings.Contains(body, `>>"$LOG"`) {
		t.Error("the wait does not write to the daemon log")
	}
	if !strings.Contains(rc, "/var/local/emhttp/var.ini") || !strings.Contains(rc, `mdState="STARTED"`) {
		t.Error("rc.shiplog does not read the array state from var.ini")
	}
	if !strings.Contains(body, "array_started") {
		t.Error("the wait does not end when the array stops")
	}
}
