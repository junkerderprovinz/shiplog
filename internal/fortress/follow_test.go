package fortress

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/relay"
)

// keeper takes the secret a phrase change hands over.
type keeper struct {
	mu     sync.Mutex
	secret []byte
}

func (k *keeper) Keep(secret []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.secret = append([]byte(nil), secret...)
	return nil
}

// backupApp is a backup app that can change the fortress phrase and answers
// backup.request with answer, or with a refusal when refuse is set.
type backupApp struct {
	t        *testing.T
	url      string
	id       member.Identity
	store    *memStore
	keeper   *keeper
	refuse   *kitfortress.Refusal
	f        *kitfortress.Fortress
	requests int
	mu       sync.Mutex
}

func newBackupApp(t *testing.T, url string, secret []byte, refuse *kitfortress.Refusal) *backupApp {
	t.Helper()
	id, err := member.NewIdentity(member.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	b := &backupApp{t: t, url: url, id: id, store: &memStore{}, keeper: &keeper{}, refuse: refuse}
	b.open(secret, nil)
	return b
}

func (b *backupApp) open(secret, previous []byte) {
	b.t.Helper()
	f, err := kitfortress.Open(kitfortress.Config{
		App: kitfortress.App{ID: "bombvault", Name: "BombVault", Version: "1.0.0"}, Secret: secret, Previous: previous, Identity: b.id,
		Self: group.Self{Name: "Tower", Deployment: "container"}, Relay: group.RelayOwn, RelayURL: b.url, Store: b.store, Keeper: b.keeper,
		Actions: []kitfortress.Action{{Name: "backup.request", Run: func(context.Context, kitfortress.Caller, []byte) ([]byte, error) {
			b.mu.Lock()
			defer b.mu.Unlock()
			b.requests++
			if b.refuse != nil {
				return nil, b.refuse
			}
			return []byte(`{"ok":true}`), nil
		}}},
	})
	if err != nil {
		b.t.Fatal(err)
	}
	b.t.Cleanup(func() { _ = f.Close() })
	b.f = f
}

// changePhrase gives the fortress a new phrase and opens the app again under
// it, answering on old for the members that hold only keys.
func (b *backupApp) changePhrase(old []byte) {
	b.t.Helper()
	if _, err := b.f.ChangePhrase(context.Background()); err != nil {
		b.t.Fatal(err)
	}
	if err := b.f.Close(); err != nil {
		b.t.Fatal(err)
	}
	b.keeper.mu.Lock()
	secret := b.keeper.secret
	b.keeper.mu.Unlock()
	b.open(secret, old)
}

func (b *backupApp) shiplogs() []string {
	var ids []string
	for _, m := range b.f.Members() {
		if m.App.ID == "shiplog" {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

func relayURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(relay.New())
	t.Cleanup(srv.CloseClientConnections)
	t.Cleanup(srv.Close)
	return srv.URL
}

func seesOnline(d *Door, id string) bool {
	for _, m := range d.Members() {
		if m.ID == id && m.Online() {
			return true
		}
	}
	return false
}

func memberID(t *testing.T, dir string) string {
	t.Helper()
	cfg, err := kitfortress.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Identity.ID()
}

func TestShipLogFollowsAPhraseChangeAndAsksAgain(t *testing.T) {
	url := relayURL(t)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	bv := newBackupApp(t, url, secret, nil)
	dir := filepath.Join(t.TempDir(), "fortress")
	d := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: url}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	self := memberID(t, dir)
	waitFor(t, "BombVault to see ShipLog", func() bool { return len(bv.shiplogs()) == 1 })

	// ShipLog is away while the phrase changes.
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	bv.changePhrase(secret)
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ShipLog to reach BombVault on the old phrase", func() bool { return seesOnline(d, bv.id.ID()) })

	out, err := d.Act(context.Background(), bv.id.ID(), "backup.request", []byte(`{}`))
	if err != nil || string(out) != `{"ok":true}` {
		t.Fatalf("act after the change = %s, %v", out, err)
	}
	if s := d.State(); !s.Joined || s.PhraseChanged {
		t.Errorf("state = %+v, want joined under the new phrase", s)
	}
	if got := memberID(t, dir); got != self {
		t.Errorf("member %s after following, want %s", got, self)
	}
	if got := bv.shiplogs(); len(got) != 1 || got[0] != self {
		t.Errorf("BombVault lists ShipLog as %v, want %s alone", got, self)
	}
}

func TestShipLogSaysThePhraseChangedWhenItCannotFollow(t *testing.T) {
	followWait = 500 * time.Millisecond
	t.Cleanup(func() { followWait = 10 * time.Second })
	url := relayURL(t)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	gone := &kitfortress.Refusal{Status: http.StatusGone, Code: "phrase_changed"}
	bv := newBackupApp(t, url, secret, gone)
	dir := filepath.Join(t.TempDir(), "fortress")
	d := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: url}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	self := memberID(t, dir)
	waitFor(t, "ShipLog to see BombVault", func() bool { return seesOnline(d, bv.id.ID()) })

	_, err = d.Act(context.Background(), bv.id.ID(), "backup.request", []byte(`{}`))
	var no *kitfortress.Refusal
	if !errors.As(err, &no) || no.Code != "phrase_changed" {
		t.Fatalf("act = %v, want the refusal", err)
	}
	bv.mu.Lock()
	asked := bv.requests
	bv.mu.Unlock()
	if asked != 1 {
		t.Errorf("BombVault was asked %d times, want once without new keys", asked)
	}
	if s := d.State(); !s.Joined || !s.PhraseChanged {
		t.Errorf("state = %+v, want the phrase change shown", s)
	}

	// The new words bring ShipLog back as the member it was.
	if err := d.Enter(w); err != nil {
		t.Fatalf("entering the words again: %v", err)
	}
	if s := d.State(); !s.Joined || s.PhraseChanged {
		t.Errorf("state after the words = %+v", s)
	}
	if got := memberID(t, dir); got != self {
		t.Errorf("member %s after the words, want %s", got, self)
	}
	if err := d.Enter(w); !errors.Is(err, ErrMember) {
		t.Errorf("joining once more: %v, want ErrMember", err)
	}
}
