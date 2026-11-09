package config

import (
	"path/filepath"
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

func TestLoadSocket(t *testing.T) {
	t.Setenv("SOCKET", "")
	if got := Load().Socket; got != "" {
		t.Fatalf("SOCKET unset: got %q, want TCP", got)
	}
	t.Setenv("SOCKET", "/var/run/shiplog.sock")
	if got := Load().Socket; got != "/var/run/shiplog.sock" {
		t.Fatalf("SOCKET: got %q", got)
	}
}

func TestBadDurationFallsBack(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "not-a-duration")
	if Load().PollInterval != 6*time.Hour {
		t.Fatal("invalid POLL_INTERVAL should fall back to 6h")
	}
}

func TestLoadDockerConfigDir(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", "/custom/dockercfg")
	if got := Load().DockerConfigDir; got != "/custom/dockercfg" {
		t.Fatalf("DOCKER_CONFIG: got %q", got)
	}
	t.Setenv("DOCKER_CONFIG", "")
	t.Setenv("HOME", "/home/someone")
	if got, want := Load().DockerConfigDir, filepath.Join("/home/someone", ".docker"); got != want {
		t.Fatalf("with HOME set: got %q, want %q", got, want)
	}
}
