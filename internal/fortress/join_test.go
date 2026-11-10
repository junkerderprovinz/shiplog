package fortress

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/status"
)

// joined writes dir the way the kit's "fortress join" does, off the network.
func joined(t *testing.T, dir string) {
	t.Helper()
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	set := kitfortress.Domain.Keys(secret)
	id, err := member.NewIdentity(member.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{
		"app": "shiplog", "name": "Tower", "relay": "off",
		"seed": hex.EncodeToString(id.Seed()), "group": set.Group,
		"frame": hex.EncodeToString(set.Frame), "peerAuth": hex.EncodeToString(set.PeerAuth),
	})
	if err := os.WriteFile(filepath.Join(dir, "fortress.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestJoinWithoutAMembershipIsInNoFortress(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		m, err := Join(dir, status.App{ID: "shiplog", Name: "ShipLog"}, nil)
		if err != nil || m != nil {
			t.Fatalf("Join(%q) = %v, %v, want no fortress", dir, m, err)
		}
	}
}

func TestJoinTakesPartWithTheKeptMembership(t *testing.T) {
	dir := t.TempDir()
	joined(t, dir)
	m, err := Join(dir, status.App{ID: "shiplog", Name: "ShipLog", Version: "3.8.0"}, nil)
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

func TestJoinRefusesADamagedMembership(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fortress.json"), []byte(`{"seed":"zz"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if m, err := Join(dir, status.App{ID: "shiplog"}, nil); err == nil {
		_ = m.Close()
		t.Fatal("want an error")
	}
}
