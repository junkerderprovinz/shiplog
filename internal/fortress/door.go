package fortress

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/backup"
)

var (
	// ErrPhrase means the words are not a fortress phrase.
	ErrPhrase = errors.New("that is not a fortress phrase")
	// ErrMember means ShipLog is in a fortress already.
	ErrMember = errors.New("ShipLog is in a fortress already")
	// ErrNotMember means ShipLog is in no fortress.
	ErrNotMember = errors.New("ShipLog is in no fortress")
)

// Door holds ShipLog's place in a fortress: it opens the membership kept in
// Dir, joins with a phrase the way the kit's fortress join does on the
// console, and forgets the membership once another member takes ShipLog out.
// The phrase itself is never kept.
type Door struct {
	Dir     string
	App     status.App
	Src     status.Source
	Actions []kitfortress.Action
	// Device is the server's name in the fortress. Open replaces it with the
	// name a kept membership joined with, so joining again keeps it.
	Device string
	// Relay is "" for the project's relay, "off" or the address of an own one.
	Relay string

	mu      sync.Mutex
	m       *Membership
	removed bool
	// gen counts the memberships opened, so a removal that arrives late does
	// not end one joined after it.
	gen int
}

// State is how ShipLog stands in its fortress. Removed says another member
// took it out, until it joins again.
type State struct {
	Joined  bool   `json:"joined"`
	Removed bool   `json:"removed"`
	Device  string `json:"device,omitempty"`
}

// Open takes part in the fortress kept in Dir, if ShipLog joined one.
func (d *Door) Open() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, err := d.join()
	if err != nil || m == nil {
		return err
	}
	d.m, d.Device = m, m.Device
	return nil
}

// Enter joins the fortress of words and opens the membership.
func (d *Door) Enter(words string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := os.Stat(filepath.Join(d.Dir, "fortress.json")); !errors.Is(err, fs.ErrNotExist) {
		return ErrMember
	}
	secret, err := phrase.Decode(words)
	if err != nil {
		return errors.Join(ErrPhrase, err)
	}
	defer clear(secret)
	m := kitfortress.Membership{App: d.App.ID, Device: d.Device, Relay: d.Relay, Uses: []string{backup.Action}}
	if err := kitfortress.Join(d.Dir, secret, m); err != nil {
		return err
	}
	in, err := d.join()
	if err != nil {
		return err
	}
	d.m, d.removed = in, false
	return nil
}

// State says how ShipLog stands in its fortress now.
func (d *Door) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := State{Joined: d.m != nil, Removed: d.removed}
	if d.m != nil {
		s.Device = d.m.Device
	}
	return s
}

// Members lists the other members, none while ShipLog is out.
func (d *Door) Members() []kitfortress.Member {
	f := d.current()
	if f == nil {
		return nil
	}
	return f.Members()
}

// Act runs action at member id, or fails with ErrNotMember while ShipLog is
// out.
func (d *Door) Act(ctx context.Context, id, action string, body []byte) ([]byte, error) {
	f := d.current()
	if f == nil {
		return nil, ErrNotMember
	}
	return f.Act(ctx, id, action, body)
}

// Close stops taking part and keeps the membership for the next start.
func (d *Door) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m == nil {
		return nil
	}
	err := d.m.Close()
	d.m = nil
	return err
}

func (d *Door) current() *kitfortress.Fortress {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m == nil {
		return nil
	}
	return d.m.Fortress
}

func (d *Door) join() (*Membership, error) {
	d.gen++
	gen := d.gen
	return Join(d.Dir, d.App, d.Src, func() { d.removedBy(gen) }, d.Actions...)
}

// removedBy forgets the membership once another member took it out: its key
// is no good anywhere now, and the phrase makes a new one. The kit calls it on
// a goroutine that closing the membership waits for, so the closing happens
// on another.
func (d *Door) removedBy(gen int) {
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.gen != gen || d.m == nil {
			return
		}
		_ = d.m.Close()
		d.m = nil
		_ = os.RemoveAll(d.Dir)
		d.removed = true
	}()
}
