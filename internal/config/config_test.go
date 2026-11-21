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

func TestTheVulnerabilityCheckIsOnUntilSwitchedOff(t *testing.T) {
	for v, want := range map[string]bool{"": true, "true": true, "yes": true, "false": false, "no": false, "0": false} {
		t.Setenv("VULN_CHECK", v)
		if got := Load().VulnCheck; got != want {
			t.Errorf("VULN_CHECK=%q: check %v, want %v", v, got, want)
		}
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

func TestLoadFindsTheNotificationChannels(t *testing.T) {
	t.Setenv("NOTIFY_FILE", "")
	if c := Load(); c.NotifyFile != "" {
		t.Errorf("NotifyFile = %q, want none in the container", c.NotifyFile)
	}
	t.Setenv("NOTIFY_FILE", "/boot/config/plugins/shiplog/notify.json")
	if c := Load(); c.NotifyFile != "/boot/config/plugins/shiplog/notify.json" {
		t.Errorf("NotifyFile = %q", c.NotifyFile)
	}
}

func TestLoadKeepsTheOldNotificationSettingsForTheMove(t *testing.T) {
	t.Setenv("MATRIX_ROOM", "#updates:example.org")
	t.Setenv("UNRAID_NOTIFY", "true")
	t.Setenv("APPRISE_URL", "")
	c := Load()
	if c.OldNotify["MATRIX_ROOM"] != "#updates:example.org" || c.OldNotify["UNRAID_NOTIFY"] != "true" || c.OldNotify["APPRISE_URL"] != "" {
		t.Errorf("OldNotify = %v", c.OldNotify)
	}
}

func TestTheSummariesStayLocalUnlessOpenRouterIsChosen(t *testing.T) {
	for v, want := range map[string]string{"": "local", "local": "local", "bogus": "local", "openrouter": "openrouter", " OpenRouter ": "openrouter"} {
		t.Setenv("AI_PROVIDER", v)
		if got := Load().AIProvider; got != want {
			t.Errorf("AI_PROVIDER=%q: provider %q, want %q", v, got, want)
		}
	}
}

func TestLoadFindsTheOpenRouterModelAndKeyFile(t *testing.T) {
	t.Setenv("OPENROUTER_MODEL", "z-ai/glm-4.6")
	t.Setenv("OPENROUTER_KEY_FILE", "/boot/config/plugins/shiplog/openrouter.key")
	c := Load()
	if c.OpenRouterModel != "z-ai/glm-4.6" || c.OpenRouterKeyFile != "/boot/config/plugins/shiplog/openrouter.key" {
		t.Errorf("OpenRouter settings = %q %q", c.OpenRouterModel, c.OpenRouterKeyFile)
	}
}
