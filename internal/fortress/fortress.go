// Package fortress reports ShipLog to the other apps of a fortress through the
// status contract: the updates that wait, the risky ones among them, failed
// auto-updates, the auto-update that runs right now and the auto-update log.
package fortress

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

// Store is what the report reads.
type Store interface {
	List() ([]model.UpdateStatus, error)
	LatestAutoUpdates() ([]store.AutoUpdateRecord, error)
	AutoUpdatesAfter(after int64, limit int) ([]store.AutoUpdateRecord, error)
}

// lostFor is how long a failed auto-update of a container that is gone stays
// in the report. A container the user removed would otherwise stay forever.
const lostFor = 7 * 24 * time.Hour

// dockerTab is where the user deals with an update.
const dockerTab = "/Docker"

// Run is the auto-update run in progress. The zero value is idle.
type Run struct {
	mu      sync.Mutex
	started time.Time
	current string
}

// Begin marks a run as started at.
func (r *Run) Begin(at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started, r.current = at, ""
}

// Updating names the container the run updates now.
func (r *Run) Updating(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = name
}

// End marks the run as done.
func (r *Run) End() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started, r.current = time.Time{}, ""
}

func (r *Run) snapshot() (time.Time, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started, r.current
}

// Source answers the status contract for ShipLog.
type Source struct {
	App      status.App
	Instance status.Instance
	Store    Store
	Run      *Run
	Now      func() time.Time
}

// Report lists what waits for the user and the auto-update that runs.
func (s *Source) Report(_ context.Context, lang string) (status.Report, error) {
	t := textsFor(lang)
	rows, err := s.Store.List()
	if err != nil {
		return status.Report{}, err
	}
	latest, err := s.Store.LatestAutoUpdates()
	if err != nil {
		return status.Report{}, err
	}
	r := status.Report{App: s.App, Instance: s.Instance, State: status.StateOK}

	waiting := 0
	for _, st := range rows {
		if st.HasUpdate() {
			waiting++
		}
	}
	if waiting > 0 {
		r.NeedsYou = append(r.NeedsYou, status.Need{ID: "updates", Severity: status.SeverityInfo,
			Code: "updates.available", Text: t.waiting(waiting), Link: dockerTab})
	}
	for _, st := range rows {
		if n, ok := risky(st, t); ok {
			r.NeedsYou = append(r.NeedsYou, n)
		}
	}
	r.NeedsYou = append(r.NeedsYou, s.failed(rows, latest, t)...)

	if started, current := s.Run.snapshot(); !started.IsZero() {
		title := t.running
		if current != "" {
			title = t.runningOn(current)
		}
		r.Work = append(r.Work, status.Task{ID: "autoupdate", Kind: status.KindUpdate, Title: title,
			StartedAt: started, BlocksRestart: true})
	}
	return r, nil
}

// risky flags an update the auto-update would not take on its own: a major
// jump or release notes that name a breaking change.
func risky(st model.UpdateStatus, t texts) (status.Need, bool) {
	if !st.HasUpdate() {
		return status.Need{}, false
	}
	to := st.Target()
	if to == "" {
		to = st.NewestTag
	}
	n := status.Need{ID: "risky-" + st.Container.Name, Severity: status.SeverityWarn, Link: st.Link()}
	switch st.Risk {
	case model.RiskCritical:
		n.Code, n.Text = "update.breaking", t.breaking(st.Container.Name, st.Container.Tag, to)
	case model.RiskHigh:
		n.Code, n.Text = "update.major", t.major(st.Container.Name, st.Container.Tag, to)
	default:
		return status.Need{}, false
	}
	return n, true
}

// failed keeps the last auto-update of a container in the report while it
// failed and the update still waits, or while the container it lost is gone.
func (s *Source) failed(rows []model.UpdateStatus, latest []store.AutoUpdateRecord, t texts) []status.Need {
	waits := map[string]bool{}
	for _, st := range rows {
		waits[st.Container.Name] = st.HasUpdate()
	}
	var out []status.Need
	for _, rec := range latest {
		if rec.Success {
			continue
		}
		at := time.Unix(rec.At, 0).UTC()
		pending, listed := waits[rec.Name]
		if (listed && !pending) || (!listed && s.Now().Sub(at) > lostFor) {
			continue
		}
		out = append(out, status.Need{ID: "autoupdate-failed-" + rec.Name, Severity: status.SeverityWarn,
			Code: "autoupdate.failed", Text: t.failed(rec), Since: at, Link: dockerTab})
	}
	return out
}

// Log pages through the auto-update log. The cursor is the id of the last
// entry handed out; one ShipLog did not hand out starts at the beginning.
func (s *Source) Log(_ context.Context, q status.LogQuery) (status.LogPage, error) {
	after, err := strconv.ParseInt(q.After, 10, 64)
	if err != nil || after < 0 {
		after = 0
	}
	recs, err := s.Store.AutoUpdatesAfter(after, q.Limit)
	if err != nil {
		return status.LogPage{}, err
	}
	t := textsFor(q.Lang)
	page := status.LogPage{Next: q.After}
	for _, rec := range recs {
		e := status.LogEntry{ID: strconv.FormatInt(rec.ID, 10), At: time.Unix(rec.At, 0).UTC(), Link: dockerTab}
		if rec.Success {
			e.Level, e.Code, e.Text = status.SeverityInfo, "autoupdate.applied", t.applied(rec)
		} else {
			e.Level, e.Code, e.Text = status.SeverityWarn, "autoupdate.failed", t.failed(rec)
		}
		page.Entries = append(page.Entries, e)
		page.Next = e.ID
	}
	return page, nil
}
