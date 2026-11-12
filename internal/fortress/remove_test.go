package fortress

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
)

// plugin is another plugin's instance in the fortress.
type plugin struct {
	f  *kitfortress.Fortress
	id string
}

// tollRoll opens a TollRoll on the relay at url, in the fortress of secret.
func tollRoll(t *testing.T, url string, secret []byte) plugin {
	t.Helper()
	ident, err := member.NewIdentity(member.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	f, err := kitfortress.Open(kitfortress.Config{
		App: kitfortress.App{ID: "tollroll", Name: "TollRoll", Version: "1.0.0"}, Secret: secret, Identity: ident,
		Self: group.Self{Name: "Bottich", Deployment: "plugin"}, Relay: group.RelayOwn, RelayURL: url, Store: &memStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return plugin{f: f, id: ident.ID()}
}

// joinedWithTollRoll has ShipLog join the fortress of a backup app on a
// relay of its own and waits until it reaches a TollRoll there.
func joinedWithTollRoll(t *testing.T) (*Door, *backupApp, plugin, []byte) {
	t.Helper()
	url := relayURL(t)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	bv := newBackupApp(t, url, secret, nil)
	tr := tollRoll(t, url, secret)
	d := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: shiplog, Src: source(&fakeStore{}), Device: "Tower", Relay: url}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ShipLog to reach TollRoll", func() bool {
		for _, m := range d.Members() {
			if m.ID == tr.id && m.Online() && m.App.Name == "TollRoll" {
				return true
			}
		}
		return false
	})
	return d, bv, tr, secret
}

func TestShipLogRemovesAMemberThatStaysOutAfterARestart(t *testing.T) {
	d, _, tr, _ := joinedWithTollRoll(t)
	self := memberID(t, d.Dir)
	if _, err := tr.f.Status(context.Background(), self, "en"); err != nil {
		t.Fatalf("TollRoll cannot read ShipLog before the removal: %v", err)
	}

	if err := d.Remove(tr.id); err != nil {
		t.Fatal(err)
	}
	want := []Removed{{App: "tollroll", Name: "TollRoll", Device: "Bottich"}}
	if got := d.State().RemovedMembers; !slices.Equal(got, want) {
		t.Fatalf("removed = %+v, want %+v", got, want)
	}
	info, err := os.Stat(filepath.Join(d.Dir, removedFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm&0o077 != 0 {
		t.Errorf("the removed list is readable by others: %v", perm)
	}

	// ShipLog starts again and still names TollRoll and still refuses it.
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	next := &Door{Dir: d.Dir, App: shiplog, Src: d.Src, Device: "Tower", Relay: d.Relay}
	t.Cleanup(func() { _ = next.Close() })
	if err := next.Open(); err != nil {
		t.Fatal(err)
	}
	if got := next.State().RemovedMembers; !slices.Equal(got, want) {
		t.Fatalf("removed after a restart = %+v, want %+v", got, want)
	}
	time.Sleep(2 * time.Second)
	for _, m := range next.Members() {
		if m.ID == tr.id {
			t.Fatal("TollRoll is a member again after the restart")
		}
	}
	if _, err := tr.f.Status(context.Background(), self, "en"); err == nil {
		t.Fatal("ShipLog answers the removed TollRoll after a restart")
	}
}

func TestShipLogRemovesNobodyItDoesNotKnow(t *testing.T) {
	out := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: shiplog, Device: "Tower"}
	if err := out.Remove("abc"); !errors.Is(err, ErrNotMember) {
		t.Errorf("removing outside a fortress = %v, want ErrNotMember", err)
	}
	d, _, _, _ := joinedWithTollRoll(t)
	for _, id := range []string{"nobody", memberID(t, d.Dir)} {
		if err := d.Remove(id); !errors.Is(err, ErrNoMember) {
			t.Errorf("removing %q = %v, want ErrNoMember", id, err)
		}
	}
	if got := d.State().RemovedMembers; len(got) != 0 {
		t.Errorf("removed = %+v, want none", got)
	}
}

func TestANewPhraseClearsTheRemovedList(t *testing.T) {
	d, bv, tr, secret := joinedWithTollRoll(t)
	if err := d.Remove(tr.id); err != nil {
		t.Fatal(err)
	}
	// ShipLog is away while the phrase changes.
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	bv.changePhrase(secret)
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ShipLog to reach BombVault on the old phrase", func() bool { return seesOnline(d, bv.id.ID()) })
	if _, err := d.Act(context.Background(), bv.id.ID(), "backup.request", []byte(`{}`)); err != nil {
		t.Fatalf("act after the change: %v", err)
	}
	if got := d.State().RemovedMembers; len(got) != 0 {
		t.Fatalf("removed after the phrase changed = %+v, want none", got)
	}
	if _, err := os.Stat(filepath.Join(d.Dir, removedFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the removed list outlived the phrase: %v", err)
	}
}

func TestTheStateListsTheOtherMembers(t *testing.T) {
	d, _, tr, _ := joinedWithTollRoll(t)
	want := []string{"BombVault on Tower as container", "TollRoll on Bottich as plugin"}
	var names []string
	waitFor(t, "ShipLog to list BombVault and TollRoll by name", func() bool {
		names = names[:0]
		for _, m := range d.State().Members {
			names = append(names, m.Name+" on "+m.Device+" as "+m.Form)
		}
		slices.Sort(names)
		return slices.Equal(names, want)
	})
	for _, m := range d.State().Members {
		if m.ID == tr.id && !m.Online {
			t.Error("TollRoll is listed as away")
		}
	}
}

func TestAMemberThatNeverSaidItsAppStaysOffTheCard(t *testing.T) {
	got := others([]kitfortress.Member{
		{ID: "a", Kind: member.KindInstance, App: kitfortress.App{ID: "tollroll", Name: "TollRoll"}, Device: "Bottich", Deployment: "plugin"},
		{ID: "b", Kind: member.KindInstance, Device: "Bottich", Deployment: "plugin"},
	})
	if len(got) != 1 || got[0].ID != "a" || got[0].Name != "TollRoll" {
		t.Errorf("listed = %+v, want TollRoll alone", got)
	}
}

func TestTheCardNeverCarriesAMembersAddress(t *testing.T) {
	got := others([]kitfortress.Member{{ID: "b", Kind: member.KindInstance, App: kitfortress.App{ID: "bombvault", Name: "BombVault"},
		Device: "Bottich", Deployment: "container", Address: "http://bottich:3000"}})
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(raw); strings.Contains(s, "address") || strings.Contains(s, "bottich:3000") {
		t.Errorf("the card gets the address: %s", s)
	}
	if len(got) != 1 || got[0].Kind != "instance" || got[0].Form != "container" || got[0].Device != "Bottich" {
		t.Errorf("the card lost what names the app that keeps the phrase: %+v", got)
	}
}

func TestTheRemovedListNamesNobodyWithoutAnApp(t *testing.T) {
	d := &Door{Dir: t.TempDir(), App: shiplog, Device: "Tower"}
	raw := `[{"app":"tollroll","name":"TollRoll","device":"Bottich"},{"app":"","name":"","device":"Bottich"}]`
	if err := os.WriteFile(filepath.Join(d.Dir, removedFile), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []Removed{{App: "tollroll", Name: "TollRoll", Device: "Bottich"}}
	if got := d.State().RemovedMembers; !slices.Equal(got, want) {
		t.Errorf("removed = %+v, want %+v", got, want)
	}
}
