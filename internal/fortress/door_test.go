package fortress

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/relay"
	"github.com/junkerderprovinz/parleyport/kit/status"
)

var shiplog = status.App{ID: "shiplog", Name: "ShipLog", Version: "3.8.0"}

type memStore struct {
	mu  sync.Mutex
	raw []byte
}

func (m *memStore) Load() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.raw), nil
}

func (m *memStore) Save(raw []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.raw = slices.Clone(raw)
	return nil
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// vault opens a backup app on the relay at url, in the fortress of secret.
func vault(t *testing.T, url string, secret []byte) *kitfortress.Fortress {
	t.Helper()
	ident, err := member.NewIdentity(member.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	f, err := kitfortress.Open(kitfortress.Config{
		App: kitfortress.App{ID: "bombvault", Name: "BombVault", Version: "1.0.0"}, Secret: secret, Identity: ident,
		Self: group.Self{Name: "Tower", Deployment: "container"}, Relay: group.RelayOwn, RelayURL: url, Store: &memStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestThePhraseLetsShipLogIn(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fortress")
	_, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	d := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: "off"}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter("abandon abandon abandon"); !errors.Is(err, ErrPhrase) {
		t.Errorf("a mistyped phrase: %v", err)
	}
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	if s := d.State(); !s.Joined || s.Device != "Tower" {
		t.Errorf("state = %+v", s)
	}
	if err := d.Enter(w); !errors.Is(err, ErrMember) {
		t.Errorf("a second join: %v", err)
	}
}

func TestOutsideAFortressShipLogReachesNobody(t *testing.T) {
	d := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: shiplog, Device: "Tower"}
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	if s := d.State(); s.Joined || s.Removed {
		t.Errorf("state = %+v", s)
	}
	if got := d.Members(); len(got) != 0 {
		t.Errorf("members = %v", got)
	}
	if _, err := d.Act(context.Background(), "m", "backup.request", nil); !errors.Is(err, ErrNotMember) {
		t.Errorf("act = %v", err)
	}
}

func TestADoorKeepsTheDeviceTheMembershipJoinedWith(t *testing.T) {
	dir := t.TempDir()
	joined(t, dir, false)
	d := &Door{Dir: dir, App: shiplog, Device: "tower.local"}
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if s := d.State(); !s.Joined || s.Device != "Tower" || d.Device != "Tower" {
		t.Errorf("state = %+v, device %q", s, d.Device)
	}
}

func TestRemovedByAnotherMemberShipLogForgetsTheFortressAndSaysSoAfterARestart(t *testing.T) {
	srv := httptest.NewServer(relay.New())
	t.Cleanup(srv.CloseClientConnections)
	t.Cleanup(srv.Close)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	bv := vault(t, srv.URL, secret)

	dir := filepath.Join(t.TempDir(), "fortress")
	d := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: srv.URL}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ShipLog to see BombVault", func() bool {
		for _, m := range d.Members() {
			if m.App.ID == "bombvault" && m.Online() {
				return true
			}
		}
		return false
	})
	var self string
	waitFor(t, "BombVault to see ShipLog", func() bool {
		for _, m := range bv.Members() {
			if m.App.ID == "shiplog" {
				self = m.ID
				return true
			}
		}
		return false
	})
	if err := bv.Remove(self); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ShipLog to leave", func() bool { return d.State().Removed })
	if s := d.State(); s.Joined {
		t.Errorf("state = %+v", s)
	}
	if got := d.Members(); len(got) != 0 {
		t.Errorf("a removed ShipLog still lists %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "fortress.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a removed ShipLog kept its membership: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	again := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: srv.URL}
	t.Cleanup(func() { _ = again.Close() })
	if err := again.Open(); err != nil {
		t.Fatal(err)
	}
	if s := again.State(); s.Joined || !s.Removed {
		t.Errorf("after a restart: %+v", s)
	}
	if err := again.Enter(w); err != nil {
		t.Fatalf("joining again: %v", err)
	}
	if s := again.State(); !s.Joined || s.Removed {
		t.Errorf("joined again: %+v", s)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	last := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: srv.URL}
	t.Cleanup(func() { _ = last.Close() })
	if err := last.Open(); err != nil {
		t.Fatal(err)
	}
	if s := last.State(); !s.Joined || s.Removed {
		t.Errorf("a restart after joining again: %+v", s)
	}
}

func TestTheOtherMembersSeeShipLogsLogoAndServer(t *testing.T) {
	srv := httptest.NewServer(relay.New())
	t.Cleanup(srv.CloseClientConnections)
	t.Cleanup(srv.Close)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	bv := vault(t, srv.URL, secret)
	icon, err := status.IconFromSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><circle cx="4" cy="4" r="3"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	app := shiplog
	app.Icon = icon
	d := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: app, Device: "Shipyard", Host: "Tower", Relay: srv.URL}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "BombVault to see ShipLog's logo on Tower", func() bool {
		for _, m := range bv.Members() {
			if m.App.ID == "shiplog" && m.App.Icon == icon && m.Host == "Tower" {
				return true
			}
		}
		return false
	})
}
