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
	t.Cleanup(func() { _ = s.Close() })
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

func TestPinnedAndLocalRoundTrip(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container: model.Container{
			ID: "pin", Name: "pinned", Repo: "ghcr.io/x/y",
			PinnedDigest: "sha256:aaa", IsLocal: true,
		},
		Kind: model.KindNone, Risk: model.RiskNone, CheckedAt: time.Now(),
	}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("pin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Container.PinnedDigest != "sha256:aaa" {
		t.Errorf("pinned_digest did not round-trip: %q", got.Container.PinnedDigest)
	}
	if !got.Container.IsLocal {
		t.Error("is_local did not round-trip")
	}
}

// Managed must round-trip through Get()/List() — every consumer of the served
// status (the JSON API, the status page) reads it back from the store, not
// from the in-memory struct the sweep computed it into. (This column existed
// on the model for several releases without ever being wired into the
// schema/selectCols/scanStatus at all, which is exactly this bug: every
// served container read back managed=false regardless of its real
// net.unraid.docker.managed label.)
func TestManagedRoundTrip(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container: model.Container{ID: "mgd", Name: "teamspeak", Repo: "docker.io/binhex/arch-teamspeak", Managed: true},
		Kind:      model.KindNone, Risk: model.RiskNone, CheckedAt: time.Now(),
	}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("mgd")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Container.Managed {
		t.Error("managed did not round-trip")
	}
}

// Unmaintained/UnmaintainedReason and CADeprecated/CADeprecatedNote must round-trip
// through Get() — engine.maybeNotifyUnmaintained's dedup ("if prior.Unmaintained {
// return }") only ever sees a real value via Store.Get, never the in-memory struct
// directly, so a silent gap here means a container flagged unmaintained gets a fresh
// notification on EVERY sweep forever instead of once on the transition into the
// state. (These columns existed on the schema and the model for several releases
// without ever being wired into selectCols/scanStatus, which is exactly this bug.)
func TestUnmaintainedAndCADeprecatedRoundTrip(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container: model.Container{ID: "flagged", Name: "handbrake", Repo: "coppit/handbrake"},
		Kind:      model.KindNone, Risk: model.RiskNone, CheckedAt: time.Now(),
		Unmaintained:       true,
		UnmaintainedReason: "Removed from Community Applications",
		CADeprecated:       true,
		CADeprecatedNote:   "A better supported and more up to date app is available from DJoss",
	}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("flagged")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Unmaintained {
		t.Error("Unmaintained did not round-trip (this is the notify-spam bug: prior.Unmaintained would always read false)")
	}
	if got.UnmaintainedReason != "Removed from Community Applications" {
		t.Errorf("UnmaintainedReason did not round-trip: %q", got.UnmaintainedReason)
	}
	if !got.CADeprecated {
		t.Error("CADeprecated did not round-trip")
	}
	if got.CADeprecatedNote != "A better supported and more up to date app is available from DJoss" {
		t.Errorf("CADeprecatedNote did not round-trip: %q", got.CADeprecatedNote)
	}

	// A healthy row must round-trip as explicitly false/empty, not just "unset".
	st2 := model.UpdateStatus{Container: model.Container{ID: "fine", Name: "fine"}, Kind: model.KindNone, Risk: model.RiskNone, CheckedAt: time.Now()}
	if err := s.Upsert(st2); err != nil {
		t.Fatal(err)
	}
	got2, err := s.Get("fine")
	if err != nil {
		t.Fatal(err)
	}
	if got2.Unmaintained || got2.CADeprecated {
		t.Errorf("healthy row must not read back flagged: unmaintained=%v ca_deprecated=%v", got2.Unmaintained, got2.CADeprecated)
	}
}

func TestHistoryAppendOnVersionChange(t *testing.T) {
	s := newTestStore(t)
	base := model.UpdateStatus{Container: model.Container{ID: "abc", Name: "immich", Tag: "1.122.0"}, RunningVersion: "1.122.0", CheckedAt: time.Now()}
	_ = s.Upsert(base)
	base.Container.Tag = "1.124.2"
	base.RunningVersion = "1.124.2"
	_ = s.Upsert(base)
	h, err := s.History("abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) < 1 {
		t.Fatal("expected a history entry on running-version change")
	}
	if h[0].FromTag != "1.122.0" || h[0].ToTag != "1.124.2" {
		t.Fatalf("history span wrong: got %q -> %q", h[0].FromTag, h[0].ToTag)
	}
}

func TestRunningVersionRoundTrip(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container:      model.Container{ID: "oh", Name: "openhands", Tag: "latest"},
		RunningVersion: "1.8.0", NewestTag: "1.8.0", CheckedAt: time.Now(),
	}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("oh")
	if err != nil {
		t.Fatal(err)
	}
	if got.RunningVersion != "1.8.0" {
		t.Fatalf("running_version did not round-trip: got %q", got.RunningVersion)
	}
}

