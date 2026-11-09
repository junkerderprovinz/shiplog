package fortress

import (
	"fmt"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/store"
)

// texts holds the report's wording in one language. The engine speaks English
// and German; every other language gets English.
type texts struct {
	one, many           string // updates waiting
	majorF, breakingF   string // name, from, to
	failedToF, failedF  string // name, (to,) error
	running, runningOnF string
	appliedF, updatedF  string // name, from, to / name
}

var english = texts{
	one:        "1 container has an update",
	many:       "%d containers have an update",
	majorF:     "%s: %s → %s is a major jump",
	breakingF:  "%s: %s → %s, the release notes name a breaking change",
	failedToF:  "%s: the auto-update to %s failed (%s)",
	failedF:    "%s: the auto-update failed (%s)",
	running:    "Auto-update",
	runningOnF: "Auto-update: %s",
	appliedF:   "%s updated from %s to %s",
	updatedF:   "%s updated",
}

var german = texts{
	one:        "1 Container hat ein Update",
	many:       "%d Container haben ein Update",
	majorF:     "%s: %s → %s ist ein großer Versionssprung",
	breakingF:  "%s: %s → %s, laut Release-Notes mit einer inkompatiblen Änderung",
	failedToF:  "%s: Das automatische Update auf %s ist gescheitert (%s)",
	failedF:    "%s: Das automatische Update ist gescheitert (%s)",
	running:    "Automatisches Update",
	runningOnF: "Automatisches Update: %s",
	appliedF:   "%s von %s auf %s aktualisiert",
	updatedF:   "%s aktualisiert",
}

func textsFor(lang string) texts {
	primary, _, _ := strings.Cut(strings.ToLower(lang), "-")
	if primary == "de" {
		return german
	}
	return english
}

func (t texts) waiting(n int) string {
	if n == 1 {
		return t.one
	}
	return fmt.Sprintf(t.many, n)
}

func (t texts) major(name, from, to string) string { return fmt.Sprintf(t.majorF, name, from, to) }

func (t texts) breaking(name, from, to string) string {
	return fmt.Sprintf(t.breakingF, name, from, to)
}

func (t texts) runningOn(name string) string { return fmt.Sprintf(t.runningOnF, name) }

func (t texts) failed(rec store.AutoUpdateRecord) string {
	if rec.ToVer == "" {
		return fmt.Sprintf(t.failedF, rec.Name, rec.Err)
	}
	return fmt.Sprintf(t.failedToF, rec.Name, rec.ToVer, rec.Err)
}

func (t texts) applied(rec store.AutoUpdateRecord) string {
	if rec.FromVer == "" || rec.ToVer == "" {
		return fmt.Sprintf(t.updatedF, rec.Name)
	}
	return fmt.Sprintf(t.appliedF, rec.Name, rec.FromVer, rec.ToVer)
}
