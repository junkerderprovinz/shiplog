package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNoop(t *testing.T) {
	var u Updater = Noop{}
	if u.Supported() {
		t.Error("Noop is never supported")
	}
	if _, err := u.Update(context.Background(), "x"); !errors.Is(err, ErrUnsupported) {
		t.Error("Noop.Update must return ErrUnsupported")
	}
}

func TestUnraidOffBox(t *testing.T) {
	if (Unraid{}).Supported() {
		t.Skip("host has the Unraid template dir; off-box assertion skipped")
	}
	if _, err := (Unraid{}).Update(context.Background(), "plex"); err == nil {
		t.Error("Unraid.Update must error off-box (no template)")
	}
}

// TestHelperScript stands in for the update script when the test binary runs
// itself, and does nothing in a normal test run.
func TestHelperScript(t *testing.T) {
	mode := os.Getenv("SHIPLOG_HELPER_SCRIPT")
	if mode == "" {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, strings.Repeat("pulling layer\n", 1000))
	_, _ = fmt.Fprint(os.Stderr, "docker: Error response from daemon: Conflict. The container name \"/Immich\" is already in use.")
	if mode == "fail" {
		os.Exit(3)
	}
	os.Exit(0)
}

func runHelper(t *testing.T, mode string) (string, error) {
	t.Setenv("SHIPLOG_HELPER_SCRIPT", mode)
	return run(context.Background(), os.Args[0], "-test.run=^TestHelperScript$")
}

func TestRunKeepsTheEndOfTheScriptOutput(t *testing.T) {
	out, err := runHelper(t, "pass")
	if err != nil {
		t.Fatalf("run = %v, want no error for exit 0", err)
	}
	if !strings.HasSuffix(out, `The container name "/Immich" is already in use.`) {
		t.Errorf("output lost what the script printed last: ...%s", out[max(0, len(out)-80):])
	}
	if !strings.HasPrefix(out, "…") || len(out) > outputLimit+len("…") {
		t.Errorf("output of %d bytes, want the last %d bytes marked as cut", len(out), outputLimit)
	}
}

func TestRunReportsAFailedScriptWithItsOutput(t *testing.T) {
	out, err := runHelper(t, "fail")
	if err == nil {
		t.Fatal("run = nil, want the exit status")
	}
	if !strings.Contains(out, "already in use") {
		t.Errorf("output of a failed script lost its last line: ...%s", out[max(0, len(out)-80):])
	}
}
