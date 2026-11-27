package main

import (
	"testing"
	"time"
)

func TestWaitForReturnsOnceTheEngineStops(t *testing.T) {
	done := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(done)
	}()
	start := time.Now()
	if !waitFor(done, 5*time.Second) {
		t.Fatal("waitFor gave up although the engine stopped")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("waitFor took %v for an engine that stopped at once", time.Since(start))
	}
}

func TestWaitForGivesUpAfterItsLimit(t *testing.T) {
	start := time.Now()
	if waitFor(make(chan struct{}), 50*time.Millisecond) {
		t.Fatal("waitFor reported a stop that never came")
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Errorf("waitFor gave up after %v", time.Since(start))
	}
}

// The rc script kills the process ten seconds after it asked it to stop, so
// the engine has to be done before that.
func TestTheEngineGraceEndsBeforeTheRcScriptKills(t *testing.T) {
	if engineGrace <= 0 || engineGrace >= 10*time.Second {
		t.Errorf("engineGrace = %v, want under the ten seconds rc.shiplog waits", engineGrace)
	}
}
