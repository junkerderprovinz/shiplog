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
// the same version, differing digests mean the image moved. Semver tags are ranked
// by the first level that differs, and a newest version at or below the
// current one counts as up to date. Other tags cannot be compared.
func Classify(cur, newest, curDigest, newDigest string) (model.Kind, model.RiskLevel, model.Reason) {
	if model.SameVersion(cur, newest) {
		if curDigest != "" && newDigest != "" && curDigest != newDigest {
			return model.KindDigest, model.RiskLow, model.Why(model.ReasonDigest)
		}
		return model.KindNone, model.RiskNone, model.Why(model.ReasonCurrent)
	}

	curVer, curOK := parseSemver(cur)
	newVer, newOK := parseSemver(newest)
	if !curOK || !newOK {
		return model.KindUnknown, model.RiskUnknown, model.Why(model.ReasonNonSemver)
	}

	switch cmp := newVer.compare(curVer); {
	case cmp <= 0:
		return model.KindNone, model.RiskNone, model.Why(model.ReasonCurrent)
	case newVer.major != curVer.major:
		return model.KindMajor, model.RiskHigh, model.Why(model.ReasonMajor, strconv.Itoa(curVer.major), strconv.Itoa(newVer.major))
	case newVer.minor != curVer.minor:
		return model.KindMinor, model.RiskMedium, model.Why(model.ReasonMinor,
			fmt.Sprintf("%d.%d", curVer.major, curVer.minor), fmt.Sprintf("%d.%d", newVer.major, newVer.minor))
	default:
		return model.KindPatch, model.RiskLow, model.Why(model.ReasonPatch, curVer.String(), newVer.String())
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

// parseSemver reads up to three numeric components from a tag, ignoring what
// model.VersionCore strips and any pre-release suffix. Missing components are 0.
func parseSemver(tag string) (semver, bool) {
	core := model.VersionCore(tag)
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
