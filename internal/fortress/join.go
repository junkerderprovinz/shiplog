package fortress

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/keys"
	"github.com/junkerderprovinz/parleyport/kit/lan"
	"github.com/junkerderprovinz/parleyport/kit/member"
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

// membership is what the kit's "fortress join" keeps in its directory: the
// member key and the fortress's own keys, never the phrase.
type membership struct {
	Name     string `json:"name"`
	Relay    string `json:"relay"`
	LAN      bool   `json:"lan"`
	Seed     string `json:"seed"`
	Group    string `json:"group"`
	Frame    string `json:"frame"`
	PeerAuth string `json:"peerAuth"`
}

// Join takes part in the fortress kept in dir, as app on the device it joined
// with, and answers the other members about ShipLog's status from src. It
// returns nil when dir keeps no membership.
func Join(dir string, app status.App, src status.Source) (*Membership, error) {
	if dir == "" {
		return nil, nil
	}
	config := filepath.Join(dir, "fortress.json")
	raw, err := os.ReadFile(config)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s membership
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s is damaged: %w", config, err)
	}
	id, set, err := s.identity()
	if err != nil {
		return nil, fmt.Errorf("%s is damaged: %w", config, err)
	}
	cfg := kitfortress.Config{
		App:      kitfortress.App{ID: app.ID, Name: app.Name, Version: app.Version},
		Keys:     set,
		Identity: id,
		Self:     group.Self{Name: s.Name, Deployment: "plugin"},
		Store:    fileStore(filepath.Join(dir, "state.json")),
		Status:   src,
		Uses:     []string{backup.Action},
	}
	switch s.Relay {
	case "":
		cfg.Relay = group.RelayProject
	case "off":
		cfg.Relay = group.RelayOff
	default:
		cfg.Relay, cfg.RelayURL = group.RelayOwn, s.Relay
	}
	if s.LAN {
		cfg.LAN = lan.New(lan.Config{Group: kitfortress.Multicast})
		cfg.LAN.Start()
	}
	f, err := kitfortress.Open(cfg)
	if err != nil {
		if cfg.LAN != nil {
			_ = cfg.LAN.Close()
		}
		return nil, err
	}
	m := &Membership{Fortress: f, Device: s.Name, close: f.Close}
	if cfg.LAN != nil {
		m.close = func() error { return errors.Join(f.Close(), cfg.LAN.Close()) }
	}
	return m, nil
}

func (s membership) identity() (member.Identity, keys.Set, error) {
	seed, err := hex.DecodeString(s.Seed)
	if err != nil {
		return member.Identity{}, keys.Set{}, err
	}
	id, err := member.FromSeed(member.KindInstance, seed)
	if err != nil {
		return member.Identity{}, keys.Set{}, err
	}
	frame, err := hex.DecodeString(s.Frame)
	if err != nil {
		return member.Identity{}, keys.Set{}, err
	}
	peerAuth, err := hex.DecodeString(s.PeerAuth)
	if err != nil {
		return member.Identity{}, keys.Set{}, err
	}
	set := keys.Set{Group: s.Group, Frame: frame, PeerAuth: peerAuth}
	if !set.Complete() {
		return member.Identity{}, keys.Set{}, errors.New("the keys are incomplete")
	}
	return id, set, nil
}

// fileStore keeps the fortress's state next to the membership. A crash leaves
// the old file or the new one, never half of either.
type fileStore string

func (p fileStore) Load() ([]byte, error) {
	raw, err := os.ReadFile(string(p))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}

func (p fileStore) Save(raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(string(p)), ".state-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), string(p))
}
