// Package model holds the types shared across every ShipLog engine unit.
package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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
	// remote digest matching any of them means "same image".
	Digests []string `json:"digests,omitempty"`
	// PinnedDigest is the "sha256:…" digest when the container references its
	// image digest-pinned ("repo@sha256:…"). Such a container updates only when
	// the pin is changed, never by a registry move.
	PinnedDigest string `json:"pinned_digest,omitempty"`
	// IsLocal marks an image that was built locally and never pushed. Resolving
	// it upstream could match an unrelated repository that shares the name.
	IsLocal bool   `json:"is_local,omitempty"`
	Source  string `json:"source"` // org.opencontainers.image.source label, may be ""
	State   string `json:"state"`  // "running" / "exited" / ...
	// Health is what the image's healthcheck says: "starting", "healthy",
	// "unhealthy", or "" for an image without one.
	Health  string `json:"health,omitempty"`
	ImageID string `json:"image_id,omitempty"`
	// Managed is true when the container carries Unraid's net.unraid.docker.managed
	// label, i.e. it was created from an Unraid template.
	Managed bool `json:"managed"`
	// ImageVersion is the image's org.opencontainers.image.version label (or
	// .revision), so a rolling ":latest" container can show its version before
	// ShipLog has seen it update.
	ImageVersion string `json:"image_version"`
	// BaseVersion is a version label the image inherited from its base image,
	// such as Ubuntu's "24.04". It never counts as the app's version.
	BaseVersion string `json:"base_version,omitempty"`
	// ChangelogFile is the URL of a changelog file the user set as the source,
	// read instead of the repo's releases.
	ChangelogFile string `json:"changelog_file,omitempty"`
}

// RiskLevel is the severity verdict for an available update.
type RiskLevel string

const (
	RiskNone     RiskLevel = "none"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical" // release notes flag a breaking change / required migration
	RiskUnknown  RiskLevel = "unknown"
)

var riskRank = map[RiskLevel]int{RiskNone: 0, RiskUnknown: 1, RiskLow: 2, RiskMedium: 3, RiskHigh: 4, RiskCritical: 5}

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

// Reason says why a status has its risk.
type Reason struct {
	Code string   `json:"code"`
	Args []string `json:"args,omitempty"`
}

const (
	ReasonDigest       = "digest"
	ReasonCurrent      = "current"
	ReasonNonSemver    = "non_semver"
	ReasonMajor        = "major"
	ReasonMinor        = "minor"
	ReasonPatch        = "patch"
	ReasonPinnedNewer  = "pinned_newer"
	ReasonBreakingIn   = "breaking_in"
	ReasonBreaking     = "breaking"
	ReasonDigestPinned = "digest_pinned"
	ReasonLocal        = "local"
	ReasonImageID      = "image_id"
	ReasonRemoved      = "removed"
	ReasonLookupFailed = "lookup_failed"
)

// ReasonCodes holds the English text of each code, with %1 and %2 for its
// values. The Docker tab's language files carry the same texts.
var ReasonCodes = map[string]string{
	ReasonDigest:       "same tag, new image digest",
	ReasonCurrent:      "up to date",
	ReasonNonSemver:    "non-semver tags, cannot compare automatically",
	ReasonMajor:        "major version bump %1.x → %2.x (review breaking changes)",
	ReasonMinor:        "minor version bump %1 → %2 (new features, review the changelog)",
	ReasonPatch:        "patch version bump %1 → %2 (fixes only, low risk)",
	ReasonPinnedNewer:  "pinned to %1, newer version %2 available, change the tag to update",
	ReasonBreakingIn:   `breaking change flagged in %1 release notes ("%2"), review before updating`,
	ReasonBreaking:     `breaking change flagged in the release notes ("%1"), review before updating`,
	ReasonDigestPinned: "pinned by digest, updates only when the pin changes",
	ReasonLocal:        "no registry digest (built or loaded locally), not checked against a registry",
	ReasonImageID:      "referenced by image ID, no tag to compare",
	ReasonRemoved:      "image removed from the registry",
	ReasonLookupFailed: "upstream lookup failed",
}

// Why builds a reason from its code and values.
func Why(code string, args ...string) Reason { return Reason{Code: code, Args: args} }

// String is the reason in English.
func (r Reason) String() string {
	s := ReasonCodes[r.Code]
	for i, a := range r.Args {
		s = strings.ReplaceAll(s, "%"+strconv.Itoa(i+1), a)
	}
	return s
}

// SetReason records why the status has its risk, as text and as a code.
func (s *UpdateStatus) SetReason(r Reason) { s.RiskCode, s.RiskReason = r, r.String() }

