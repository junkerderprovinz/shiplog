package risk

import (
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// breakingSignals are phrases in release notes that mean the operator has to
// act before the pull, such as a manual migration or an incompatible format.
// The version delta cannot show this: a ":latest" digest move reads as low risk
// even when the notes demand a hand-run database migration. Generic "back up
// first" advice is left out so that a critical verdict stays rare enough to be
// trusted. Entries are lower-case.
var breakingSignals = []string{
	"breaking change",
	"breaking:",
	"action required",
	"manual intervention",
	"manual migration",
	"manual action",
	"manual step",
	"must migrate",
	"must be migrated",
	"requires migration",
	"requires a migration",
	"requires manual",
	"no longer supported",
	"dropped support",
	"drop support for",
	"removed support",
	"backward incompatible",
	"backwards incompatible",
	"not backward compatible",
	"not backwards compatible",
	"data loss",
	"irreversible migration",
}

// ScanBreaking looks for a breaking signal in the notes of every release newer
// than fromTag, since a multi-version span can hide it in an intermediate
// release. A note in a release already running describes a change already in
// effect and does not count. With fromTag empty nothing shows that a release is
// still ahead of the container, since a rolling tag's recent releases may all be
// behind it. It returns a reason naming the signal and the release.
func ScanBreaking(cl *model.Changelog, fromTag string) (model.Reason, bool) {
	if cl == nil || fromTag == "" {
		return model.Reason{}, false
	}
	from, fromOK := parseSemver(fromTag)
	for _, e := range cl.Entries {
		// Without two semver tags there is no order, so the entry is scanned.
		if fromOK {
			if ev, ok := parseSemver(e.Tag); ok && ev.compare(from) <= 0 {
				continue
			}
		}
		if sig, ok := findSignal(e.Body); ok {
			return breakingReason(e.Tag, sig), true
		}
	}
	// The raw body cannot be filtered by version, and beside entries it only repeats
	// the newest one, which may be the running release's own notes.
	if len(cl.Entries) == 0 && advancesVersion(fromTag, cl.ToTag) {
		if sig, ok := findSignal(cl.Raw); ok {
			return breakingReason("", sig), true
		}
	}
	return model.Reason{}, false
}

// ScanRelease looks for a breaking signal in the notes of tag alone, for a
// container whose running version is unknown: tag is the one release known to be
// on offer, and every other entry may be behind what the container runs.
func ScanRelease(cl *model.Changelog, tag string) (model.Reason, bool) {
	if cl == nil || tag == "" {
		return model.Reason{}, false
	}
	for _, e := range cl.Entries {
		if !model.SameVersion(e.Tag, tag) {
			continue
		}
		if sig, ok := findSignal(e.Body); ok {
			return breakingReason(e.Tag, sig), true
		}
		break
	}
	return model.Reason{}, false
}

// advancesVersion reports whether toTag is newer than fromTag, falling back to
// plain inequality when either is not semver.
func advancesVersion(fromTag, toTag string) bool {
	f, fok := parseSemver(fromTag)
	t, tok := parseSemver(toTag)
	if fok && tok {
		return t.compare(f) > 0
	}
	from := strings.TrimSpace(fromTag)
	to := strings.TrimSpace(toTag)
	return to != "" && to != from
}

func findSignal(body string) (string, bool) {
	if body == "" {
		return "", false
	}
	low := strings.ToLower(body)
	for _, s := range breakingSignals {
		if strings.Contains(low, s) {
			return s, true
		}
	}
	return "", false
}

func breakingReason(tag, signal string) model.Reason {
	if tag != "" {
		return model.Why(model.ReasonBreakingIn, tag, signal)
	}
	return model.Why(model.ReasonBreaking, signal)
}
