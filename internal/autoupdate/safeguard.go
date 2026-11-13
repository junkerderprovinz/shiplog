package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/backup"
	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

// Backup saves a container's data before its update.
type Backup interface {
	Save(ctx context.Context, name string) (backup.Snapshot, error)
}

// Images keeps a container's old image while its update runs.
type Images interface {
	Hold(ctx context.Context, image, name string) error
	Release(ctx context.Context, name string) error
}

// Templates reads and sets the image a container's Unraid template runs.
type Templates interface {
	Repository(name string) (string, error)
	SetRepository(name, ref string) error
}

// Guards keeps what the safeguard knows across a restart.
type Guards interface {
	SetGuard(store.Guard) error
	DeleteGuard(name string) error
}

// Safeguard has the backup app of the fortress back a container up before its
// update and puts the container back on its old image when the update fails.
// Without a backup app on this server it stays out of the way, and updates
// run as they did before.
type Safeguard struct {
	Backup    Backup
	Images    Images
	Templates Templates
	Guards    Guards
	// WithoutBackup are the containers that update even when their backup
	// fails.
	WithoutBackup []string
}

// The checks after an update: a container that ran before has to keep running
// for readyGrace, and its healthcheck has up to readyTimeout to pass.
const (
	readyGrace   = 30 * time.Second
	readyTimeout = 2 * time.Minute
)

// guarded is what the safeguard knows about one update in the run.
type guarded struct {
	repository string // the template's image before the update
	digest     string
	snapshot   backup.Snapshot
	onOld      bool // the container ended up on its old image
}

// before backs the container up, unless it was backed up just before, and
// holds its image. It returns no guard when there is no backup app, and an
// error when the update has to wait.
func (s *Safeguard) before(ctx context.Context, o *Outcome, c model.Container, backedUp bool) (*guarded, error) {
	var snap backup.Snapshot
	var err error
	if !backedUp {
		snap, err = s.Backup.Save(ctx, o.Name)
	}
	switch {
	case errors.Is(err, backup.ErrNoMember):
		return nil, nil
	case err != nil && !ContainerExcluded(o.Name, s.WithoutBackup):
		_ = s.Guards.SetGuard(store.Guard{Name: o.Name, State: store.GuardNoBackup, Reason: err.Error(), At: time.Now().Unix()})
		return nil, fmt.Errorf("held back without a backup: %w", err)
	case err != nil:
		o.WithoutBackup = true
	default:
		o.Snapshot = snap
	}
	g := &guarded{digest: c.Digest, snapshot: snap}
	g.repository, _ = s.Templates.Repository(o.Name)
	// Without the hold the registry still serves the old digest in most cases.
	_ = s.Images.Hold(ctx, c.ImageID, o.Name)
	_ = s.Guards.SetGuard(g.state(o.Name, store.GuardUpdating, ""))
	return g, nil
}

func (g *guarded) state(name, state, reason string) store.Guard {
	return store.Guard{Name: name, State: state, Repository: g.repository, Digest: g.digest,
		Snapshot: g.snapshot.ID, Reason: reason, At: time.Now().Unix()}
}

// ready checks a container that ran before its update: it has to stay up for
// readyGrace and pass its healthcheck.
func (e *Executor) ready(ctx context.Context, name string, before model.Container) error {
	if before.State != "running" {
		return nil
	}
	for waited := time.Duration(0); ; waited += settlePoll {
		c, found, err := e.running(ctx, name)
		switch {
		case err != nil:
			return fmt.Errorf("could not check the container after the update: %w", err)
		case !found:
			return errors.New("container gone after the update")
		case c.Health == "unhealthy":
			return errors.New("unhealthy after the update")
		case c.State == "running" && c.Health != "starting" && waited >= readyGrace:
			return nil
		case waited >= readyTimeout:
			return fmt.Errorf("not ready %s after the update (%s)", readyTimeout, describe(c))
		}
		if err := e.sleep(ctx, settlePoll); err != nil {
			return err
		}
	}
}

func describe(c model.Container) string {
	if c.Health != "" {
		return c.State + ", " + c.Health
	}
	return c.State
}

// rollBack points the template at the image the container ran before and
// recreates the container from it. A container still on its old image needs
// nothing.
func (e *Executor) rollBack(ctx context.Context, a *applied, o *Outcome) {
	g := a.guard
	if c, found, err := e.running(ctx, o.Name); err == nil && found && c.HasDigest(g.digest) &&
		(a.before.State != "running" || c.State == "running") {
		g.onOld = true
		return
	}
	if err := e.goBack(ctx, a, o); err != nil {
		o.Err = fmt.Errorf("%w; going back to the old image failed: %v", o.Err, err)
		return
	}
	g.onOld, o.RolledBack = true, true
}

func (e *Executor) goBack(ctx context.Context, a *applied, o *Outcome) error {
	g := a.guard
	switch {
	case g.digest == "":
		return errors.New("the old image has no registry digest")
	case g.repository == "":
		return errors.New("the template names no image")
	}
	if err := e.safe.Templates.SetRepository(o.Name, pin(g.repository, g.digest)); err != nil {
		return err
	}
	out, err := e.upd.Update(ctx, o.Name)
	a.out = strings.TrimSpace(a.out + "\n" + out)
	if err != nil {
		return err
	}
	after, _, err := e.keep(ctx, a, o)
	if err != nil {
		return err
	}
	if !after.HasDigest(g.digest) {
		return fmt.Errorf("it runs %s", shortDigest(after.Digest))
	}
	return e.ready(ctx, o.Name, a.before)
}

// finish ends the guard of an update the run is done with. The old image stays
// held only when the container could not go back to it.
func (s *Safeguard) finish(ctx context.Context, a *applied, o *Outcome) {
	g := a.guard
	switch {
	case o.RolledBack:
		_ = s.Guards.SetGuard(g.state(o.Name, store.GuardRolledBack, o.Err.Error()))
	case o.Updated, g.onOld:
		_ = s.Guards.DeleteGuard(o.Name)
	default:
		_ = s.Guards.SetGuard(g.state(o.Name, store.GuardUpdating, o.Err.Error()))
		return
	}
	_ = s.Images.Release(ctx, o.Name)
}

// pin has ref name digest, which wins over any tag it names.
func pin(ref, digest string) string {
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		ref = ref[:at]
	}
	return ref + "@" + digest
}