// UpdateStatus is the per-container result the engine stores and serves.
type UpdateStatus struct {
	Container Container `json:"container"`
	// RunningVersion is the version believed to be running. For a pinned tag it
	// is the tag itself; for a rolling tag it is resolved while the running image
	// is the registry's current one and remembered across sweeps, so a later
	// update shows a real "prev -> new" jump.
	RunningVersion string    `json:"running_version"`
	NewestTag      string    `json:"newest_tag"`
	NewestDigest   string    `json:"newest_digest"`
	Kind           Kind      `json:"kind"`
	Risk           RiskLevel `json:"risk"`
	RiskReason     string    `json:"risk_reason"`
	// RiskCode is RiskReason as a code with its values, which the Docker tab
	// says in the viewer's language.
	RiskCode Reason `json:"risk_code,omitzero"`
	// NewerVersion is the newest version tag when the container is pinned to an
	// older version tag that the registry still serves unchanged. A pull of the
	// pinned tag cannot reach it, so Kind stays none: never counted, notified
	// or auto-applied.
	NewerVersion string `json:"newer_version,omitempty"`
	// Unmaintained flags an app at a dead end: its template was removed from
	// Community Applications, its image is gone from the registry, or its source
	// repository was archived. It is independent of Risk, since an app can be up
	// to date and unmaintained at once.
	Unmaintained bool `json:"unmaintained,omitempty"`
	// UnmaintainedReason is a short label for why, e.g. "Removed from Community
	// Applications".
	UnmaintainedReason string `json:"unmaintained_reason,omitempty"`
	// CADeprecated flags an app that Community Applications demoted but still
	// lists. It stays installable and keeps receiving updates, so the flag is
	// informational; Unmaintained takes precedence when both apply.
	CADeprecated bool `json:"ca_deprecated,omitempty"`
	// CADeprecatedNote is the moderator's stated reason.
	CADeprecatedNote string `json:"ca_deprecated_note,omitempty"`
	// VulnRaised marks a risk the engine rated one step higher because the
	// update closes a known critical vulnerability.
	VulnRaised bool       `json:"vuln_raised,omitempty"`
	Changelog  *Changelog `json:"changelog,omitempty"`
	// Vulns holds the known vulnerabilities of the running image while the
	// check is switched on.
	Vulns     *Vulns    `json:"vulns,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	Error     string    `json:"error,omitempty"` // per-container failure, never fatal
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

// Target names the version an update delivers, or "" when all that is known is
// a new image behind the running tag. The changelog's ToTag is the version the
// engine could vouch for. Its newest entry only supplies the spelling, because a
// rolling tag's recent releases say nothing about the image it serves.
func (s UpdateStatus) Target() string {
	to := s.NewestTag
	if cl := s.Changelog; cl != nil && cl.ToTag != "" {
		to = cl.ToTag
		if len(cl.Entries) > 0 && SameVersion(cl.Entries[0].Tag, to) {
			to = cl.Entries[0].Tag
		}
	}
	if SameVersion(to, s.Container.Tag) {
		return ""
	}
	return to
}

// Link returns the changelog file or the changelog's repo root, else the
// container's OCI source repo root, so every message links to the same place.
func (s UpdateStatus) Link() string {
	link := ""
	switch {
	case s.Changelog != nil && s.Changelog.Provider == "file":
		link = s.Changelog.URL
	case s.Changelog != nil:
		link = repoRoot(s.Changelog.URL)
	}
	if link == "" {
		link = repoRoot(s.Container.Source)
	}
	return link
}

// repoRoot keeps only scheme://host/owner/repo from a URL (drops /compare/...).
func repoRoot(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	parts := strings.SplitN(u, "/", 7) // scheme,"",host,owner,repo,...
	if len(parts) < 5 || !strings.HasPrefix(u, "http") {
		return strings.TrimSuffix(u, ".git")
	}
	return strings.TrimSuffix(strings.Join(parts[:5], "/"), ".git")
}

var versionPrefix = regexp.MustCompile(`^(release-)?[vV]?\d`)

// VersionCore strips what projects put around a version in one place and not in
// another, such as a git tag and an image tag: a "release-" or "v" prefix and
// semver build metadata. "v10.3.5", "release-10.3.5" and "10.3.5+build.7" all
// give "10.3.5"; a tag like "latest" is only trimmed.
func VersionCore(tag string) string {
	s := strings.TrimSpace(tag)
	m := versionPrefix.FindStringIndex(s)
	if m == nil {
		return s
	}
	s = s[m[1]-1:]
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	return s
}

// SameVersion reports whether two tags name the same version. A pre-release
// such as "1.2.3-rc1" stays apart from "1.2.3".
func SameVersion(a, b string) bool { return VersionCore(a) == VersionCore(b) }

// Changelog is the resolved "what changed" payload for an update.
type Changelog struct {
	FromTag      string         `json:"from_tag"`
	ToTag        string         `json:"to_tag"`
	SkippedCount int            `json:"skipped_count"`
	Entries      []ReleaseEntry `json:"entries"` // newest first
	Raw          string         `json:"raw"`
	Summary      *AISummary     `json:"summary,omitempty"`
	Source       string         `json:"source"`     // human label, e.g. "GitHub releases via OCI label"
	URL          string         `json:"url"`        // releases/compare link
	Provider     string         `json:"provider"`   // "github" / "fallback" / ...
	Deprecated   bool           `json:"deprecated"` // upstream repo is archived (EOL)
	// RateLimited is set when the upstream API limit (GitHub allows 60 anonymous
	// requests per hour and IP) kept the changelog from resolving.
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

// Vulns counts the known critical and high vulnerabilities of the running
// image and how many of them the newest image no longer has.
type Vulns struct {
	// Scope is VulnScopeImage when the image's own package list was checked
	// and VulnScopeSystem when only the system packages inside the container
	// could be read. Without either, Reason says why nothing was checked.
	Scope    string `json:"scope,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	// NewestKnown says the newest image's package list was read, so the
	// Fixed counts hold. Without it nobody can tell what an update closes.
	NewestKnown   bool `json:"newest_known,omitempty"`
	FixedCritical int  `json:"fixed_critical,omitempty"`
	FixedHigh     int  `json:"fixed_high,omitempty"`
	// Items lists the critical and high ones, critical first.
	Items []VulnItem `json:"items,omitempty"`
}

