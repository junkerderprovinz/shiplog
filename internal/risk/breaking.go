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
// update needs manual action before it is safe to pull. It scans every release
// entry body first — a multi-version span can hide the breaking note in an
// intermediate release, not the newest — then falls back to the raw body. It
// returns a human-readable reason naming the matched signal and the release it
// came from, or ("", false) when nothing matches. Pure: no I/O.
func ScanBreaking(cl *model.Changelog) (string, bool) {
	if cl == nil {
		return "", false
	}
	for _, e := range cl.Entries {
		if sig, ok := findSignal(e.Body); ok {
			return breakingReason(e.Tag, sig), true
		}
	}
	if sig, ok := findSignal(cl.Raw); ok {
		return breakingReason("", sig), true
	}
	return "", false
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