// For a :latest container the tag never changes, so history must key off the
// remembered running version — and must NOT record a row the first time we
// learn it (prior running version empty).
func TestHistoryOnLatestRunningVersionChange(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{Container: model.Container{ID: "oh", Name: "openhands", Tag: "latest"}, CheckedAt: time.Now()}

	// First sight: version unknown → no history.
	_ = s.Upsert(st)
	// We learn the version (e.g. running == :latest) → still no history (just learning).
	st.RunningVersion = "1.7.0"
	_ = s.Upsert(st)
	if h, _ := s.History("oh"); len(h) != 0 {
		t.Fatalf("learning the first version must not create history, got %d rows", len(h))
	}
	// User updates: 1.7.0 -> 1.8.0, tag still "latest" → one history row.
	st.RunningVersion = "1.8.0"
	_ = s.Upsert(st)
	h, _ := s.History("oh")
	if len(h) != 1 {
		t.Fatalf("expected one history row on :latest version change, got %d", len(h))
	}
	if h[0].FromTag != "1.7.0" || h[0].ToTag != "1.8.0" {
		t.Fatalf("history span wrong: got %q -> %q", h[0].FromTag, h[0].ToTag)
	}
}

// A row migrated from a DB that predates running_version has the column
// backfilled NULL. Upsert must tolerate that (COALESCE) instead of erroring on
// the NULL->string scan and aborting the whole update.
func TestUpsertToleratesNullRunningVersion(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{Container: model.Container{ID: "old", Name: "legacy", Tag: "latest"}, RunningVersion: "1.0.0", CheckedAt: time.Now()}
	if err := s.Upsert(st); err != nil {
		t.Fatal(err)
	}
	// Simulate the post-migration state: existing row with running_version NULL.
	if _, err := s.db.Exec(`UPDATE status SET running_version = NULL WHERE container_id = 'old'`); err != nil {
		t.Fatal(err)
	}
	st.RunningVersion = "1.1.0"
	if err := s.Upsert(st); err != nil {
		t.Fatalf("upsert must tolerate a NULL prior running_version, got: %v", err)
	}
	got, err := s.Get("old")
	if err != nil {
		t.Fatal(err)
	}
	if got.RunningVersion != "1.1.0" {
		t.Fatalf("row not updated after NULL prior: got %q", got.RunningVersion)
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

func TestSourceOverridesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if m, err := s.SourceOverrides(); err != nil || len(m) != 0 {
		t.Fatalf("empty: got %v, %v", m, err)
	}
	if err := s.SetSourceOverride("lscr.io/linuxserver/radarr", "https://github.com/Radarr/Radarr"); err != nil {
		t.Fatal(err)
	}
	// Upsert: a second set for the same repo replaces, not duplicates.
	if err := s.SetSourceOverride("lscr.io/linuxserver/radarr", "https://github.com/me/fork"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSourceOverride("ghcr.io/x/media-preview-generator", "https://github.com/owner/mpg"); err != nil {
		t.Fatal(err)
	}
	m, err := s.SourceOverrides()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m["lscr.io/linuxserver/radarr"] != "https://github.com/me/fork" || m["ghcr.io/x/media-preview-generator"] != "https://github.com/owner/mpg" {
		t.Fatalf("overrides = %v", m)
	}
	if err := s.DeleteSourceOverride("lscr.io/linuxserver/radarr"); err != nil {
		t.Fatal(err)
	}
	// Deleting an absent repo is a no-op, not an error.
	if err := s.DeleteSourceOverride("does/not-exist"); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
	m, _ = s.SourceOverrides()
	if len(m) != 1 || m["ghcr.io/x/media-preview-generator"] == "" {
		t.Fatalf("after delete = %v", m)
	}
}

func TestSuppressedUnmaintainedRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if m, err := s.SuppressedUnmaintained(); err != nil || len(m) != 0 {
		t.Fatalf("empty: got %v, %v", m, err)
	}
	if err := s.SuppressUnmaintained("ghcr.io/x/myapp"); err != nil {
		t.Fatal(err)
	}
	// Upsert: suppressing an already-suppressed repo is a harmless no-op.
	if err := s.SuppressUnmaintained("ghcr.io/x/myapp"); err != nil {
		t.Fatal(err)
	}
	if err := s.SuppressUnmaintained("ghcr.io/x/other"); err != nil {
		t.Fatal(err)
	}
	m, err := s.SuppressedUnmaintained()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || !m["ghcr.io/x/myapp"] || !m["ghcr.io/x/other"] {
		t.Fatalf("suppressed = %v", m)
	}
	if err := s.UnsuppressUnmaintained("ghcr.io/x/myapp"); err != nil {
		t.Fatal(err)
	}
	// Unsuppressing an absent repo is a no-op, not an error.
	if err := s.UnsuppressUnmaintained("does/not-exist"); err != nil {
		t.Fatalf("unsuppress absent: %v", err)
	}
	m, _ = s.SuppressedUnmaintained()
	if len(m) != 1 || !m["ghcr.io/x/other"] {
		t.Fatalf("after unsuppress = %v", m)
	}
}
