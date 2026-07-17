// Package model holds the types shared across every ShipLog engine unit.
package model

import "time"

// Container is a single container as discovered on the host.
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`  // image ref as referenced, e.g. "ghcr.io/x/y:1.2.3" or "redis"
	Repo   string `json:"repo"`   // normalized, e.g. "ghcr.io/x/y" or "docker.io/library/redis"; "" for a bare image-ID ref
	Tag    string `json:"tag"`    // "1.2.3" / "latest"; "" for digest-pinned or image-ID refs
	Digest string `json:"digest"` // running image digest, "sha256:..."
	// Digests lists every manifest digest from the image's RepoDigests. An image
	// pulled through a mirror and retagged carries one entry per registry; a
	// remote digest matching ANY of them means "same image".
	Digests []string `json:"digests,omitempty"`
	// PinnedDigest is the "sha256:…" digest when the container references its
	// image digest-pinned ("repo@sha256:…"). Such a container updates only when
	// the pin is changed, never by a registry move.
	PinnedDigest string `json:"pinned_digest,omitempty"`
	// IsLocal marks an image without any registry counterpart (built locally,
	// never pushed): RepoDigests was empty on a successful inspect. Resolving it
	// upstream would at best waste a roundtrip and at worst match a FOREIGN
	// repository that happens to share the name.
	IsLocal bool   `json:"is_local,omitempty"`
	Source  string `json:"source"` // org.opencontainers.image.source label, may be ""
	State   string `json:"state"`  // "running" / "exited" / ...
	// ImageVersion is the version the running image declares about itself via the
	// OCI label org.opencontainers.image.version (fallback .revision), read from
	// the image config. "" when the image carries no such label. It lets a
	// rolling (":latest") container report its version immediately, without
	// waiting for ShipLog to first witness an update.
	ImageVersion string `json:"image_version"`
}

// RiskLevel is the severity verdict for an available update.
type RiskLevel string

const (
	RiskNone    RiskLevel = "none"
	RiskLow     RiskLevel = "low"
	RiskMedium  RiskLevel = "medium"
	RiskHigh    RiskLevel = "high"
	RiskUnknown RiskLevel = "unknown"
)

var riskRank = map[RiskLevel]int{RiskNone: 0, RiskUnknown: 1, RiskLow: 2, RiskMedium: 3, RiskHigh: 4}

// MoreSevere reports whether r outranks o (unknown never outranks a real level).
func (r RiskLevel) MoreSevere(o RiskLevel) bool { return riskRank[r] > riskRank[o] }

// Kind classifies the version delta between the running and newest image.
type Kind string

const (
	KindNone    Kind = "none"
	KindDigest  Kind = "digest" // same tag, new digest (e.g. :latest moved)
	KindPatch   Kind = "patch"
	KindMinor   Kind = "minor"
	KindMajor   Kind = "major"
	KindUnknown Kind = "unknown" // non-semver tag, can't classify
)

// UpdateStatus is the per-container result the engine stores and serves.
type UpdateStatus struct {
	Container Container `json:"container"`
	// RunningVersion is the version label we believe is currently running. For a
	// pinned tag it's the tag itself; for a rolling tag (":latest") it's resolved
	// when the running image is the registry's current one, then REMEMBERED across
	// sweeps so a later update shows a real "prev -> new" jump.
	RunningVersion string     `json:"running_version"`
	NewestTag      string     `json:"newest_tag"`
	NewestDigest   string     `json:"newest_digest"`
	Kind           Kind       `json:"kind"`
	Risk           RiskLevel  `json:"risk"`
	RiskReason     string     `json:"risk_reason"`
	Changelog      *Changelog `json:"changelog,omitempty"`
	CheckedAt      time.Time  `json:"checked_at"`
	Error          string     `json:"error,omitempty"` // per-container failure, never fatal
}

// HasDigest reports whether d equals the container's primary digest or any of
// its RepoDigests entries (mirror pulls carry one digest per registry).
func (c Container) HasDigest(d string) bool {
	if d == "" {
		return false
	}
	if d == c.Digest {
		return true
	}
	for _, have := range c.Digests {
		if have == d {
			return true
		}
	}
	return false
}

// HasUpdate reports whether this status represents an actionable update.
func (s UpdateStatus) HasUpdate() bool { return s.Kind != KindNone && s.Kind != "" }

// Changelog is the resolved "what changed" payload for an update.
type Changelog struct {
	FromTag      string         `json:"from_tag"`
	ToTag        string         `json:"to_tag"`
	SkippedCount int            `json:"skipped_count"`
	Entries      []ReleaseEntry `json:"entries"` // newest first
	Raw          string         `json:"raw"`
	Summary      *AISummary     `json:"summary,omitempty"` // P1 (Ollama)
	Source       string         `json:"source"`            // human label, e.g. "GitHub releases via OCI label"
	URL          string         `json:"url"`               // releases/compare link
	Provider     string         `json:"provider"`          // "github" / "fallback" / ...
	Deprecated   bool           `json:"deprecated"`        // upstream repo is archived (EOL)
	// RateLimited is set when the changelog could not be resolved because the
	// upstream API (GitHub's anonymous 60 req/h, shared per IP) was exhausted.
	// The UI shows an honest "rate limited, try later" note instead of a blank.
	RateLimited bool `json:"rate_limited"`
	// Recent marks that Entries are the repo's latest releases (a digest/rolling
	// update, or no release matched toTag exactly), not an exact match for toTag.
	Recent bool `json:"recent"`
}

// ReleaseEntry is a single upstream release in a changelog span.
type ReleaseEntry struct {
	Tag         string    `json:"tag"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// AISummary is populated only when Ollama is configured (P1).
type AISummary struct {
	Bullets  []string `json:"bullets"`
	Breaking []string `json:"breaking"`
	Risk     string   `json:"risk"`
	Model    string   `json:"model"`
}
