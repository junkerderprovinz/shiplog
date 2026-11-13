package fortress

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
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
	// ErrNoMember means no other member of the fortress has the id.
	ErrNoMember = errors.New("no other member of the fortress has that id")
)

// removedFile sits next to the membership and names the members ShipLog took
// out since the phrase last changed.
const removedFile = "removed.json"

// removedHereFile is all that stays of a membership another member ended, so
// the settings page still says so after a restart.
const removedHereFile = "removed-here"

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
	// Host is the server as every app on it names it, so the others find
	// ShipLog there under any device name.
	Host string
	// Relay is "" for the project's relay, "off" or the address of an own one.
	Relay string

	mu            sync.Mutex
	m             *Membership
	removed       bool
	phraseChanged bool
	// following lets one follow run at a time without holding mu, so the
	// settings page is not kept waiting.
	following sync.Mutex
	// gen counts the memberships opened, so a removal that arrives late does
	// not end one joined after it.
	gen int
}

// State is how ShipLog stands in its fortress. Removed says another member
// took it out, until it joins again. PhraseChanged says the fortress has a
// new phrase ShipLog could not follow on its own, until the new words are
// entered.
type State struct {
	Joined        bool   `json:"joined"`
	Removed       bool   `json:"removed"`
	PhraseChanged bool   `json:"phraseChanged"`
	Device        string `json:"device,omitempty"`
	// Members are the other members, RemovedMembers those ShipLog took out
	// since the phrase last changed, which the phrase still lets back in.
	Members        []Other   `json:"members"`
	RemovedMembers []Removed `json:"removedMembers"`
}

// Other is another member as the settings page lists it. Form is how it
// runs, its deployment: plugin, container, desktop or android. Relay says it
// is in reach through the relay alone.
type Other struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	App      string    `json:"app"`
	Name     string    `json:"name"`
	Device   string    `json:"device"`
	Form     string    `json:"form,omitempty"`
	Version  string    `json:"version,omitempty"`
	Icon     string    `json:"icon,omitempty"`
	Online   bool      `json:"online"`
	Relay    bool      `json:"relay"`
	LastSeen time.Time `json:"lastSeen,omitzero"`
}

// MemberStatus is what another member says about itself: its status report
// and its newest log lines, the newest first.
type MemberStatus struct {
	Report status.Report     `json:"report"`
	Log    []status.LogEntry `json:"log"`
}

// Removed is a member ShipLog took out of the fortress.
type Removed struct {
	App    string `json:"app"`
	Name   string `json:"name"`
	Device string `json:"device"`
}

// followWait is how long a member that answered on the old phrase gets to
// hand over the new keys.
var followWait = 10 * time.Second

// Open takes part in the fortress kept in Dir, if ShipLog joined one.
func (d *Door) Open() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, err := d.join()
	if err != nil {
		return err
	}
	if m == nil {
		_, err := os.Stat(filepath.Join(d.Dir, removedHereFile))
		d.removed = err == nil
		return nil
	}
	d.m, d.Device = m, m.Device
	return nil
}

