// Package config loads the engine configuration from the environment.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings (env-driven; the Unraid template surfaces
// every field).
type Config struct {
	DockerSocket string        // DOCKER_SOCKET
	Port         string        // PORT
	DataDir      string        // DATA_DIR (SQLite + curated-mapping override)
	PollInterval time.Duration // POLL_INTERVAL
	GithubToken  string        // GITHUB_TOKEN (optional; raises GitHub API limit for changelogs)

	// Docker Hub credentials (optional; raise the anonymous Docker Hub pull/manifest
	// rate limit used when resolving versions + digests for docker.io images).
	DockerHubUser  string // DOCKERHUB_USERNAME
	DockerHubToken string // DOCKERHUB_TOKEN (a Docker Hub access token / password)

	// P1 (parsed now, unused until the providers land):
	OllamaURL        string // OLLAMA_URL
	OllamaModel      string // OLLAMA_MODEL
	MatrixHomeserver string // MATRIX_HOMESERVER
	MatrixToken      string // MATRIX_TOKEN
	MatrixRoom       string // MATRIX_ROOM

	// AutoUpdate holds the scheduled SemVer-gated auto-update settings (Unraid
	// plugin only; default off). See internal/autoupdate.
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
}

// Load reads the environment, applying the documented defaults.
func Load() Config {
	return Config{
		DockerSocket:     env("DOCKER_SOCKET", "/var/run/docker.sock"),
		Port:             env("PORT", "8484"),
		DataDir:          env("DATA_DIR", "/config"),
		PollInterval:     dur("POLL_INTERVAL", 6*time.Hour),
		GithubToken:      os.Getenv("GITHUB_TOKEN"),
		DockerHubUser:    os.Getenv("DOCKERHUB_USERNAME"),
		DockerHubToken:   os.Getenv("DOCKERHUB_TOKEN"),
		OllamaURL:        os.Getenv("OLLAMA_URL"),
		OllamaModel:      os.Getenv("OLLAMA_MODEL"),
		MatrixHomeserver: os.Getenv("MATRIX_HOMESERVER"),
		MatrixToken:      os.Getenv("MATRIX_TOKEN"),
		MatrixRoom:       os.Getenv("MATRIX_ROOM"),
		AutoUpdate: AutoUpdateConfig{
			Enabled:    yes("AUTOUPDATE_ENABLED"),
			Level:      levelOrOff(os.Getenv("AUTOUPDATE_LEVEL")),
			Digest:     yes("AUTOUPDATE_DIGEST"),
			DryRun:     yes("AUTOUPDATE_DRYRUN"),
			SchedMode:  env("AUTOUPDATE_SCHED_MODE", "off"),
			SchedTime:  env("AUTOUPDATE_SCHED_TIME", "04:00"),
			SchedEvery: atoiMin1("AUTOUPDATE_SCHED_EVERY", 6),
		},
	}
}

// yes reports whether the env var equals "yes" (case-insensitive).
func yes(key string) bool { return strings.EqualFold(os.Getenv(key), "yes") }

// levelOrOff validates an auto-update level, defaulting anything else to "off".
func levelOrOff(s string) string {
	switch s {
	case "patch", "minor", "major":
		return s
	default:
		return "off"
	}
}

// atoiMin1 parses an int env var, clamped to >= 1, falling back to def.
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
