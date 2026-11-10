package autoupdate

import (
	"errors"
	"fmt"
	"strings"
)

// RenderSummary builds the plain-text and HTML bodies for one auto-update run,
// both empty when nothing was eligible.
func RenderSummary(res Result) (text, html string) {
	if len(res.Outcomes) == 0 {
		return "", ""
	}
	verb, run := "Auto-updated", "Auto-update run"
	if res.DryRun {
		verb, run = "Would auto-update", "Auto-update dry run"
	}
	var updated, failed, held, blocked, skipped, current []Outcome
	for _, o := range res.Outcomes {
		switch {
		case o.Skipped:
			skipped = append(skipped, o)
		case o.UpToDate:
			current = append(current, o)
		case o.Blocked:
			blocked = append(blocked, o)
		case o.NoBackup:
			held = append(held, o)
		case o.Err != nil:
			failed = append(failed, o)
		default:
			updated = append(updated, o)
		}
	}

	var t, h strings.Builder
	t.WriteString("🚢 ShipLog · ")
	h.WriteString("🚢 <b>ShipLog</b> · ")
	if len(updated) > 0 {
		var tp, hp []string
		for _, o := range updated {
			note := withKept(o.Level, o)
			switch {
			case o.WithoutBackup:
				note += ", without a backup"
			case o.Snapshot.Run != "" || o.Snapshot.ID != "":
				note += ", backed up first"
			}
			tp = append(tp, fmt.Sprintf("%s %s→%s (%s)", o.Name, verOr(o.From), verOr(o.To), note))
			hp = append(hp, fmt.Sprintf("<b>%s</b> <code>%s→%s</code> (%s)", esc(o.Name), esc(verOr(o.From)), esc(verOr(o.To)), esc(note)))
		}
		fmt.Fprintf(&t, "%s %d: %s", verb, len(updated), strings.Join(tp, ", "))
		fmt.Fprintf(&h, "%s %d: %s", verb, len(updated), strings.Join(hp, ", "))
	} else {
		t.WriteString(run)
		h.WriteString(run)
	}
	if len(failed) > 0 {
		var tp, hp []string
		for _, o := range failed {
			note := withKept(o.Err.Error(), o)
			if !o.RolledBack {
				tp = append(tp, fmt.Sprintf("%s (%s)", o.Name, note))
				hp = append(hp, fmt.Sprintf("%s (%s)", esc(o.Name), esc(note)))
				continue
			}
			note += ", back on its old image and pinned to it in the template"
			label := "backup " + o.Snapshot.ID
			if o.Snapshot.ID == "" {
				label = "backup"
			}
			snap := esc(label)
			if o.Snapshot.Link != "" {
				snap = fmt.Sprintf(`<a href="%s">%s</a>`, esc(o.Snapshot.Link), esc(label))
			}
			tp = append(tp, fmt.Sprintf("%s (%s, %s)", o.Name, note, label))
			hp = append(hp, fmt.Sprintf("%s (%s, %s)", esc(o.Name), esc(note), snap))
		}
		fmt.Fprintf(&t, ". %d failed: %s", len(failed), strings.Join(tp, ", "))
		fmt.Fprintf(&h, ". %d failed: %s", len(failed), strings.Join(hp, ", "))
	}
	if len(held) > 0 {
		var tp, hp []string
		for _, o := range held {
			reason := o.Err
			if inner := errors.Unwrap(o.Err); inner != nil {
				reason = inner
			}
			tp = append(tp, fmt.Sprintf("%s (%s)", o.Name, reason))
			hp = append(hp, fmt.Sprintf("%s (%s)", esc(o.Name), esc(reason.Error())))
		}
		const hint = `. To update one anyway, turn on "Update without a backup" in its ShipLog window`
		fmt.Fprintf(&t, ". %d held back without a backup: %s%s", len(held), strings.Join(tp, ", "), hint)
		fmt.Fprintf(&h, ". %d held back without a backup: %s%s", len(held), strings.Join(hp, ", "), esc(hint))
	}
	if len(blocked) > 0 {
		var tp, hp []string
		for _, o := range blocked {
			tp = append(tp, fmt.Sprintf("%s (changelog mentions '%s')", o.Name, o.BlockedWord))
			hp = append(hp, fmt.Sprintf("%s (changelog mentions '%s')", esc(o.Name), esc(o.BlockedWord)))
		}
		fmt.Fprintf(&t, ". %d blocked: %s", len(blocked), strings.Join(tp, ", "))
		fmt.Fprintf(&h, ". %d blocked: %s", len(blocked), strings.Join(hp, ", "))
	}
	if len(skipped) > 0 {
		var tp, hp []string
		for _, o := range skipped {
			tp = append(tp, fmt.Sprintf("%s %s→%s (%s)", o.Name, verOr(o.From), verOr(o.To), o.Level))
			hp = append(hp, fmt.Sprintf("%s <code>%s→%s</code> (%s)", esc(o.Name), esc(verOr(o.From)), esc(verOr(o.To)), esc(o.Level)))
		}
		fmt.Fprintf(&t, ". %d skipped (excluded from auto-update): %s", len(skipped), strings.Join(tp, ", "))
		fmt.Fprintf(&h, ". %d skipped (excluded from auto-update): %s", len(skipped), strings.Join(hp, ", "))
	}
	if len(current) > 0 {
		var tp, hp []string
		for _, o := range current {
			tp = append(tp, o.Name)
			hp = append(hp, esc(o.Name))
		}
		fmt.Fprintf(&t, ". %d already up to date: %s", len(current), strings.Join(tp, ", "))
		fmt.Fprintf(&h, ". %d already up to date: %s", len(current), strings.Join(hp, ", "))
	}
	return t.String(), h.String()
}

// withKept adds to note what the run did to keep the container after its update.
func withKept(note string, o Outcome) string {
	switch {
	case o.Recreated && o.Started:
		return note + ", recreated from its template and started"
	case o.Recreated:
		return note + ", recreated from its template"
	case o.Started:
		return note + ", started again"
	}
	return note
}

func verOr(v string) string {
	if v == "" {
		return "?"
	}
	return v
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