// Enter joins the fortress of words and opens the membership. After a
// phrase change ShipLog could not follow, the new words keep the member key
// in Dir, so the others know it as before.
func (d *Door) Enter(words string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := os.Stat(filepath.Join(d.Dir, "fortress.json")); !d.phraseChanged && !errors.Is(err, fs.ErrNotExist) {
		return ErrMember
	}
	secret, err := phrase.Decode(words)
	if err != nil {
		return errors.Join(ErrPhrase, err)
	}
	defer clear(secret)
	// The kit opens the membership once while joining, and its state file
	// takes one writer at a time.
	if d.m != nil {
		_ = d.m.Close()
		d.m = nil
	}
	m := kitfortress.Membership{App: d.App.ID, Device: d.Device, Relay: d.Relay, LAN: true, Uses: []string{backup.Action}}
	joinErr := kitfortress.Join(d.Dir, secret, m)
	in, err := d.join()
	if err != nil || joinErr != nil {
		d.m = in
		return errors.Join(joinErr, err)
	}
	d.m, d.removed, d.phraseChanged = in, false, false
	if err := os.Remove(filepath.Join(d.Dir, removedHereFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return d.forgetRemoved()
}

// State says how ShipLog stands in its fortress now.
func (d *Door) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := State{Joined: d.m != nil, Removed: d.removed, PhraseChanged: d.phraseChanged,
		Members: []Other{}, RemovedMembers: d.removedList()}
	if d.m == nil {
		return s
	}
	s.Device = d.m.Device
	s.Members = others(d.m.Fortress.Members())
	return s
}

// others lists the members the settings page shows: the other instances of
// an app. One that never said which app it runs, such as a helper that is
// online only while it works, would be a row without a name, and a viewer
// only reads, so both stay off.
func others(members []kitfortress.Member) []Other {
	out := []Other{}
	for _, m := range members {
		if m.App.ID == "" || m.Kind == member.KindViewer {
			continue
		}
		out = append(out, Other{ID: m.ID, Kind: string(m.Kind), App: m.App.ID, Name: AppName(m.App),
			Device: m.Device, Form: m.Deployment, Version: m.App.Version, Icon: m.App.Icon,
			Online: m.Online(), Relay: m.Relay && !m.Direct, LastSeen: m.LastSeen})
	}
	return out
}

// Leave takes ShipLog out of its fortress and forgets the membership. The
// members in reach drop it at once, the others when they hear it from them,
// and the phrase brings it back.
func (d *Door) Leave() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m == nil {
		return ErrNotMember
	}
	d.m.Fortress.Leave()
	err := d.m.Close()
	d.m = nil
	d.removed, d.phraseChanged = false, false
	return errors.Join(err, os.RemoveAll(d.Dir))
}

// statusLines is how many log lines the settings page shows of a member.
const statusLines = 10

// Status asks member id, another member of the fortress, how it is doing,
// its texts in lang.
func (d *Door) Status(ctx context.Context, id, lang string) (MemberStatus, error) {
	f := d.current()
	if f == nil {
		return MemberStatus{}, ErrNotMember
	}
	if !slices.ContainsFunc(others(f.Members()), func(m Other) bool { return m.ID == id }) {
		return MemberStatus{}, ErrNoMember
	}
	r, err := f.Status(ctx, id, lang)
	if err != nil {
		return MemberStatus{}, err
	}
	return MemberStatus{Report: r, Log: newestLines(ctx, f, id, lang)}, nil
}

// newestLines reads member id's log, which pages from its oldest entry, to
// the end and keeps the last lines, the newest first. A log that cannot be
// read leaves the lines out and the report stands.
func newestLines(ctx context.Context, f *kitfortress.Fortress, id, lang string) []status.LogEntry {
	tail := []status.LogEntry{}
	q := status.LogQuery{Limit: status.MaxLogLimit, Lang: lang}
	for {
		p, err := f.Log(ctx, id, q)
		if err != nil {
			break
		}
		tail = append(tail, p.Entries...)
		tail = tail[max(0, len(tail)-statusLines):]
		if len(p.Entries) < q.Limit || p.Next == q.After {
			break
		}
		q.After = p.Next
	}
	slices.Reverse(tail)
	return tail
}

// Remove takes member id out of the fortress, here and at every member in
// reach, and keeps its name until the phrase changes. ShipLog is no other
// member of its own fortress, so it cannot remove itself.
func (d *Door) Remove(id string) error {
	f := d.current()
	if f == nil {
		return ErrNotMember
	}
	for _, m := range others(f.Members()) {
		if m.ID != id {
			continue
		}
		if err := f.Remove(id); err != nil {
			return err
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		list := append(d.removedList(), Removed{App: m.App, Name: m.Name, Device: m.Device})
		raw, err := json.Marshal(list)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(d.Dir, removedFile), raw)
	}
	return ErrNoMember
}

