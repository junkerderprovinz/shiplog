// Package risk classifies the delta between a running image and the newest
// available one into a severity verdict with a readable reason.
package risk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Classify compares the current tag and digest against the newest ones. Under
// the same tag, differing digests mean the image moved. Semver tags are ranked
// by the first level that differs, and a newest version at or below the
// current one counts as up to date. Other tags cannot be compared.
func Classify(cur, newest, curDigest, newDigest string) (model.Kind, model.RiskLevel, string) {
	if cur == newest {
		if curDigest != "" && newDigest != "" && curDigest != newDigest {
			return model.KindDigest, model.RiskLow, "same tag, new image digest"
		}
		return model.KindNone, model.RiskNone, "up to date"
	}

	curVer, curOK := parseSemver(cur)
	newVer, newOK := parseSemver(newest)
	if !curOK || !newOK {
		return model.KindUnknown, model.RiskUnknown, "non-semver tags, cannot compare automatically"
	}

	switch cmp := newVer.compare(curVer); {
	case cmp <= 0:
		return model.KindNone, model.RiskNone, "up to date"
	case newVer.major != curVer.major:
		reason := fmt.Sprintf("major version bump %d.x → %d.x (review breaking changes)", curVer.major, newVer.major)
		return model.KindMajor, model.RiskHigh, reason
	case newVer.minor != curVer.minor:
		reason := fmt.Sprintf("minor version bump %d.%d → %d.%d (new features, review the changelog)", curVer.major, curVer.minor, newVer.major, newVer.minor)
		return model.KindMinor, model.RiskMedium, reason
	default:
		reason := fmt.Sprintf("patch version bump %s → %s (fixes only, low risk)", curVer.String(), newVer.String())
		return model.KindPatch, model.RiskLow, reason
	}
}

// semver is a parsed major.minor.patch triple. Missing components are zero.
type semver struct {
	major, minor, patch int
}

func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (v semver) compare(o semver) int {
	if c := cmpInt(v.major, o.major); c != 0 {
		return c
	}
	if c := cmpInt(v.minor, o.minor); c != 0 {
		return c
	}
	return cmpInt(v.patch, o.patch)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// parseSemver reads up to three numeric components from a tag, ignoring a
// leading "v" and any pre-release or build suffix. Missing components are 0.
func parseSemver(tag string) (semver, bool) {
	core := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if core == "" {
		return semver{}, false
	}
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core = core[:i]
	}
	if i := strings.IndexByte(core, '-'); i >= 0 {
		core = core[:i]
	}
	if core == "" {
		return semver{}, false
	}

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		parts = parts[:3]
	}

	var v semver
	dst := []*int{&v.major, &v.minor, &v.patch}
	for i, p := range parts {
		if p == "" {
			return semver{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		*dst[i] = n
	}
	return v, true
}
