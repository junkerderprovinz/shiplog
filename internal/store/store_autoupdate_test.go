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
	// newest first: at=200 (radarr), a failure with its reason preserved.
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

	// an absent key returns "" and no error (so a first-ever start reads no last run).
	if v, err := s.GetMeta("autoupdate_last_run"); err != nil || v != "" {
		t.Fatalf("absent key: got (%q, %v), want (\"\", nil)", v, err)
	}
	if err := s.SetMeta("autoupdate_last_run", "1700000000"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.GetMeta("autoupdate_last_run"); err != nil || v != "1700000000" {
		t.Fatalf("after set: got (%q, %v)", v, err)
	}
	// upsert overwrites in place, not a second row.
	if err := s.SetMeta("autoupdate_last_run", "1700009999"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetMeta("autoupdate_last_run"); v != "1700009999" {
		t.Fatalf("upsert did not overwrite: got %q", v)
	}
}
