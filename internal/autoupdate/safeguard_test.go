package autoupdate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/backup"
	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

type fakeBackup struct {
	err   error
	snap  backup.Snapshot
	saved []string
	// updatesSeen is how many updates the host had run at each backup.
	updatesSeen []int
	h           *fakeHost
}

func (b *fakeBackup) Save(_ context.Context, name string) (backup.Snapshot, error) {
	b.saved = append(b.saved, name)
	b.updatesSeen = append(b.updatesSeen, b.h.updatedAt)
	return b.snap, b.err
}

type fakeImages struct {
	held     []string
	released []string
}

func (i *fakeImages) Hold(_ context.Context, image, name string) error {
	i.held = append(i.held, image+" "+name)
	return nil
}

func (i *fakeImages) Release(_ context.Context, name string) error {
	i.released = append(i.released, name)
	return nil
}

// fakeTemplates is the template directory. Setting an image pulls it on the
// next update, as Unraid's script does, unless ignored.
type fakeTemplates struct {
	repo    map[string]string
	h       *fakeHost
	ignored bool
}

func (t *fakeTemplates) Repository(name string) (string, error) {
	r, ok := t.repo[name]
	if !ok {
		return "", errors.New("no template")
	}
	return r, nil
}

func (t *fakeTemplates) SetRepository(name, ref string) error {
	t.repo[name] = ref
	if _, digest, ok := strings.Cut(ref, "@"); ok && !t.ignored {
		t.h.pulled[name] = []string{digest}
	}
	return nil
}

type fakeGuards struct {
	m       map[string]store.Guard
	history []string
}

func (g *fakeGuards) SetGuard(gd store.Guard) error {
	g.m[gd.Name] = gd
	g.history = append(g.history, gd.Name+" "+gd.State)
	return nil
}

func (g *fakeGuards) DeleteGuard(name string) error {
	delete(g.m, name)
	g.history = append(g.history, name+" deleted")
	return nil
}

type guardedHost struct {
	*fakeHost
	backup    *fakeBackup
	images    *fakeImages
	templates *fakeTemplates
	guards    *fakeGuards
	safe      *Safeguard
}

const (
	oldDigest = "sha256:a"
	newDigest = "sha256:b"
)

// newGuardedHost runs nextcloud on the old image, with a template that names
// its tag and a backup app that answers with a snapshot.
func newGuardedHost() *guardedHost {
	h := newHost(map[string]string{"nextcloud": oldDigest})
	c := h.live["nextcloud"]
	c.ImageID = "sha256:1111"
	h.live["nextcloud"] = c
	h.pulled["nextcloud"] = []string{newDigest}
	g := &guardedHost{
		fakeHost:  h,
		backup:    &fakeBackup{h: h, snap: backup.Snapshot{Member: "m1", Run: "7", ID: "4f2a9c1e", Link: "https://tower.lan/snap/4f2a9c1e"}},
		images:    &fakeImages{},
		templates: &fakeTemplates{h: h, repo: map[string]string{"nextcloud": "lscr.io/linuxserver/nextcloud:latest"}},
		guards:    &fakeGuards{m: map[string]store.Guard{}},
	}
	g.safe = &Safeguard{Backup: g.backup, Images: g.images, Templates: g.templates, Guards: g.guards}
	return g
}

func (g *guardedHost) run() Outcome {
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{stWithDigest("nextcloud", newDigest)}}, g.fakeHost).
		WithDocker(g.fakeHost).WithSafeguard(g.safe)
	e.sleep = g.sleep
	return e.Run(context.Background(), Policy{Digest: true}, false).Outcomes[0]
}

func TestSafeguardWithoutABackupAppUpdatesAsBefore(t *testing.T) {
	g := newGuardedHost()
	g.backup.err = backup.ErrNoMember
	o := g.run()
	if !o.Updated || o.Err != nil || o.Snapshot.ID != "" {
		t.Fatalf("outcome = %+v, want a plain update", o)
	}
	if len(g.images.held) != 0 || len(g.guards.history) != 0 || g.templates.repo["nextcloud"] != "lscr.io/linuxserver/nextcloud:latest" {
		t.Errorf("held %v, guards %v, template %q; want all untouched", g.images.held, g.guards.history, g.templates.repo["nextcloud"])
	}
	plain := newHost(map[string]string{"nextcloud": oldDigest})
	plain.pulled["nextcloud"] = []string{newDigest}
	runHost(plain, false, stWithDigest("nextcloud", newDigest))
	if g.slept != plain.slept {
		t.Errorf("waited %s, want the %s an update waits without a fortress", g.slept, plain.slept)
	}
}

