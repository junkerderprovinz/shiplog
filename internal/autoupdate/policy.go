// Package autoupdate decides which container updates to apply automatically and
// runs them serially. The policy here is pure (no I/O), so it is exhaustively
// unit-tested; the schedule and executor build on top of it.
package autoupdate

import "github.com/junkerderprovinz/shiplog/internal/model"

// Level is the SemVer threshold: auto-apply updates at or below it.
type Level int

const (
	LevelOff Level = iota
	LevelPatch
	LevelMinor
	LevelMajor
)

// ParseLevel maps a config string to a Level; anything unrecognised → LevelOff.
func ParseLevel(s string) Level {
	switch s {
	case "patch":
		return LevelPatch
	case "minor":
		return LevelMinor
	case "major":
		return LevelMajor
	default:
		return LevelOff
	}
}

// Policy is the global auto-update decision input.
type Policy struct {
	Level  Level // SemVer threshold
	Digest bool  // also auto-apply :latest / digest-only moves (level-less)
}

// Eligible reports whether the container's available update should be applied
// under the policy. Unknown (non-SemVer) bumps are never eligible; digest moves
// only via the separate toggle; SemVer bumps only at or below the threshold.
func Eligible(st model.UpdateStatus, p Policy) bool {
	if !st.HasUpdate() {
		return false
	}
	switch st.Kind {
	case model.KindPatch:
		return p.Level >= LevelPatch
	case model.KindMinor:
		return p.Level >= LevelMinor
	case model.KindMajor:
		return p.Level >= LevelMajor
	case model.KindDigest:
		return p.Digest
	default: // KindUnknown, KindNone, anything unclassified — never auto
		return false
	}
}
