package risk

import (
	"fmt"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// breakingSignals are phrases in upstream release notes that mark an update as
// needing operator action BEFORE the pull: a required migration, a removed or
// swapped extension, a dropped/renamed API, an incompatible on-disk format. The
// version delta alone never reveals these — a ":latest" digest move reads as
// "low" even when the notes say the database must be migrated by hand (the exact
// trap that let an Immich pgvecto.rs -> VectorChord swap slip through as low
// risk). Matching any one escalates the verdict to critical.
//
// The list is deliberately high-signal: vague "back up first" boilerplate is
// left out so a critical verdict stays rare enough to be trusted. All entries
// are lower-case; matching is case-insensitive substring.
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

// ScanBreaking looks through a changelog's release notes for a signal that the
// update needs manual action before it is safe to pull. It only considers
// releases STRICTLY NEWER than fromTag (the running version): a breaking note in
// a release you already run — or in a same-version digest rebuild where the span
// does not advance — describes a change already in effect, not a pending action,
// and must never raise the verdict. It scans every qualifying release entry body
// first (a multi-version span can hide the note in an intermediate release, not
// the newest), then falls back to the raw body, but only when the span genuinely
// advances the version. Returns a human-readable reason naming the matched signal
// and the release it came from, or ("", false) when nothing matches. Pure: no I/O.
func ScanBreaking(cl *model.Changelog, fromTag string) (string, bool) {
	if cl == nil {
		return "", false
	}
	from, fromOK := parseSemver(fromTag)
	for _, e := range cl.Entries {
		// A note in a release at or below the running version is already in place,
		// not part of the pending update — skip it. Only filter when both tags
		// parse as semver; otherwise stay conservative and scan the entry.
		if fromOK {
			if ev, ok := parseSemver(e.Tag); ok && ev.compare(from) <= 0 {
				continue
			}
		}
		if sig, ok := findSignal(e.Body); ok {
			return breakingReason(e.Tag, sig), true
		}
	}
	// The raw body is unstructured and can't be version-filtered, so only trust it
	// when the span actually advances the version. On a same-version digest rebuild
	// the raw body is the CURRENT release's own notes, whose breaking line is
	// already in effect — scanning it there would raise a phantom "critical".
	if advancesVersion(fromTag, cl.ToTag) {
		if sig, ok := findSignal(cl.Raw); ok {
			return breakingReason("", sig), true
		}
	}
	return "", false
}

// advancesVersion reports whether toTag is a strictly newer version than fromTag.
// When both parse as semver it compares them; otherwise it falls back to a plain
// inequality, so a rolling tag that resolved to the same version (or the same
// literal tag) is correctly treated as "no advance", while genuinely different
// tags still permit the raw-body fallback.
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

// findSignal reports the first breaking signal contained in body (case-insensitive).
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

// breakingReason renders the escalation reason, naming the release the signal
// came from when known.
func breakingReason(tag, signal string) string {
	where := "the release notes"
	if tag != "" {
		where = tag + " release notes"
	}
	return fmt.Sprintf("breaking change flagged in %s (%q) — review before updating", where, signal)
}
