// Package autoupdate decides which container updates to apply automatically and
// applies them one at a time on a schedule.
package autoupdate

import (
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Level is the SemVer threshold: auto-apply updates at or below it.
type Level int

const (
	LevelOff Level = iota
	LevelPatch
	LevelMinor
	LevelMajor
)

// ParseLevel maps a config string to a Level, defaulting to LevelOff.
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
	Level        Level    // SemVer threshold
	Digest       bool     // also auto-apply :latest and other digest-only moves
	ExcludeWords []string // block an eligible update whose changelog contains one of these
}

// ParseExcludeWords splits the comma-separated setting. It keeps the casing, so
// a blocked-update message quotes the word as the admin typed it.
func ParseExcludeWords(s string) []string {
	var words []string
	for _, w := range strings.Split(s, ",") {
		w = strings.TrimSpace(w)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

// MatchedExcludeWord returns the first word found in the target release's notes,
// ignoring case. An update without release notes is never blocked.
func MatchedExcludeWord(cl *model.Changelog, words []string) string {
	if cl == nil || cl.Raw == "" || len(words) == 0 {
		return ""
	}
	raw := strings.ToLower(cl.Raw)
	for _, w := range words {
		if strings.Contains(raw, strings.ToLower(w)) {
			return w
		}
	}
	return ""
}

// Eligible reports whether the policy allows applying the container's update.
// A version bump that cannot be classified never is.
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
	default:
		return false
	}
}
