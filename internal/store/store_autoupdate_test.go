package store_test

import (
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/store"
)

func TestAutoUpdateLog(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.LogAutoUpdate(store.AutoUpdateRecord{Name: "plex", FromVer: "1.2.3", ToVer: "1.2.4", Level: "patch", Success: true, At: 100}); err != nil {
		t.Fatal(err)
	}
	if err := s.LogAutoUpdate(store.AutoUpdateRecord{Name: "radarr", Level: "minor", Success: false, Err: "pull error", At: 200}); err != nil {
		t.Fatal(err)
	}

	hist, err := s.AutoUpdateHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("want 2 records, got %d", len(hist))
	}
	if hist[0].Name != "radarr" || hist[0].Success || hist[0].Err != "pull error" || hist[0].Level != "minor" {
		t.Fatalf("newest record wrong: %+v", hist[0])
	}
	if hist[1].Name != "plex" || !hist[1].Success || hist[1].FromVer != "1.2.3" || hist[1].ToVer != "1.2.4" {
		t.Fatalf("second record wrong: %+v", hist[1])
	}
}

func TestMetaRoundTrip(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if v, err := s.GetMeta("autoupdate_last_run"); err != nil || v != "" {
		t.Fatalf("absent key: got (%q, %v), want (\"\", nil)", v, err)
	}
	if err := s.SetMeta("autoupdate_last_run", "1700000000"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.GetMeta("autoupdate_last_run"); err != nil || v != "1700000000" {
		t.Fatalf("after set: got (%q, %v)", v, err)
	}
	if err := s.SetMeta("autoupdate_last_run", "1700009999"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetMeta("autoupdate_last_run"); v != "1700009999" {
		t.Fatalf("upsert did not overwrite: got %q", v)
	}
}

func openAutoUpdates(t *testing.T, recs ...store.AutoUpdateRecord) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, r := range recs {
		if err := s.LogAutoUpdate(r); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestAutoUpdatesAfterPagesOldestFirst(t *testing.T) {
	s := openAutoUpdates(t,
		store.AutoUpdateRecord{Name: "plex", Success: true, At: 300},
		store.AutoUpdateRecord{Name: "radarr", Success: false, Err: "pull error", At: 100},
		store.AutoUpdateRecord{Name: "sonarr", Success: true, At: 200},
	)
	first, err := s.AutoUpdatesAfter(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].Name != "plex" || first[1].Name != "radarr" {
		t.Fatalf("first page = %+v, want plex and radarr in the order they were logged", first)
	}
	if first[0].ID >= first[1].ID {
		t.Fatalf("ids do not rise: %d then %d", first[0].ID, first[1].ID)
	}
	rest, err := s.AutoUpdatesAfter(first[1].ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 1 || rest[0].Name != "sonarr" || rest[0].At != 200 {
		t.Fatalf("second page = %+v, want sonarr alone", rest)
	}
	if none, _ := s.AutoUpdatesAfter(rest[0].ID, 2); len(none) != 0 {
		t.Fatalf("after the last entry = %+v, want nothing", none)
	}
}

func TestLatestAutoUpdatesKeepsTheLastTryOfEachContainer(t *testing.T) {
	s := openAutoUpdates(t,
		store.AutoUpdateRecord{Name: "plex", Success: false, Err: "pull error", At: 100},
		store.AutoUpdateRecord{Name: "radarr", Success: false, Err: "gone", At: 150},
		store.AutoUpdateRecord{Name: "plex", Success: true, ToVer: "1.2.4", At: 200},
	)
	got, err := s.LatestAutoUpdates()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]store.AutoUpdateRecord{}
	for _, r := range got {
		byName[r.Name] = r
	}
	if len(got) != 2 || !byName["plex"].Success || byName["plex"].ToVer != "1.2.4" || byName["radarr"].Err != "gone" {
		t.Fatalf("latest = %+v, want plex's success and radarr's failure", got)
	}
}
