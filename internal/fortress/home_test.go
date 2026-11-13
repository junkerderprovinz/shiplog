package fortress

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"testing"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
)

// testNetwork points the fortress's announces at a group of the test's own,
// or skips the test on a host that cannot join one.
func testNetwork(t *testing.T) {
	t.Helper()
	c, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.UDPAddr).AddrPort().Port()
	_ = c.Close()
	group := netip.AddrPortFrom(netip.MustParseAddr("239.255.77.251"), port)
	m, err := net.ListenMulticastUDP("udp4", nil, net.UDPAddrFromAddrPort(group))
	if err != nil {
		t.Skipf("this host cannot join a multicast group: %v", err)
	}
	_ = m.Close()
	was := kitfortress.Multicast
	kitfortress.Multicast = group
	t.Cleanup(func() { kitfortress.Multicast = was })
}

// homeVault opens a backup app that takes no relay and answers direct calls
// on this network.
func homeVault(t *testing.T, secret []byte) {
	t.Helper()
	ident, err := member.NewIdentity(member.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	l := lan.New(lan.Config{Group: kitfortress.Multicast})
	l.Start()
	t.Cleanup(func() { _ = l.Close() })
	f, err := kitfortress.Open(kitfortress.Config{
		App: kitfortress.App{ID: "bombvault", Name: "BombVault", Version: "1.0.0"}, Secret: secret, Identity: ident,
		Self:  group.Self{Name: "Tower", Deployment: "container", DirectURL: srv.URL},
		Relay: group.RelayOff, LAN: l, Store: &memStore{},
		Actions: []kitfortress.Action{{Name: "backup.request", Run: func(context.Context, kitfortress.Caller, []byte) ([]byte, error) {
			return []byte(`{"ok":true}`), nil
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	mux.Handle(group.DirectPath, f.DirectHandler())
}

func TestWithoutARelayShipLogGetsItsBackupFromBombVaultOnThisNetwork(t *testing.T) {
	testNetwork(t)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	homeVault(t, secret)
	d := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: shiplog, Device: "Tower", Relay: "off"}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	var vault string
	waitFor(t, "ShipLog to find BombVault on the network", func() bool {
		for _, m := range d.Members() {
			if m.App.ID == "bombvault" && m.Direct {
				vault = m.ID
				return true
			}
		}
		return false
	})
	out, err := d.Act(context.Background(), vault, "backup.request", []byte(`{}`))
	if err != nil || string(out) != `{"ok":true}` {
		t.Fatalf("asking for a backup = %s, %v", out, err)
	}
}
