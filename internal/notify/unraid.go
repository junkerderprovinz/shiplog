// Unraid sends native Unraid notifications by shelling out to Unraid's own
// notify tool. They appear in Unraid's notification centre and fan out to every
// agent the user configured (email, Discord, Telegram, Pushover, ...). It only
// works on an Unraid host (the ShipLog plugin runs the engine as a host daemon),
// so New returns nil when the tool is absent or the feature is off.
package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// UnraidScriptPath is Unraid's notification helper. Overridable via
// UNRAID_NOTIFY_SCRIPT (used by tests; real Unraid hosts use the default).
const UnraidScriptPath = "/usr/local/emhttp/webGui/scripts/notify"

// Unraid shells out to the Unraid notify tool.
type Unraid struct {
	script string
}

// NewUnraid returns an Unraid notifier when the feature is enabled and the
// notify tool exists, else nil (a nil *Unraid is a no-op on every method, so
// callers need not special-case it).
func NewUnraid(enabled bool) *Unraid {
	if !enabled {
		return nil
	}
	script := os.Getenv("UNRAID_NOTIFY_SCRIPT")
	if script == "" {
		script = UnraidScriptPath
	}
	if _, err := os.Stat(script); err != nil {
		return nil // not on an Unraid host (or notify tool missing) → stay silent
	}
	return &Unraid{script: script}
}

// Notify sends a per-update notification. nil receiver → no-op.
func (u *Unraid) Notify(ctx context.Context, st model.UpdateStatus) error {
	if u == nil {
		return nil
	}
	return u.run(ctx, updateArgs(st))
}

// SendMessage sends the auto-update run summary as a normal notification. The
// HTML form is ignored — Unraid notifications are plain text. nil → no-op.
func (u *Unraid) SendMessage(ctx context.Context, text, _ string) error {
	if u == nil {
		return nil
	}
	return u.run(ctx, []string{"-e", "ShipLog", "-s", "ShipLog auto-update", "-d", text, "-i", "normal"})
}

func (u *Unraid) run(ctx context.Context, args []string) error {
	out, err := exec.CommandContext(ctx, u.script, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unraid notify: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// updateArgs builds the notify CLI arguments for one container update. Pure, so
// it is unit-testable without invoking the script.
func updateArgs(st model.UpdateStatus) []string {
	name := st.Container.Name
	from := st.Container.Tag
	to := st.NewestTag
	if st.Changelog != nil && len(st.Changelog.Entries) > 0 && st.Changelog.Entries[0].Tag != "" {
		to = st.Changelog.Entries[0].Tag
	}
	risk := strings.ToUpper(string(st.Risk))

	// Unraid importance: a major (high-risk) update is a "warning", the rest are
	// "normal". "alert" is reserved for genuine failures, not update news.
	importance := "normal"
	if st.Risk == model.RiskHigh {
		importance = "warning"
	}

	subject := fmt.Sprintf("%s: update available", name)
	desc := fmt.Sprintf("%s → %s (risk: %s)", from, to, risk)
	args := []string{"-e", "ShipLog", "-s", subject, "-d", desc, "-i", importance}
	if link := updateLink(st); link != "" {
		args = append(args, "-l", link)
	}
	return args
}
