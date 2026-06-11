package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DOCKER_SOCKET", "")
	t.Setenv("PORT", "")
	t.Setenv("POLL_INTERVAL", "")
	c := Load()
	if c.DockerSocket != "/var/run/docker.sock" || c.Port != "8484" || c.PollInterval != 6*time.Hour {
		t.Fatalf("defaults wrong: %+v", c)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("POLL_INTERVAL", "90m")
	t.Setenv("POLL_INTERVAL_BAD", "")
	c := Load()
	if c.Port != "9000" || c.PollInterval != 90*time.Minute {
		t.Fatalf("overrides wrong: %+v", c)
	}
}

func TestBadDurationFallsBack(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "not-a-duration")
	if Load().PollInterval != 6*time.Hour {
		t.Fatal("invalid POLL_INTERVAL should fall back to 6h")
	}
}
