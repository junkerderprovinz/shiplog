// Package config loads the engine configuration from the environment.
package config

import (
	"os"
	"time"
)

// Config holds all runtime settings (env-driven; the Unraid template surfaces
// every field).
type Config struct {
	DockerSocket string        // DOCKER_SOCKET
	Port         string        // PORT
	DataDir      string        // DATA_DIR (SQLite + curated-mapping override)
	PollInterval time.Duration // POLL_INTERVAL
	GithubToken  string        // GITHUB_TOKEN (optional; raises API limit)

	// P1 (parsed now, unused until the providers land):
	OllamaURL        string // OLLAMA_URL
	OllamaModel      string // OLLAMA_MODEL
	MatrixHomeserver string // MATRIX_HOMESERVER
	MatrixToken      string // MATRIX_TOKEN
	MatrixRoom       string // MATRIX_ROOM
}

// Load reads the environment, applying the documented defaults.
func Load() Config {
	return Config{
		DockerSocket:     env("DOCKER_SOCKET", "/var/run/docker.sock"),
		Port:             env("PORT", "8484"),
		DataDir:          env("DATA_DIR", "/config"),
		PollInterval:     dur("POLL_INTERVAL", 6*time.Hour),
		GithubToken:      os.Getenv("GITHUB_TOKEN"),
		OllamaURL:        os.Getenv("OLLAMA_URL"),
		OllamaModel:      os.Getenv("OLLAMA_MODEL"),
		MatrixHomeserver: os.Getenv("MATRIX_HOMESERVER"),
		MatrixToken:      os.Getenv("MATRIX_TOKEN"),
		MatrixRoom:       os.Getenv("MATRIX_ROOM"),
	}
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
