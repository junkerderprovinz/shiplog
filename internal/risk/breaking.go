package risk

import (
	"fmt"
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
// effect and does not count. It returns a reason naming the signal and the
// release.
func ScanBreaking(cl *model.Changelog, fromTag string) (string, bool) {
	if cl == nil {
		return "", false
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
	// The raw body cannot be filtered by version. On a same-version rebuild it
	// holds the running release's own notes, which would raise a false critical.
	if advancesVersion(fromTag, cl.ToTag) {
		if sig, ok := findSignal(cl.Raw); ok {
			return breakingReason("", sig), true
		}
	}
	return "", false
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

func breakingReason(tag, signal string) string {
	where := "the release notes"
	if tag != "" {
		where = tag + " release notes"
	}
	return fmt.Sprintf("breaking change flagged in %s (%q), review before updating", where, signal)
}
