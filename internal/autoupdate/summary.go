package autoupdate

import (
	"fmt"
	"strings"
)

// RenderSummary builds the plain-text + HTML bodies for one auto-update run, or
// ("","") when nothing was eligible (the caller then sends nothing). It lives in
// this package (not notify) so notify stays model-only and this owns its wording.
func RenderSummary(res Result) (text, html string) {
	if len(res.Outcomes) == 0 {
		return "", ""
	}
	verb := "Auto-updated"
	if res.DryRun {
		verb = "Would auto-update"
	}
	var updated, failed []Outcome
	for _, o := range res.Outcomes {
		if o.Err != nil {
			failed = append(failed, o)
		} else {
			updated = append(updated, o)
		}
	}

	var t, h strings.Builder
	t.WriteString("🚢 ShipLog · ")
	h.WriteString("🚢 <b>ShipLog</b> · ")
	if len(updated) > 0 {
		var tp, hp []string
		for _, o := range updated {
			tp = append(tp, fmt.Sprintf("%s %s→%s (%s)", o.Name, verOr(o.From), verOr(o.To), o.Level))
			hp = append(hp, fmt.Sprintf("<b>%s</b> <code>%s→%s</code> (%s)", esc(o.Name), esc(verOr(o.From)), esc(verOr(o.To)), esc(o.Level)))
		}
		fmt.Fprintf(&t, "%s %d: %s", verb, len(updated), strings.Join(tp, ", "))
		fmt.Fprintf(&h, "%s %d: %s", verb, len(updated), strings.Join(hp, ", "))
	} else {
		t.WriteString("Auto-update run")
		h.WriteString("Auto-update run")
	}
	if len(failed) > 0 {
		var tp, hp []string
		for _, o := range failed {
			tp = append(tp, fmt.Sprintf("%s (%s)", o.Name, o.Err.Error()))
			hp = append(hp, fmt.Sprintf("%s (%s)", esc(o.Name), esc(o.Err.Error())))
		}
		fmt.Fprintf(&t, ". %d failed: %s", len(failed), strings.Join(tp, ", "))
		fmt.Fprintf(&h, ". %d failed: %s", len(failed), strings.Join(hp, ", "))
	}
	return t.String(), h.String()
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
