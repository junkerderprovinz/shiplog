package fortress

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

var now = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

type fakeStore struct {
	rows []model.UpdateStatus
	log  []store.AutoUpdateRecord
}

func (f *fakeStore) List() ([]model.UpdateStatus, error) { return f.rows, nil }

func (f *fakeStore) LatestAutoUpdates() ([]store.AutoUpdateRecord, error) {
	last := map[string]store.AutoUpdateRecord{}
	var order []string
	for _, r := range f.log {
		if _, ok := last[r.Name]; !ok {
			order = append(order, r.Name)
		}
		last[r.Name] = r
	}
	out := make([]store.AutoUpdateRecord, 0, len(order))
	for _, n := range order {
		out = append(out, last[n])
	}
	return out, nil
}

func (f *fakeStore) AutoUpdatesAfter(after int64, limit int) ([]store.AutoUpdateRecord, error) {
	var out []store.AutoUpdateRecord
	for _, r := range f.log {
		if r.ID > after && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) logged(recs ...store.AutoUpdateRecord) *fakeStore {
	for _, r := range recs {
		r.ID = int64(len(f.log) + 1)
		f.log = append(f.log, r)
	}
	return f
}

func container(name, tag, newest string, kind model.Kind, risk model.RiskLevel) model.UpdateStatus {
	return model.UpdateStatus{
		Container: model.Container{ID: name + "-id", Name: name, Tag: tag},
		NewestTag: newest, Kind: kind, Risk: risk,
	}
}

func source(st Store) *Source {
	return &Source{
		App:      status.App{ID: "shiplog", Name: "ShipLog", Version: "3.8.0"},
		Instance: status.Instance{ID: "abc", Name: "Tower"},
		Store:    st,
		Run:      &Run{},
		Now:      func() time.Time { return now },
	}
}

func report(t *testing.T, s *Source, lang string) status.Report {
	t.Helper()
	r, err := s.Report(context.Background(), lang)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func need(r status.Report, id string) (status.Need, bool) {
	for _, n := range r.NeedsYou {
		if n.ID == id {
			return n, true
		}
	}
	return status.Need{}, false
}

func TestReportCountsTheWaitingUpdates(t *testing.T) {
	s := source(&fakeStore{rows: []model.UpdateStatus{
		container("plex", "1.0", "1.1", model.KindMinor, model.RiskMedium),
		container("radarr", "5.1", "5.1.1", model.KindPatch, model.RiskLow),
		container("redis", "7", "7", model.KindNone, model.RiskNone),
	}})
	r := report(t, s, "en")
	n, ok := need(r, "updates")
	if !ok {
		t.Fatalf("needsYou = %+v, want an item for the waiting updates", r.NeedsYou)
	}
	if n.Severity != status.SeverityInfo || n.Code != "updates.available" || n.Text != "2 containers have an update" {
		t.Errorf("updates = %+v", n)
	}
	if len(r.NeedsYou) != 1 {
		t.Errorf("needsYou = %+v, want only the waiting updates", r.NeedsYou)
	}
	if r.State != status.StateOK || r.App.ID != "shiplog" || r.Instance.Name != "Tower" {
		t.Errorf("report = %+v", r)
	}
}

func TestReportSaysNothingWhileEverythingIsCurrent(t *testing.T) {
	s := source(&fakeStore{rows: []model.UpdateStatus{container("redis", "7", "7", model.KindNone, model.RiskNone)}})
	if r := report(t, s, "en"); len(r.NeedsYou) != 0 || len(r.Work) != 0 {
		t.Errorf("report = %+v, want nothing to do", r)
	}
}

func TestReportNamesOneUpdateInTheSingular(t *testing.T) {
	s := source(&fakeStore{rows: []model.UpdateStatus{container("plex", "1.0", "1.1", model.KindMinor, model.RiskLow)}})
	r := report(t, s, "de")
	if n, _ := need(r, "updates"); n.Text != "1 Container hat ein Update" {
		t.Errorf("text = %q", n.Text)
	}
}

func TestReportFlagsRiskyJumps(t *testing.T) {
	major := container("nextcloud", "27.1.4", "28.0.0", model.KindMajor, model.RiskHigh)
	major.Container.Source = "https://github.com/nextcloud/server"
	breaking := container("immich", "1.90", "1.91", model.KindMinor, model.RiskCritical)
	s := source(&fakeStore{rows: []model.UpdateStatus{major, breaking,
		container("plex", "1.0", "1.1", model.KindMinor, model.RiskMedium)}})
	r := report(t, s, "en")

	n, ok := need(r, "risky-nextcloud")
	if !ok || n.Severity != status.SeverityWarn || n.Code != "update.major" ||
		n.Text != "nextcloud: 27.1.4 → 28.0.0 is a major jump" || n.Link != "https://github.com/nextcloud/server" {
		t.Errorf("major jump = %+v", n)
	}
	n, ok = need(r, "risky-immich")
	if !ok || n.Severity != status.SeverityWarn || n.Code != "update.breaking" ||
		n.Text != "immich: 1.90 → 1.91, the release notes name a breaking change" {
		t.Errorf("breaking change = %+v", n)
	}
	if _, ok := need(r, "risky-plex"); ok {
		t.Error("a medium risk update is flagged as risky")
	}
}

func TestReportKeepsAFailedAutoUpdateWhileTheUpdateWaits(t *testing.T) {
	st := (&fakeStore{rows: []model.UpdateStatus{container("radarr", "5.1", "5.2", model.KindMinor, model.RiskMedium)}}).
		logged(store.AutoUpdateRecord{Name: "radarr", ToVer: "5.2", Err: "still on the old image after the update", At: now.Add(-time.Hour).Unix()})
	r := report(t, source(st), "en")
	n, ok := need(r, "autoupdate-failed-radarr")
	if !ok {
		t.Fatalf("needsYou = %+v, want the failed auto-update", r.NeedsYou)
	}
	if n.Severity != status.SeverityWarn || n.Code != "autoupdate.failed" ||
		n.Text != "radarr: the auto-update to 5.2 failed (still on the old image after the update)" ||
		!n.Since.Equal(now.Add(-time.Hour)) {
		t.Errorf("failed auto-update = %+v", n)
	}
}

func TestReportDropsAFailedAutoUpdateOnceItIsSettled(t *testing.T) {
	current := container("radarr", "5.2", "5.2", model.KindNone, model.RiskNone)
	failed := store.AutoUpdateRecord{Name: "radarr", ToVer: "5.2", Err: "pull error", At: now.Add(-time.Hour).Unix()}
	tests := map[string]*fakeStore{
		"updated by hand": (&fakeStore{rows: []model.UpdateStatus{current}}).logged(failed),
		"a later run succeeded": (&fakeStore{rows: []model.UpdateStatus{container("radarr", "5.1", "5.2", model.KindMinor, model.RiskLow)}}).
			logged(failed, store.AutoUpdateRecord{Name: "radarr", Success: true, At: now.Unix()}),
		"gone for a week": (&fakeStore{}).logged(store.AutoUpdateRecord{Name: "radarr", Err: "container not found", At: now.Add(-8 * 24 * time.Hour).Unix()}),
	}
	for name, st := range tests {
		if _, ok := need(report(t, source(st), "en"), "autoupdate-failed-radarr"); ok {
			t.Errorf("%s: the failure is still reported", name)
		}
	}
}

func TestReportKeepsAContainerTheAutoUpdateLost(t *testing.T) {
	st := (&fakeStore{}).logged(store.AutoUpdateRecord{Name: "radarr", Err: "container gone after the update and could not be recreated", At: now.Add(-time.Hour).Unix()})
	n, ok := need(report(t, source(st), "en"), "autoupdate-failed-radarr")
	if !ok || n.Text != "radarr: the auto-update failed (container gone after the update and could not be recreated)" {
		t.Errorf("lost container = %+v, %v", n, ok)
	}
}

func TestReportShowsTheRunningAutoUpdate(t *testing.T) {
	s := source(&fakeStore{})
	s.Run.Begin(now.Add(-time.Minute))
	s.Run.Updating("plex")

	r := report(t, s, "en")
	if len(r.Work) != 1 {
		t.Fatalf("work = %+v, want the auto-update", r.Work)
	}
	w := r.Work[0]
	if w.ID != "autoupdate" || w.Kind != status.KindUpdate || !w.BlocksRestart || w.Title != "Auto-update: plex" || !w.StartedAt.Equal(now.Add(-time.Minute)) {
		t.Errorf("work = %+v", w)
	}
	if de := report(t, s, "de"); de.Work[0].Title != "Automatisches Update: plex" {
		t.Errorf("German title = %q", de.Work[0].Title)
	}

	s.Run.End()
	if r := report(t, s, "en"); len(r.Work) != 0 {
		t.Errorf("work after the run = %+v", r.Work)
	}
}

func TestReportAnswersInGermanForAnyGermanTag(t *testing.T) {
	s := source(&fakeStore{rows: []model.UpdateStatus{container("plex", "1.0", "1.1", model.KindMinor, model.RiskLow)}})
	for _, lang := range []string{"de", "de-AT", "DE-ch"} {
		if n, _ := need(report(t, s, lang), "updates"); !strings.HasPrefix(n.Text, "1 Container hat") {
			t.Errorf("lang %s: %q", lang, n.Text)
		}
	}
	for _, lang := range []string{"", "fr", "dk"} {
		if n, _ := need(report(t, s, lang), "updates"); n.Text != "1 container has an update" {
			t.Errorf("lang %q: %q, want English", lang, n.Text)
		}
	}
}

func TestLogPagesThroughTheAutoUpdatesOldestFirst(t *testing.T) {
	st := (&fakeStore{}).logged(
		store.AutoUpdateRecord{Name: "plex", FromVer: "1.0", ToVer: "1.1", Success: true, At: now.Add(-2 * time.Hour).Unix()},
		store.AutoUpdateRecord{Name: "radarr", ToVer: "5.2", Err: "pull error", At: now.Add(-time.Hour).Unix()},
		store.AutoUpdateRecord{Name: "sonarr", Success: true, At: now.Unix()},
	)
	s := source(st)
	page, err := s.Log(context.Background(), status.LogQuery{Limit: 2, Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 2 || page.Next != "2" {
		t.Fatalf("first page = %+v", page)
	}
	if e := page.Entries[0]; e.ID != "1" || e.Level != status.SeverityInfo || e.Code != "autoupdate.applied" ||
		e.Text != "plex updated from 1.0 to 1.1" || !e.At.Equal(now.Add(-2*time.Hour)) {
		t.Errorf("first entry = %+v", e)
	}
	if e := page.Entries[1]; e.Level != status.SeverityWarn || e.Code != "autoupdate.failed" ||
		e.Text != "radarr: the auto-update to 5.2 failed (pull error)" {
		t.Errorf("second entry = %+v", e)
	}

	page, _ = s.Log(context.Background(), status.LogQuery{After: page.Next, Limit: 2, Lang: "de"})
	if len(page.Entries) != 1 || page.Entries[0].Text != "sonarr aktualisiert" || page.Next != "3" {
		t.Fatalf("second page = %+v", page)
	}
	page, _ = s.Log(context.Background(), status.LogQuery{After: "3", Limit: 2})
	if len(page.Entries) != 0 || page.Next != "3" {
		t.Errorf("past the end = %+v, want no entries and the same cursor", page)
	}
}

func TestLogStartsOverOnACursorItDidNotHandOut(t *testing.T) {
	st := (&fakeStore{}).logged(store.AutoUpdateRecord{Name: "plex", Success: true, At: now.Unix()})
	page, err := source(st).Log(context.Background(), status.LogQuery{After: "nonsense", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 1 || page.Next != strconv.Itoa(1) {
		t.Errorf("page = %+v, want the log from its start", page)
	}
}