func TestSafeguardBacksUpAndHoldsTheOldImageBeforeTheUpdate(t *testing.T) {
	g := newGuardedHost()
	o := g.run()
	if !o.Updated || o.Err != nil {
		t.Fatalf("outcome = %+v, want updated", o)
	}
	if len(g.backup.saved) != 1 || g.backup.updatesSeen[0] != 0 {
		t.Errorf("backups %v with %v updates run before, want one before the update", g.backup.saved, g.backup.updatesSeen)
	}
	if o.Snapshot.ID != "4f2a9c1e" || o.Snapshot.Link != "https://tower.lan/snap/4f2a9c1e" {
		t.Errorf("snapshot = %+v", o.Snapshot)
	}
	if len(g.images.held) != 1 || g.images.held[0] != "sha256:1111 nextcloud" {
		t.Errorf("held = %v, want the old image of nextcloud", g.images.held)
	}
	if want := "nextcloud updating,nextcloud deleted"; strings.Join(g.guards.history, ",") != want {
		t.Errorf("guards = %v, want %s", g.guards.history, want)
	}
	if len(g.images.released) != 1 {
		t.Errorf("released = %v, want the old image let go after the update", g.images.released)
	}
}

func TestSafeguardKeepsTheOldImageInTheGuardWhileUpdating(t *testing.T) {
	g := newGuardedHost()
	var during store.Guard
	g.backup.snap.ID = "s1"
	g.safe.Guards = guardSpy{g.guards, func(gd store.Guard) {
		if gd.State == store.GuardUpdating {
			during = gd
		}
	}}
	g.run()
	if during.Repository != "lscr.io/linuxserver/nextcloud:latest" || during.Digest != oldDigest || during.Snapshot != "s1" {
		t.Fatalf("guard while updating = %+v, want the template image, the old digest and the snapshot", during)
	}
}

type guardSpy struct {
	*fakeGuards
	seen func(store.Guard)
}

func (s guardSpy) SetGuard(gd store.Guard) error {
	s.seen(gd)
	return s.fakeGuards.SetGuard(gd)
}

func TestSafeguardHoldsTheUpdateBackWhenTheBackupFails(t *testing.T) {
	g := newGuardedHost()
	g.backup.err = errors.New("the backup failed: repository locked")
	o := g.run()
	if o.Updated || !o.NoBackup || o.Err == nil || !strings.Contains(o.Err.Error(), "repository locked") {
		t.Fatalf("outcome = %+v, want held back with the reason", o)
	}
	if len(g.calls) != 0 {
		t.Errorf("update ran for %v, want no update without a backup", g.calls)
	}
	if gd := g.guards.m["nextcloud"]; gd.State != store.GuardNoBackup || !strings.Contains(gd.Reason, "repository locked") {
		t.Errorf("guard = %+v, want no_backup with the reason", gd)
	}
}

func TestSafeguardUpdatesWithoutABackupWhenTheContainerIsSwitchedTo(t *testing.T) {
	g := newGuardedHost()
	g.backup.err = errors.New("the backup app did not answer")
	g.safe.WithoutBackup = []string{"NextCloud"}
	o := g.run()
	if !o.Updated || !o.WithoutBackup || o.Err != nil {
		t.Fatalf("outcome = %+v, want updated without a backup", o)
	}
	if len(g.images.held) != 1 {
		t.Errorf("held = %v, want the old image held all the same", g.images.held)
	}
}