// VulnItem is one known vulnerability of the running image.
type VulnItem struct {
	ID       string `json:"id"`
	Package  string `json:"package"`
	Severity string `json:"severity"` // "critical" or "high"
	Fixed    bool   `json:"fixed,omitempty"`
}

const (
	VulnScopeImage  = "image"
	VulnScopeSystem = "system"
	// VulnNoPackages: no package list could be read, as for an image without
	// a distribution the database covers.
	VulnNoPackages = "no_packages"
	// VulnLookupFailed: the database could not be asked. The next sweep tries again.
	VulnLookupFailed = "lookup_failed"
)

// ClosesCritical reports whether the newest image is known to close a
// critical vulnerability of the running one.
func (v *Vulns) ClosesCritical() bool { return v != nil && v.NewestKnown && v.FixedCritical > 0 }

// Note sums the report up for a notification, or gives "" when no critical
// or high vulnerability is known. With update set it also says what the
// update closes, or that nobody can tell.
func (v *Vulns) Note(update bool) string {
	if v == nil || v.Reason != "" || v.Critical+v.High == 0 {
		return ""
	}
	total, fixed := v.Critical+v.High, v.FixedCritical+v.FixedHigh
	note := counted(v.Critical, v.High)
	switch {
	case !update:
	case !v.NewestKnown:
		note += ", unknown what the update closes"
	case fixed == 0 && total == 1:
		note += ", the update does not close it"
	case fixed == 0:
		note += ", the update closes none of them"
	case fixed == total && total == 1:
		note += ", the update closes it"
	case fixed == total:
		note += ", the update closes all of them"
	default:
		note += ", the update closes " + severities(v.FixedCritical, v.FixedHigh)
	}
	return note
}

// Closes names what the newest image is known to close, or "".
func (v *Vulns) Closes() string {
	if v == nil || !v.NewestKnown || v.FixedCritical+v.FixedHigh == 0 {
		return ""
	}
	return counted(v.FixedCritical, v.FixedHigh)
}

// severities names the counts that are not zero, such as "2 critical and 5 high".
func severities(critical, high int) string {
	var parts []string
	if critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", critical))
	}
	if high > 0 {
		parts = append(parts, fmt.Sprintf("%d high", high))
	}
	return strings.Join(parts, " and ")
}

func counted(critical, high int) string {
	noun := "vulnerabilities"
	if critical+high == 1 {
		noun = "vulnerability"
	}
	return severities(critical, high) + " known " + noun
}

// VulnNote is the status page's line on the known vulnerabilities, which also
// says why nothing was checked. It is "" while the check is off.
func (s UpdateStatus) VulnNote() string {
	v := s.Vulns
	switch {
	case v == nil:
		return ""
	case v.Reason == VulnNoPackages:
		return "vulnerabilities unknown, no readable package list"
	case v.Reason != "":
		return "vulnerability lookup failed, ShipLog tries again at the next check"
	case v.Critical+v.High == 0:
		return "no known critical or high vulnerabilities"
	}
	note := v.Note(s.HasUpdate())
	if v.Scope == VulnScopeSystem {
		note += " (system packages only)"
	}
	return note
}

// AISummary is populated only when Ollama is configured.
type AISummary struct {
	Bullets  []string `json:"bullets"`
	Breaking []string `json:"breaking"`
	Risk     string   `json:"risk"`
	Model    string   `json:"model"`
}
