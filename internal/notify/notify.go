// Package notify sends ShipLog's messages to the channels the user keeps in
// notify.json, and builds that file once from the settings ShipLog read from
// its environment before.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/notify/legacy"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

const source = "ShipLog"

// Load reads the channels from path. When there is no such file yet, it builds
// them from the old settings in env, saves them to path and reports migrated.
// A file that does not parse is left as it is and returned as an error. An
// empty path builds the channels from env and saves nothing.
func Load(path string, env map[string]string) (channels []kit.Channel, migrated bool, err error) {
	if path == "" {
		return legacy.FromEnv(env), false, nil
	}
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &channels); err != nil {
			return nil, false, fmt.Errorf("read %s: %w", path, err)
		}
		return channels, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	channels = legacy.FromEnv(env)
	if channels == nil {
		channels = []kit.Channel{}
	}
	if err := save(path, channels); err != nil {
		return nil, false, err
	}
	return channels, true, nil
}

// save writes through a temporary file, so a crash never leaves half a file
// that would stop every channel.
func save(path string, channels []kit.Channel) error {
	b, err := json.MarshalIndent(channels, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".notify-*.json")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Notifier sends ShipLog's events. A nil *Notifier sends nothing.
type Notifier struct {
	d *kit.Dispatcher
}

// New returns a Notifier for channels, or nil when none is switched on.
func New(channels []kit.Channel, opt kit.Options) (*Notifier, error) {
	on := false
	for _, c := range channels {
		on = on || c.Enabled
	}
	if !on {
		return nil, nil
	}
	opt.Source = source
	d, err := kit.NewDispatcher(channels, opt)
	if err != nil {
		return nil, err
	}
	return &Notifier{d: d}, nil
}

// Notify announces a newly sighted update or an app that is no longer
// maintained.
func (n *Notifier) Notify(ctx context.Context, st model.UpdateStatus) error {
	return n.Send(ctx, update(st))
}

// Send delivers e to every channel that takes it. The error names each
// channel that failed, with its secrets taken out.
func (n *Notifier) Send(ctx context.Context, e kit.Event) error {
	if n == nil {
		return nil
	}
	var errs []error
	for _, r := range n.d.Send(ctx, e) {
		if !r.OK() {
			errs = append(errs, fmt.Errorf("%s: %w", r.Channel, r.Err))
		}
	}
	return errors.Join(errs...)
}

// Summary is the message after an auto-update run.
func Summary(text, html string, failed bool) kit.Event {
	sev := kit.SeveritySuccess
	if failed {
		sev = kit.SeverityFailure
	}
	return kit.Event{Kind: "autoupdate.run", Severity: sev, Title: "ShipLog update", Body: text, HTML: html}
}

// NotApplied reports an update after which the container still runs the old
// image.
func NotApplied(name string) kit.Event {
	return kit.Event{
		Kind: "update.not_applied", Severity: kit.SeverityFailure,
		Title: name + ": the update did not apply",
		Body:  "The container still runs the old image.",
		HTML:  fmt.Sprintf("🚢 <b>ShipLog</b> · <b>%s</b>: the update did not apply, the container still runs the old image", esc(name)),
	}
}

func update(st model.UpdateStatus) kit.Event {
	name, link := st.Container.Name, st.Link()
	var e kit.Event
	if st.Unmaintained {
		reason := st.UnmaintainedReason
		if reason == "" {
			reason = "no longer maintained"
		}
		e = kit.Event{Kind: "app.unmaintained", Severity: kit.SeverityWarning,
			Title: name + ": no longer maintained", Body: reason,
			HTML: fmt.Sprintf("🚢 <b>ShipLog</b> · <b>%s</b>: no longer maintained (%s)", esc(name), esc(reason))}
	} else {
		from, to := st.Container.Tag, st.Target()
		risk := strings.ToUpper(string(st.Risk))
		e = kit.Event{Kind: "update.available", Severity: severity(st.Risk), Title: name + ": update available"}
		if to == "" {
			e.Body = fmt.Sprintf("New image for %s (risk: %s)", from, risk)
			e.HTML = fmt.Sprintf("🚢 <b>ShipLog</b> · <b>%s</b>: new image for <code>%s</code> (risk: %s)", esc(name), esc(from), esc(risk))
		} else {
			e.Body = fmt.Sprintf("%s → %s (risk: %s)", from, to, risk)
			e.HTML = fmt.Sprintf("🚢 <b>ShipLog</b> · <b>%s</b>: update <code>%s</code> → <code>%s</code> (risk: %s)",
				esc(name), esc(from), esc(to), esc(risk))
		}
	}
	if link != "" {
		e.Link = link
		e.HTML += `<br/><a href="` + esc(link) + `">` + esc(link) + `</a>`
	}
	return e
}

// severity follows the risk: a breaking change needs a manual migration before
// the pull, so it raises an alert rather than a warning.
func severity(r model.RiskLevel) kit.Severity {
	switch r {
	case model.RiskCritical:
		return kit.SeverityCritical
	case model.RiskHigh:
		return kit.SeverityWarning
	}
	return kit.SeverityInfo
}

func esc(s string) string { return template.HTMLEscapeString(s) }
