package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/fortress"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

func TestTheInstanceKeepsItsIDAcrossStarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shiplog.db")
	open := func() *store.Store {
		db, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	first := instance(db)
	_ = db.Close()
	db = open()
	defer func() { _ = db.Close() }()
	again := instance(db)
	if first.ID == "" || first.ID != again.ID {
		t.Errorf("ids %q and %q, want one id that stays", first.ID, again.ID)
	}
	if first.Name == "" {
		t.Error("the instance has no name")
	}
}

func TestShipLogShowsItsLogoInTheFortress(t *testing.T) {
	if err := status.CheckIcon(app().Icon); err != nil {
		t.Fatal(err)
	}
}

func TestShipLogFindsTheBackupAppByTheServerItRunsOn(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skip(err)
	}
	report := &fortress.Source{App: app(), Instance: status.Instance{ID: "x", Name: "Shipyard"}}
	door, client := openFortress(config.Config{FortressDir: filepath.Join(t.TempDir(), "fortress")}, report)
	if door.Host != host || client.Host != host || client.Device != "Shipyard" {
		t.Errorf("door on %q, client on %q as %q, want both on %q", door.Host, client.Host, client.Device, host)
	}
}