func TestSafeguardRollsBackAnUnhealthyUpdate(t *testing.T) {
	g := newGuardedHost()
	g.health = map[string]string{"nextcloud": "unhealthy"}
	o := g.run()
	if o.Updated || !o.RolledBack || o.Err == nil || !strings.Contains(o.Err.Error(), "unhealthy") {
		t.Fatalf("outcome = %+v, want rolled back after an unhealthy update", o)
	}
	if got := g.templates.repo["nextcloud"]; got != "lscr.io/linuxserver/nextcloud:latest@"+oldDigest {
		t.Errorf("template = %q, want the tag pinned to the old digest", got)
	}
	if c := g.live["nextcloud"]; c.Digest != oldDigest || c.State != "running" {
		t.Errorf("container = %+v, want running on the old image", c)
	}
	if gd := g.guards.m["nextcloud"]; gd.State != store.GuardRolledBack || gd.Snapshot != "4f2a9c1e" {
		t.Errorf("guard = %+v, want rolled_back with the snapshot", gd)
	}
	if len(g.images.released) != 1 {
		t.Errorf("released = %v, want the hold let go once the old image runs", g.images.released)
	}
}

func TestSafeguardRollsBackAnUpdateToAnUnexpectedImage(t *testing.T) {
	g := newGuardedHost()
	g.pulled["nextcloud"] = []string{"sha256:z"}
	o := g.run()
	if !o.RolledBack || g.live["nextcloud"].Digest != oldDigest {
		t.Fatalf("outcome = %+v on %s, want rolled back to the old image", o, g.live["nextcloud"].Digest)
	}
}

func TestSafeguardRollsBackAContainerThatStopsAfterTheUpdate(t *testing.T) {
	g := newGuardedHost()
	g.onSleep = func() { g.setState("nextcloud", "restarting") }
	o := g.run()
	if !o.RolledBack {
		t.Fatalf("outcome = %+v, want rolled back", o)
	}
}

func TestSafeguardWaitsForAHealthcheckThatIsStarting(t *testing.T) {
	g := newGuardedHost()
	g.health = map[string]string{"nextcloud": "starting"}
	g.onSleep = func() {
		c := g.live["nextcloud"]
		c.Health = "healthy"
		g.live["nextcloud"] = c
	}
	o := g.run()
	if !o.Updated || o.RolledBack {
		t.Fatalf("outcome = %+v, want updated once healthy", o)
	}
}

func TestSafeguardReportsARollbackThatFailed(t *testing.T) {
	g := newGuardedHost()
	g.health = map[string]string{"nextcloud": "unhealthy"}
	g.templates.ignored = true
	o := g.run()
	if o.Updated || o.RolledBack || o.Err == nil || !strings.Contains(o.Err.Error(), "going back to the old image failed") {
		t.Fatalf("outcome = %+v, want a failed rollback reported", o)
	}
	if gd := g.guards.m["nextcloud"]; gd.State != store.GuardUpdating {
		t.Errorf("guard = %+v, want it kept with the old image", gd)
	}
	if len(g.images.released) != 0 {
		t.Errorf("released = %v, want the old image kept for a manual rollback", g.images.released)
	}
}

func TestSafeguardLeavesAContainerThatStayedOnItsOldImage(t *testing.T) {
	g := newGuardedHost()
	g.stale["nextcloud"] = true
	o := g.run()
	if !errors.Is(o.Err, ErrNoChange) || o.RolledBack {
		t.Fatalf("outcome = %+v, want ErrNoChange and no rollback", o)
	}
	if g.templates.repo["nextcloud"] != "lscr.io/linuxserver/nextcloud:latest" || len(g.guards.m) != 0 || len(g.images.released) != 1 {
		t.Errorf("template %q, guards %v, released %v; want all as before", g.templates.repo["nextcloud"], g.guards.m, g.images.released)
	}
}

func TestPinReplacesAnExistingDigest(t *testing.T) {
	for in, want := range map[string]string{
		"nginx":                    "nginx@sha256:a",
		"nginx:1.27":               "nginx:1.27@sha256:a",
		"nginx:1.27@sha256:b":      "nginx:1.27@sha256:a",
		"registry:5000/app:latest": "registry:5000/app:latest@sha256:a",
	} {
		if got := pin(in, "sha256:a"); got != want {
			t.Errorf("pin(%q) = %q, want %q", in, got, want)
		}
	}
}
