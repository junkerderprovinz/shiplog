package fortress

import (
	"errors"
	"io/fs"
	"slices"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
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
// kept in dir, as app, answers the other members about ShipLog's status from
// src and offers them actions. removed runs once another member takes ShipLog
// out. It returns nil when nobody joined there.
func Join(dir string, app status.App, src status.Source, removed func(), actions ...kitfortress.Action) (*Membership, error) {
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
	cfg.App = kitfortress.App{ID: app.ID, Name: app.Name, Version: app.Version}
	cfg.Status = src
	cfg.Actions = append(cfg.Actions, actions...)
	cfg.OnRemoved = removed
	if !slices.Contains(cfg.Uses, backup.Action) {
		cfg.Uses = append(cfg.Uses, backup.Action)
	}
	if cfg.LAN != nil {
		cfg.LAN.Start()
	}
	f, err := kitfortress.Open(cfg)
	if err != nil {
		if cfg.LAN != nil {
			_ = cfg.LAN.Close()
		}
		return nil, err
	}
	m := &Membership{Fortress: f, Device: cfg.Self.Name, close: f.Close}
	if cfg.LAN != nil {
		m.close = func() error { return errors.Join(f.Close(), cfg.LAN.Close()) }
	}
	return m, nil
}
