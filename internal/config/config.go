// Package config loads the engine configuration from the environment.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings, each read from the environment variable
// named beside it.
type Config struct {
	DockerSocket string // DOCKER_SOCKET
	// DockerConfigDir (DOCKER_CONFIG, else $HOME/.docker) holds the config.json
	// whose registry logins the resolver reuses.
	DockerConfigDir string
	Port            string        // PORT
	Socket          string        // SOCKET, a unix socket the API listens on instead of PORT
	DataDir         string        // DATA_DIR (SQLite + curated-mapping override)
	PollInterval    time.Duration // POLL_INTERVAL
	GithubToken     string        // GITHUB_TOKEN (optional; raises GitHub API limit for changelogs)

	// IgnoreUnmanaged (IGNORE_UNMANAGED) skips containers that were not created
	// from an Unraid template, such as Compose stacks or plain `docker run`.
	IgnoreUnmanaged bool

	// Docker Hub credentials raise the anonymous rate limit for docker.io lookups.
	DockerHubUser  string // DOCKERHUB_USERNAME
	DockerHubToken string // DOCKERHUB_TOKEN (access token or password)

	OllamaURL      string // OLLAMA_URL
	OllamaModel    string // OLLAMA_MODEL
	LlamaSwapURL   string // LLAMASWAP_URL, used instead of Ollama when set
	LlamaSwapModel string // LLAMASWAP_MODEL

	// NotifyFile (NOTIFY_FILE) holds the notification channels. Without it,
	// as in the container, OldNotify sets them up on every start.
	NotifyFile string
	// OldNotify holds the variables notifications were set up with before
	// NotifyFile, which the plugin reads once to build it.
	OldNotify map[string]string

	// AutoUpdate holds the scheduled auto-update settings of the Unraid plugin.
	AutoUpdate AutoUpdateConfig

	// FortressDir (FORTRESS_DIR) keeps ShipLog's membership in a fortress, as
	// the kit's fortress join writes it. Empty or without a membership, ShipLog
	// is in no fortress.
	FortressDir string
	// RunsFile (RUNS_FILE, else runs.json in DataDir) keeps the update runs
	// other apps asked for, so they get their answer after a restart.
	RunsFile string
}

// AutoUpdateConfig configures scheduled auto-updates (all AUTOUPDATE_* env vars).
type AutoUpdateConfig struct {
	Enabled    bool   // AUTOUPDATE_ENABLED ("yes"/"no")
	Level      string // AUTOUPDATE_LEVEL: off|patch|minor|major
	Digest     bool   // AUTOUPDATE_DIGEST: also auto-apply :latest/digest moves
	DryRun     bool   // AUTOUPDATE_DRYRUN: notify only, apply nothing
	SchedMode  string // AUTOUPDATE_SCHED_MODE: off|daily|boot|hours|days
	SchedTime  string // AUTOUPDATE_SCHED_TIME "HH:MM" (daily)
	SchedEvery int    // AUTOUPDATE_SCHED_EVERY (hours|days), >= 1
	// ExcludeWords (AUTOUPDATE_EXCLUDE_WORDS, comma-separated) blocks an eligible
	// update whose changelog contains one of the words, so "breaking" can hold
	// back a patch release that calls out a breaking change.
	ExcludeWords string
	// ExcludeContainers (AUTOUPDATE_EXCLUDE_CONTAINERS, comma-separated names)
	// are never auto-updated; the admin updates them by hand.
	ExcludeContainers string
	// BackupTimeout (AUTOUPDATE_BACKUP_MINUTES) is how long an update waits for
	// the fortress's backup app to back the container up.
	BackupTimeout time.Duration
	// WithoutBackup (AUTOUPDATE_WITHOUT_BACKUP, comma-separated names) update
	// even when their backup fails.
	WithoutBackup string
}

// Load reads the environment, applying the documented defaults.
func Load() Config {
	c := Config{
		DockerSocket:    env("DOCKER_SOCKET", "/var/run/docker.sock"),
		DockerConfigDir: dockerConfigDir(),
		Port:            env("PORT", "8484"),
		Socket:          os.Getenv("SOCKET"),
		DataDir:         env("DATA_DIR", "/config"),
		PollInterval:    dur("POLL_INTERVAL", 6*time.Hour),
		GithubToken:     os.Getenv("GITHUB_TOKEN"),
		IgnoreUnmanaged: truthy("IGNORE_UNMANAGED"),
		DockerHubUser:   os.Getenv("DOCKERHUB_USERNAME"),
		DockerHubToken:  os.Getenv("DOCKERHUB_TOKEN"),
		OllamaURL:       os.Getenv("OLLAMA_URL"),
		OllamaModel:     os.Getenv("OLLAMA_MODEL"),
		LlamaSwapURL:    os.Getenv("LLAMASWAP_URL"),
		LlamaSwapModel:  os.Getenv("LLAMASWAP_MODEL"),
		NotifyFile:      os.Getenv("NOTIFY_FILE"),
		FortressDir:     os.Getenv("FORTRESS_DIR"),
		RunsFile:        os.Getenv("RUNS_FILE"),
		OldNotify:       oldNotify(),
		AutoUpdate: AutoUpdateConfig{
			Enabled:           truthy("AUTOUPDATE_ENABLED"),
			Level:             levelOrOff(os.Getenv("AUTOUPDATE_LEVEL")),
			Digest:            truthy("AUTOUPDATE_DIGEST"),
			DryRun:            truthy("AUTOUPDATE_DRYRUN"),
			SchedMode:         env("AUTOUPDATE_SCHED_MODE", "off"),
			SchedTime:         env("AUTOUPDATE_SCHED_TIME", "04:00"),
			SchedEvery:        atoiMin1("AUTOUPDATE_SCHED_EVERY", 6),
			ExcludeWords:      os.Getenv("AUTOUPDATE_EXCLUDE_WORDS"),
			ExcludeContainers: os.Getenv("AUTOUPDATE_EXCLUDE_CONTAINERS"),
			BackupTimeout:     time.Duration(atoiMin1("AUTOUPDATE_BACKUP_MINUTES", 30)) * time.Minute,
			WithoutBackup:     os.Getenv("AUTOUPDATE_WITHOUT_BACKUP"),
		},
	}
	if c.RunsFile == "" {
		c.RunsFile = filepath.Join(c.DataDir, "runs.json")
	}
	return c
}

func oldNotify() map[string]string {
	m := map[string]string{}
	for _, k := range []string{"MATRIX_HOMESERVER", "MATRIX_TOKEN", "MATRIX_ROOM", "APPRISE_URL", "UNRAID_NOTIFY"} {
		m[k] = os.Getenv(k)
	}
	return m
}

// truthy accepts the settings page's "true" as well as "yes", "1" and "on".
func truthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "yes", "1", "on":
		return true
	}
	return false
}

func levelOrOff(s string) string {
	switch s {
	case "patch", "minor", "major":
		return s
	default:
		return "off"
	}
}

// atoiMin1 falls back to def when the value is missing, invalid or below 1.
func atoiMin1(key string, def int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil || n < 1 {
		return def
	}
	return n
}

// dockerConfigDir finds the directory the way the Docker CLI does. The daemon
// runs as root, so an unset HOME means /root.
func dockerConfigDir() string {
	if d := os.Getenv("DOCKER_CONFIG"); d != "" {
		return d
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".docker")
	}
	return "/root/.docker"
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// dur parses a Go duration (e.g. "6h", "90m"); falls back to def on empty/invalid.
func dur(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
