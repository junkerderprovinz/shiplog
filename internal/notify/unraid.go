package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// UnraidScriptPath is Unraid's notification helper. Tests override it through
// UNRAID_NOTIFY_SCRIPT.
const UnraidScriptPath = "/usr/local/emhttp/webGui/scripts/notify"

// Unraid shells out to Unraid's notify tool, whose notifications fan out to
// every agent the user configured (email, Discord, Pushover, ...).
type Unraid struct {
	script string
}

// NewUnraid returns nil when the feature is off or the notify tool is missing,
// as on any host that is not Unraid. A nil *Unraid does nothing.
func NewUnraid(enabled bool) *Unraid {
	if !enabled {
		return nil
	}
	script := os.Getenv("UNRAID_NOTIFY_SCRIPT")
	if script == "" {
		script = UnraidScriptPath
	}
	if _, err := os.Stat(script); err != nil {
		return nil
	}
	return &Unraid{script: script}
}

// Notify sends a per-update notification.
func (u *Unraid) Notify(ctx context.Context, st model.UpdateStatus) error {
	if u == nil {
		return nil
	}
	return u.run(ctx, updateArgs(st))
}

// SendMessage sends the auto-update run summary. The HTML form is dropped
// because Unraid notifications are plain text.
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

func updateArgs(st model.UpdateStatus) []string {
	name := st.Container.Name
	if st.Unmaintained {
		reason := st.UnmaintainedReason
		if reason == "" {
			reason = "no longer maintained"
		}
		args := []string{"-e", "ShipLog", "-s", fmt.Sprintf("%s: no longer maintained", name), "-d", reason, "-i", "warning"}
		if link := updateLink(st); link != "" {
			args = append(args, "-l", link)
		}
		return args
	}
	from := st.Container.Tag
	to := st.NewestTag
	if st.Changelog != nil && len(st.Changelog.Entries) > 0 && st.Changelog.Entries[0].Tag != "" {
		to = st.Changelog.Entries[0].Tag
	}
	risk := strings.ToUpper(string(st.Risk))

	// A breaking change needs a manual migration before the pull, so it raises an
	// alert rather than a warning.
	importance := "normal"
	switch st.Risk {
	case model.RiskCritical:
		importance = "alert"
	case model.RiskHigh:
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