// removedList reads the members ShipLog took out. A list that cannot be read
// names nobody rather than keep the settings page from showing the fortress.
func (d *Door) removedList() []Removed {
	var read []Removed
	raw, err := os.ReadFile(filepath.Join(d.Dir, removedFile))
	if err != nil || json.Unmarshal(raw, &read) != nil {
		return []Removed{}
	}
	list := []Removed{}
	for _, r := range read {
		if r.App != "" {
			list = append(list, r)
		}
	}
	return list
}

// forgetRemoved drops the list once the phrase changed: the members on it
// do not know the new one.
func (d *Door) forgetRemoved() error {
	err := os.Remove(filepath.Join(d.Dir, removedFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
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
// out. A member that answers on the phrase the fortress left hands over the
// new keys, and the action runs once more under them.
func (d *Door) Act(ctx context.Context, id, action string, body []byte) ([]byte, error) {
	f := d.current()
	if f == nil {
		return nil, ErrNotMember
	}
	out, err := f.Act(ctx, id, action, body)
	var no *kitfortress.Refusal
	if !errors.As(err, &no) || no.Code != "phrase_changed" {
		return out, err
	}
	moved := d.follow(ctx, f, true)
	if moved == nil {
		return out, err
	}
	reach(ctx, moved, id)
	return moved.Act(ctx, id, action, body)
}

// Watch follows a phrase change now and then, so that ShipLog, which calls
// the others only around an update, still comes along while the old phrase
// answers. It returns when ctx ends.
func (d *Door) Watch(ctx context.Context) {
	t := time.NewTimer(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if f := d.current(); f != nil {
			d.follow(ctx, f, false)
		}
		t.Reset(15 * time.Minute)
	}
}

// follow takes the membership in Dir to the fortress's new keys and opens it
// again, returning the fortress under them, or nil where none came. f is the
// fortress that ran into the old phrase, and signalled says a member answered
// so. The kit asks that member what it is only after its answer, so the keys
// may take a moment to be on offer.
func (d *Door) follow(ctx context.Context, f *kitfortress.Fortress, signalled bool) *kitfortress.Fortress {
	d.following.Lock()
	defer d.following.Unlock()
	if cur := d.current(); cur != f {
		return cur
	}
	deadline := time.Now().Add(followWait)
	moved, err := kitfortress.Follow(ctx, d.Dir, f)
	for signalled && !moved && err == nil && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-time.After(200 * time.Millisecond):
			moved, err = kitfortress.Follow(ctx, d.Dir, f)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m == nil {
		return nil
	}
	if d.m.Fortress != f {
		return d.m.Fortress
	}
	if !moved {
		if signalled || err != nil {
			d.phraseChanged = true
		}
		return nil
	}
	_ = d.m.Close()
	_ = d.forgetRemoved()
	in, err := d.join()
	d.m = in
	if err != nil || in == nil {
		d.phraseChanged = true
		return nil
	}
	d.phraseChanged = false
	return in.Fortress
}

// reach waits until member id is online in f, which has just opened, for as
// long as followWait.
func reach(ctx context.Context, f *kitfortress.Fortress, id string) {
	ctx, cancel := context.WithTimeout(ctx, followWait)
	defer cancel()
	for {
		for _, m := range f.Members() {
			if m.ID == id && m.Online() {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
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
	return Join(d.Dir, d.App, d.Host, d.Src, func() { d.removedBy(gen) }, d.Actions...)
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
		if os.MkdirAll(d.Dir, 0o700) == nil {
			_ = os.WriteFile(filepath.Join(d.Dir, removedHereFile), nil, 0o600)
		}
		d.removed, d.phraseChanged = true, false
	}()
}

// writeFile replaces path with raw, so that a crash leaves the old list or the
// new one. CreateTemp makes the file readable by its owner alone.
func writeFile(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".removed-*")
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
	return os.Rename(tmp.Name(), path)
}
