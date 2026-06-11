// Package risk classifies the delta between a running image and the newest
// available one, returning a severity verdict and a human-readable reason.
// It is a pure unit: no I/O, no external dependencies — only the standard
// library and the shared model types.
package risk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Classify compares the current tag/digest against the newest tag/digest and
// returns the update Kind, its RiskLevel, and a human-readable reason.
//
// Rules:
//   - Same tag: if both digests are non-empty and differ, the image moved under
//     a fixed tag (KindDigest/RiskLow); otherwise it is up to date
//     (KindNone/RiskNone).
//   - Different tags that both parse as semver: the first differing level
//     (major, then minor, then patch) decides the kind/risk. A newest that is
//     lower than or equal to the current is treated as up to date.
//   - Anything else: KindUnknown/RiskUnknown — the tags can't be compared.
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
		// Newest is lower than or equal to current (e.g. a downgrade, or the
		// same version expressed differently). Nothing to upgrade to.
		return model.KindNone, model.RiskNone, "up to date"
	case newVer.major != curVer.major:
		reason := fmt.Sprintf("major version bump %d.x → %d.x — review breaking changes", curVer.major, newVer.major)
		return model.KindMajor, model.RiskHigh, reason
	case newVer.minor != curVer.minor:
		reason := fmt.Sprintf("minor version bump %d.%d → %d.%d — new features, low risk", curVer.major, curVer.minor, newVer.major, newVer.minor)
		return model.KindMinor, model.RiskMedium, reason
	default:
		reason := fmt.Sprintf("patch version bump %s → %s — fixes only, low risk", curVer.String(), newVer.String())
		return model.KindPatch, model.RiskLow, reason
	}
}

// semver is a parsed major.minor.patch triple. Missing components are zero.
type semver struct {
	major, minor, patch int
}

// String renders the triple as "major.minor.patch".
func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

// compare returns -1 if v < o, 0 if equal, +1 if v > o.
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

// parseSemver hand-parses a tag into a semver triple. It strips a leading 'v',
// drops any '-prerelease' or '+build' suffix, splits on '.', and reads up to
// three numeric components (missing components default to 0). It reports false
// if there are zero numeric components or any present component is non-numeric.
func parseSemver(tag string) (semver, bool) {
	core := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if core == "" {
		return semver{}, false
	}
	// Drop build metadata first ("+..."), then any pre-release ("-...").
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
