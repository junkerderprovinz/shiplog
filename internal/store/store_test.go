package store

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/shiplog.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndList(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container: model.Container{ID: "abc", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.122.0"},
		NewestTag: "1.124.2", Kind: model.KindMinor, Risk: model.RiskMedium, CheckedAt: time.Now(),
	}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	st.Risk = model.RiskHigh
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 row, got %d", len(all))
	}
	if all[0].Risk != model.RiskHigh {
		t.Fatal("upsert did not update")
	}
}

func TestHistoryAppendOnVersionChange(t *testing.T) {
	s := newTestStore(t)
	base := model.UpdateStatus{Container: model.Container{ID: "abc", Name: "immich", Tag: "1.122.0"}, CheckedAt: time.Now()}
	_ = s.Upsert(base)
	base.Container.Tag = "1.124.2"
	_ = s.Upsert(base)
	h, err := s.History("abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) < 1 {
		t.Fatal("expected a history entry on running-version change")
	}
}

func TestChangelogRoundTrip(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container: model.Container{ID: "abc", Name: "immich", Tag: "1.122.0"},
		NewestTag: "1.124.2",
		CheckedAt: time.Now(),
		Changelog: &model.Changelog{
			FromTag: "1.122.0",
			ToTag:   "1.124.2",
			Source:  "GitHub releases via OCI label",
			Entries: []model.ReleaseEntry{{Tag: "1.124.2", Body: "fixes"}},
		},
	}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 row, got %d", len(all))
	}
	cl := all[0].Changelog
	if cl == nil {
		t.Fatal("changelog did not survive round-trip")
	}
	if cl.Source != "GitHub releases via OCI label" {
		t.Fatalf("changelog field lost: got source %q", cl.Source)
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListOrdersByRiskThenName(t *testing.T) {
	s := newTestStore(t)
	rows := []model.UpdateStatus{
		{Container: model.Container{ID: "1", Name: "zeta"}, Risk: model.RiskLow, CheckedAt: time.Now()},
		{Container: model.Container{ID: "2", Name: "alpha"}, Risk: model.RiskHigh, CheckedAt: time.Now()},
		{Container: model.Container{ID: "3", Name: "beta"}, Risk: model.RiskHigh, CheckedAt: time.Now()},
		{Container: model.Container{ID: "4", Name: "gamma"}, Risk: model.RiskMedium, CheckedAt: time.Now()},
	}
	for _, r := range rows {
		if err := s.Upsert(r); err != nil {
			t.Fatal(err)
		}
	}
	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(all))
	for i, r := range all {
		got[i] = r.Container.Name
	}
	want := []string{"alpha", "beta", "gamma", "zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrong order: got %v, want %v", got, want)
		}
	}
}
