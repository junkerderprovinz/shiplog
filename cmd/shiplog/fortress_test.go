package main

import (
	"path/filepath"
	"testing"

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
