package store_test

import (
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/store"
)

func TestGuardsKeepOneEntryPerContainer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	updating := store.Guard{Name: "nextcloud", State: store.GuardUpdating, Repository: "lscr.io/linuxserver/nextcloud",
		Digest: "sha256:aaaa", Snapshot: "4f2a9c1e", At: 100}
	if err := s.SetGuard(updating); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGuard(store.Guard{Name: "mariadb", State: store.GuardNoBackup, Reason: "the backup failed: locked", At: 120}); err != nil {
		t.Fatal(err)
	}
	rolled := updating
	rolled.State, rolled.Reason, rolled.At = store.GuardRolledBack, "unhealthy", 200
	if err := s.SetGuard(rolled); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	// What a crash in the middle of an update leaves must survive a restart.
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	got, err := s.Guards()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "mariadb" || got[1] != rolled {
		t.Fatalf("guards = %+v, want mariadb and the rolled back nextcloud", got)
	}
	if err := s.DeleteGuard("mariadb"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Guards(); len(got) != 1 || got[0].Name != "nextcloud" {
		t.Fatalf("after delete = %+v", got)
	}
}
