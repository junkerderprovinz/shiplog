package fortress

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"slices"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/backup"
)

// Membership is ShipLog taking part in a fortress.
type Membership struct {
	Fortress *kitfortress.Fortress
	// Device is the server's name in the fortress, the one ShipLog joined with.
	Device string
	close  func() error
}

// Close leaves the fortress.
func (m *Membership) Close() error { return m.close() }

// Join takes part in the fortress whose membership the kit's fortress join
// kept in dir, as app on host, answers the other members about ShipLog's status from
// src and offers them actions. removed runs once another member takes ShipLog
// out. It returns nil when nobody joined there.
func Join(dir string, app status.App, host string, src status.Source, removed func(), actions ...kitfortress.Action) (*Membership, error) {
	if dir == "" {
		return nil, nil
	}
	cfg, err := kitfortress.Load(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.App = kitfortress.App{ID: app.ID, Name: app.Name, Version: app.Version, Icon: app.Icon}
	cfg.Host = host
	cfg.Status = src
	cfg.Actions = append(cfg.Actions, actions...)
	cfg.OnRemoved = removed
	if !slices.Contains(cfg.Uses, backup.Action) {
		cfg.Uses = append(cfg.Uses, backup.Action)
	}
	if cfg.LAN == nil {
		f, err := kitfortress.Open(cfg)
		if err != nil {
			return nil, err
		}
		return &Membership{Fortress: f, Device: cfg.Self.Name, close: f.Close}, nil
	}
	ln, err := listenDirect()
	if err != nil {
		return nil, err
	}
	if ln != nil {
		cfg.Self.DirectURL = "http://" + ln.Addr().String()
	}
	cfg.LAN.Start()
	f, err := kitfortress.Open(cfg)
	if err != nil {
		if ln != nil {
			_ = ln.Close()
		}
		return nil, errors.Join(err, cfg.LAN.Close())
	}
	m := &Membership{Fortress: f, Device: cfg.Self.Name, close: func() error { return errors.Join(f.Close(), cfg.LAN.Close()) }}
	if ln != nil {
		srv := serveDirect(ln, f)
		m.close = func() error { return errors.Join(srv.Close(), f.Close(), cfg.LAN.Close()) }
	}
	return m, nil
}

// listenDirect opens the port the other members call ShipLog on, on the
// address it announces on this network and no other. The OS picks the port,
// since the announce carries it. A host without an IPv4 address gets none and
// reaches the members through the relay alone.
func listenDirect() (net.Listener, error) {
	ip := lan.LocalIPv4()
	if ip == "" {
		return nil, nil
	}
	return net.Listen("tcp4", net.JoinHostPort(ip, "0"))
}

// serveDirect answers the members' direct calls at ln and nothing else. Every
// call is sealed and signed under the fortress's keys, so only a member gets
// past the kit's handler. The server logs nothing, since its errors name the
// caller's address.
func serveDirect(ln net.Listener, f *kitfortress.Fortress) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(group.DirectPath, f.DirectHandler())
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	go func() { _ = srv.Serve(ln) }()
	return srv
}
