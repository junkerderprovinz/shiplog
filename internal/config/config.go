// Package config loads the engine configuration from the environment.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings, each read from the environment variable
// named beside it.
type Config struct {
	DockerSocket string        // DOCKER_SOCKET
	Port         string        // PORT
	DataDir      string        // DATA_DIR (SQLite + curated-mapping override)
	PollInterval time.Duration // POLL_INTERVAL
	GithubToken  string        // GITHUB_TOKEN (optional; raises GitHub API limit for changelogs)

	// IgnoreUnmanaged (IGNORE_UNMANAGED) skips containers that were not created
	// from an Unraid template, such as Compose stacks or plain `docker run`.
	IgnoreUnmanaged bool

	// Docker Hub credentials raise the anonymous rate limit for docker.io lookups.
	DockerHubUser  string // DOCKERHUB_USERNAME
	DockerHubToken string // DOCKERHUB_TOKEN (access token or password)

	OllamaURL        string // OLLAMA_URL
	OllamaModel      string // OLLAMA_MODEL
	MatrixHomeserver string // MATRIX_HOMESERVER
	MatrixToken      string // MATRIX_TOKEN
	MatrixRoom       string // MATRIX_ROOM

	// UnraidNotify (UNRAID_NOTIFY) also sends native Unraid notifications, which
	// fan out to every agent configured on the host.
	UnraidNotify bool

	// AutoUpdate holds the scheduled auto-update settings of the Unraid plugin.
	AutoUpdate AutoUpdateConfig
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
}

// Load reads the environment, applying the documented defaults.
func Load() Config {
	return Config{
		DockerSocket:     env("DOCKER_SOCKET", "/var/run/docker.sock"),
		Port:             env("PORT", "8484"),
		DataDir:          env("DATA_DIR", "/config"),
		PollInterval:     dur("POLL_INTERVAL", 6*time.Hour),
		GithubToken:      os.Getenv("GITHUB_TOKEN"),
		IgnoreUnmanaged:  truthy("IGNORE_UNMANAGED"),
		DockerHubUser:    os.Getenv("DOCKERHUB_USERNAME"),
		DockerHubToken:   os.Getenv("DOCKERHUB_TOKEN"),
		OllamaURL:        os.Getenv("OLLAMA_URL"),
		OllamaModel:      os.Getenv("OLLAMA_MODEL"),
		MatrixHomeserver: os.Getenv("MATRIX_HOMESERVER"),
		MatrixToken:      os.Getenv("MATRIX_TOKEN"),
		MatrixRoom:       os.Getenv("MATRIX_ROOM"),
		UnraidNotify:     truthy("UNRAID_NOTIFY"),
		AutoUpdate: AutoUpdateConfig{
			Enabled:      truthy("AUTOUPDATE_ENABLED"),
			Level:        levelOrOff(os.Getenv("AUTOUPDATE_LEVEL")),
			Digest:       truthy("AUTOUPDATE_DIGEST"),
			DryRun:       truthy("AUTOUPDATE_DRYRUN"),
			SchedMode:    env("AUTOUPDATE_SCHED_MODE", "off"),
			SchedTime:    env("AUTOUPDATE_SCHED_TIME", "04:00"),
			SchedEvery:   atoiMin1("AUTOUPDATE_SCHED_EVERY", 6),
			ExcludeWords: os.Getenv("AUTOUPDATE_EXCLUDE_WORDS"),
		},
	}
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
