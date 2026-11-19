package fortress

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/backup"
	"github.com/junkerderprovinz/shiplog/internal/notify"
)

// joined lets dir join a fortress the way the plugin does, with the kit's
// fortress join.
func joined(t *testing.T, dir string, lan bool) {
	t.Helper()
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	m := kitfortress.Membership{App: "shiplog", Device: "Tower", Relay: "off", LAN: lan}
	if err := kitfortress.Join(dir, secret, m); err != nil {
		t.Fatal(err)
	}
}

func TestJoinWithoutAMembershipIsInNoFortress(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		m, err := Join(dir, status.App{ID: "shiplog", Name: "ShipLog"}, "", nil, nil)
		if err != nil || m != nil {
			t.Fatalf("Join(%q) = %v, %v, want no fortress", dir, m, err)
		}
	}
}

func TestJoinTakesPartWithTheKeptMembership(t *testing.T) {
	dir := t.TempDir()
	joined(t, dir, false)
	m, err := Join(dir, status.App{ID: "shiplog", Name: "ShipLog", Version: "3.8.0"}, "", nil, nil)
	if err != nil || m == nil {
		t.Fatalf("Join = %v, %v", m, err)
	}
	defer func() { _ = m.Close() }()
	if m.Device != "Tower" {
		t.Errorf("device = %q, want the name it joined with", m.Device)
	}
	if got := m.Fortress.Members(); len(got) != 0 {
		t.Errorf("members off the network = %v, want none", got)
	}
}

func TestJoinTakesPartOnThisNetworkAndLeavesIt(t *testing.T) {
	dir := t.TempDir()
	joined(t, dir, true)
	m, err := Join(dir, status.App{ID: "shiplog", Name: "ShipLog"}, "", nil, nil)
	if err != nil || m == nil {
		t.Fatalf("Join = %v, %v", m, err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
}

func TestJoinRefusesADamagedMembership(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fortress.json"), []byte(`{"seed":"zz"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if m, err := Join(dir, status.App{ID: "shiplog"}, "", nil, nil); err == nil {
		_ = m.Close()
		t.Fatal("want an error")
	}
}

func TestShipLogCallsTheBackupAndTheNotificationHub(t *testing.T) {
	for _, kept := range [][]string{nil, {backup.Action}, {backup.Action, notify.DeliverAction}} {
		got := uses(kept)
		if len(got) != 2 || !slices.Contains(got, backup.Action) || !slices.Contains(got, notify.DeliverAction) {
			t.Errorf("uses(%v) = %v, want the backup and the hub once each", kept, got)
		}
	}
}
