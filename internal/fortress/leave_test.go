package fortress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/member"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
	"github.com/junkerderprovinz/parleyport/kit/status"
)

func TestShipLogLeavesTheFortressAndThePhraseBringsItBack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fortress")
	_, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	d := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: "off"}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Leave(); !errors.Is(err, ErrNotMember) {
		t.Errorf("leaving outside a fortress = %v, want ErrNotMember", err)
	}
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	if err := d.Leave(); err != nil {
		t.Fatal(err)
	}
	if s := d.State(); s.Joined || s.Removed || s.PhraseChanged {
		t.Errorf("state after leaving = %+v", s)
	}
	if _, err := os.Stat(filepath.Join(dir, "fortress.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ShipLog kept its membership after leaving: %v", err)
	}
	again := &Door{Dir: dir, App: shiplog, Device: "Tower", Relay: "off"}
	t.Cleanup(func() { _ = again.Close() })
	if err := again.Open(); err != nil {
		t.Fatal(err)
	}
	if s := again.State(); s.Joined || s.Removed {
		t.Errorf("after a restart = %+v", s)
	}
	if err := again.Enter(w); err != nil {
		t.Fatalf("joining again: %v", err)
	}
	if s := again.State(); !s.Joined {
		t.Errorf("joined again = %+v", s)
	}
}

func TestTheCardGetsWhatItShowsOfEachMember(t *testing.T) {
	seen := time.Date(2026, 11, 12, 21, 5, 0, 0, time.UTC)
	got := others([]kitfortress.Member{
		{ID: "a", Kind: member.KindInstance, App: kitfortress.App{ID: "tollroll", Name: "TollRoll", Version: "1.4.0", Icon: "data:image/svg+xml;base64,PHN2Zy8+"},
			Device: "Bottich", Deployment: "plugin", Relay: true},
		{ID: "b", Kind: member.KindInstance, App: kitfortress.App{ID: "bombvault", Name: "BombVault"}, Device: "Tower", Deployment: "container", Direct: true, Relay: true},
		{ID: "c", Kind: member.KindInstance, App: kitfortress.App{ID: "bombvault", Name: "BombVault"}, Device: "Laptop", Deployment: "desktop", LastSeen: seen},
		{ID: "d", Kind: member.KindViewer, App: kitfortress.App{ID: "glance", Name: "Glance"}, Device: "Tower", Direct: true},
	})
	want := []Other{
		{ID: "a", Kind: "instance", App: "tollroll", Name: "TollRoll", Device: "Bottich", Form: "plugin", Version: "1.4.0",
			Icon: "data:image/svg+xml;base64,PHN2Zy8+", Online: true, Relay: true},
		{ID: "b", Kind: "instance", App: "bombvault", Name: "BombVault", Device: "Tower", Form: "container", Online: true},
		{ID: "c", Kind: "instance", App: "bombvault", Name: "BombVault", Device: "Laptop", Form: "desktop", LastSeen: seen},
	}
	if !slices.Equal(got, want) {
		t.Errorf("members =\n%+v\nwant\n%+v", got, want)
	}
}

// logSource is a member's status with a log of n entries, the oldest first.
type logSource struct{ n int }

func (s logSource) Report(_ context.Context, lang string) (status.Report, error) {
	return status.Report{State: status.StateOK, NeedsYou: []status.Need{{ID: "x", Severity: status.SeverityWarn, Code: "x", Text: "in " + lang}}}, nil
}

func (s logSource) Log(_ context.Context, q status.LogQuery) (status.LogPage, error) {
	after, _ := strconv.Atoi(q.After)
	var p status.LogPage
	for i := after + 1; i <= s.n && len(p.Entries) < q.Limit; i++ {
		p.Entries = append(p.Entries, status.LogEntry{ID: strconv.Itoa(i), At: now.Add(time.Duration(i) * time.Minute), Level: status.SeverityInfo, Text: fmt.Sprintf("line %d in %s", i, q.Lang)})
	}
	p.Next = q.After
	if len(p.Entries) > 0 {
		p.Next = p.Entries[len(p.Entries)-1].ID
	}
	return p, nil
}

func TestShipLogReadsAnotherPluginsStatusAndItsNewestLines(t *testing.T) {
	url := relayURL(t)
	secret, w, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	ident, err := member.NewIdentity(member.KindInstance)
	if err != nil {
		t.Fatal(err)
	}
	tt, err := kitfortress.Open(kitfortress.Config{
		App: kitfortress.App{ID: "towertorch", Name: "TowerTorch", Version: "2.0.0"}, Secret: secret, Identity: ident,
		Self: group.Self{Name: "Bottich", Deployment: "plugin"}, Relay: group.RelayOwn, RelayURL: url, Store: &memStore{},
		Status: logSource{n: 230},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tt.Close() })

	out := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: shiplog, Device: "Tower"}
	if _, err := out.Status(context.Background(), ident.ID(), "de"); !errors.Is(err, ErrNotMember) {
		t.Errorf("status outside a fortress = %v, want ErrNotMember", err)
	}

	d := &Door{Dir: filepath.Join(t.TempDir(), "fortress"), App: shiplog, Src: source(&fakeStore{}), Device: "Tower", Relay: url}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Enter(w); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ShipLog to know TowerTorch by name", func() bool {
		return slices.ContainsFunc(d.State().Members, func(m Other) bool { return m.ID == ident.ID() && m.Online })
	})

	got, err := d.Status(context.Background(), ident.ID(), "de")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Report.NeedsYou) != 1 || got.Report.NeedsYou[0].Text != "in de" {
		t.Errorf("report = %+v", got.Report)
	}
	var lines []string
	for _, e := range got.Log {
		lines = append(lines, e.Text)
	}
	var want []string
	for i := 230; i > 220; i-- {
		want = append(want, fmt.Sprintf("line %d in de", i))
	}
	if !slices.Equal(lines, want) {
		t.Errorf("log = %q, want %q", lines, want)
	}
	if _, err := d.Status(context.Background(), "nobody", "de"); !errors.Is(err, ErrNoMember) {
		t.Errorf("status of a stranger = %v, want ErrNoMember", err)
	}
}
