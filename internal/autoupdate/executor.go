package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/updater"
)

// Lister supplies the current per-container update statuses (the store).
type Lister interface {
	List() ([]model.UpdateStatus, error)
}

// ErrNoChange is the failure of an update after which the container still
// runs the image it ran before. Unraid's update script exits 0 even when the
// pull fails or the container is not recreated.
var ErrNoChange = errors.New("still on the old image after the update")

// Outcome is the per-container result of one auto-update run.
type Outcome struct {
	Name    string
	From    string
	To      string
	Level   string
	Updated bool
	Err     error
	// Blocked marks an eligible update whose changelog matched BlockedWord. It is
	// set in dry-run mode too, so the exclude list can be checked before it is
	// trusted.
	Blocked     bool
	BlockedWord string
	// Skipped marks an eligible update of a container on the exclude list. It
	// is the admin's own choice, so it is reported apart from Blocked.
	Skipped bool
	// UpToDate marks an eligible update whose container already ran the newest
	// image when the run looked, because the stored status was older.
	UpToDate bool
	// Recreated marks a container the update lost and the run created again
	// from its template, and Started one it had to start because it ran before.
	Recreated bool
	Started   bool
	// Output is what the update script printed, kept only for an update that
	// did not cleanly succeed, so the log shows what went wrong.
	Output string
}

// Result aggregates one auto-update run.
type Result struct {
	Outcomes []Outcome
	DryRun   bool
}

// Notable reports whether the run is worth a notification. A run that only
// skipped excluded containers is not; the log still lists them.
func (r Result) Notable() bool {
	for _, o := range r.Outcomes {
		if !o.Skipped && !o.UpToDate {
			return true
		}
	}
	return false
}

// Failed reports whether an update of the run failed.
func (r Result) Failed() bool {
	for _, o := range r.Outcomes {
		if o.Err != nil {
			return true
		}
	}
	return false
}

// Docker reads the containers as Docker sees them and starts one an update left
// stopped (the Docker client).
type Docker interface {
	List(ctx context.Context) ([]model.Container, error)
	Start(ctx context.Context, name string) error
}

// Unraid's update script exits 0 whatever happened to the container, and Docker
// can still be stopping or removing the old one when it returns, so the run
// waits for Docker to settle before it looks at the result.
const (
	settleTimeout = 5 * time.Minute
	settlePoll    = 5 * time.Second
)

// Executor applies eligible updates serially.
type Executor struct {
	list     Lister
	upd      updater.Updater
	docker   Docker
	progress func(name string)
	sleep    func(context.Context, time.Duration) error
}

// NewExecutor builds an Executor over a status lister and an updater.
func NewExecutor(l Lister, u updater.Updater) *Executor {
	return &Executor{list: l, upd: u, sleep: sleep}
}

// WithDocker makes the run look at each container before and after its update:
// one already on the newest image is left alone, one the update lost or left
// stopped is brought back to how it was, and an update counts only once the
// container runs the newest image.
func (e *Executor) WithDocker(d Docker) *Executor {
	e.docker = d
	return e
}

// WithProgress has the run name each container right before it updates it.
func (e *Executor) WithProgress(f func(name string)) *Executor {
	e.progress = f
	return e
}

// Run applies every eligible update one at a time, or only reports them in
// dryRun. A failure is recorded in its Outcome and does not stop the rest.
func (e *Executor) Run(ctx context.Context, p Policy, dryRun bool) Result {
	res := Result{DryRun: dryRun}
	if !e.upd.Supported() {
		return res
	}
	statuses, err := e.list.List()
	if err != nil {
		return res
	}
	var done []applied
	for _, st := range statuses {
		if !Eligible(st, p) {
			continue
		}
		o := Outcome{
			Name:  st.Container.Name,
			From:  st.RunningVersion,
			To:    st.NewestTag,
			Level: string(st.Kind),
		}
		if ContainerExcluded(o.Name, p.ExcludeContainers) {
			o.Skipped = true
			res.Outcomes = append(res.Outcomes, o)
			continue
		}
		if word := MatchedExcludeWord(st.Changelog, p.ExcludeWords); word != "" {
			o.Blocked = true
			o.BlockedWord = word
			res.Outcomes = append(res.Outcomes, o)
			continue
		}
		before, found, ierr := e.running(ctx, o.Name)
		switch {
		case ierr != nil:
			// What cannot be looked at before cannot be confirmed after.
			o.Err = fmt.Errorf("could not inspect the container before the update: %w", ierr)
		case e.docker != nil && !found:
			o.Err = errors.New("container not found")
		case e.docker != nil && st.NewestDigest != "" && before.HasDigest(st.NewestDigest):
			o.UpToDate = true
		case dryRun:
			o.Updated = true
		default:
			if e.progress != nil {
				e.progress(o.Name)
			}
			a := applied{i: len(res.Outcomes), st: st, before: before}
			o.Err = e.apply(ctx, &a, &o)
			o.Updated = o.Err == nil
			done = append(done, a)
		}
		res.Outcomes = append(res.Outcomes, o)
	}
	e.recheck(ctx, res.Outcomes, done)
	for _, a := range done {
		if o := &res.Outcomes[a.i]; !o.Updated || o.Recreated || o.Started {
			o.Output = a.out
		}
	}
	return res
}

// applied is one update the run ran, kept for the check at the end of the run.
type applied struct {
	i         int // into Result.Outcomes
	st        model.UpdateStatus
	before    model.Container
	out       string
	scriptErr error
}

// apply runs the update and brings the container back to the run state it had
// before. The script exits 0 even when it lost the container on the way, so the
// container is checked whatever the script reported.
func (e *Executor) apply(ctx context.Context, a *applied, o *Outcome) error {
	a.out, a.scriptErr = e.upd.Update(ctx, o.Name)
	if e.docker == nil {
		return a.scriptErr
	}
	after, _, err := e.keep(ctx, a, o)
	if err != nil {
		return err
	}
	if a.scriptErr != nil {
		return a.scriptErr
	}
	verr := verify(a.st, a.before, after)
	if verr == nil || (!errors.Is(verr, ErrNoChange) && after.Digest != "") {
		return verr
	}
	// A container that looks unchanged may still be on its way out, and Docker
	// can remove it after the script has returned.
	leaving, err := e.watch(ctx, o.Name)
	if err != nil {
		return fmt.Errorf("could not confirm the update: %w", err)
	}
	if !leaving {
		return verr
	}
	if after, _, err = e.keep(ctx, a, o); err != nil {
		return err
	}
	return verify(a.st, a.before, after)
}

// keep waits for Docker to settle on the container, recreates it from its
// template when it is gone and starts it when it ran before the update. It
// reports whether it had to do either.
func (e *Executor) keep(ctx context.Context, a *applied, o *Outcome) (model.Container, bool, error) {
	after, found, err := e.settle(ctx, o.Name)
	if err != nil {
		return after, false, fmt.Errorf("could not confirm the update: %w", err)
	}
	fixed := false
	if !found {
		// With no old container left, the script creates one from the template.
		more, err := e.upd.Update(ctx, o.Name)
		a.out = strings.TrimSpace(a.out + "\n" + more)
		if err != nil {
			return after, false, fmt.Errorf("container gone after the update, recreating it failed: %w", err)
		}
		if after, found, err = e.settle(ctx, o.Name); err != nil {
			return after, false, fmt.Errorf("could not confirm the recreated container: %w", err)
		}
		if !found {
			return after, false, errors.New("container gone after the update and could not be recreated")
		}
		o.Recreated, fixed = true, true
	}
	if a.before.State == "running" && (after.State == "created" || after.State == "exited") {
		if err := e.docker.Start(ctx, o.Name); err != nil {
			return after, fixed, fmt.Errorf("container %s after the update, starting it failed: %w", after.State, err)
		}
		o.Started, fixed = true, true
	}
	return after, fixed, nil
}

// recheck looks at every container the run updated once more after a settle
// period, since Docker can finish a removal long after the script returned.
func (e *Executor) recheck(ctx context.Context, outcomes []Outcome, done []applied) {
	if e.docker == nil || len(done) == 0 || e.sleep(ctx, settleTimeout) != nil {
		return
	}
	for k := range done {
		a := &done[k]
		o := &outcomes[a.i]
		after, fixed, err := e.keep(ctx, a, o)
		switch {
		case err != nil:
			o.Err = err
		case !fixed:
			continue
		case a.scriptErr != nil:
			o.Err = a.scriptErr
		default:
			o.Err = verify(a.st, a.before, after)
		}
		o.Updated = o.Err == nil
	}
}

// watch looks at the container for one settle period and reports whether
// Docker began removing it or it went away.
func (e *Executor) watch(ctx context.Context, name string) (bool, error) {
	for waited := time.Duration(0); waited < settleTimeout; waited += settlePoll {
		if err := e.sleep(ctx, settlePoll); err != nil {
			return false, err
		}
		c, found, err := e.running(ctx, name)
		if err != nil {
			return false, err
		}
		if !found || c.State == "removing" {
			return true, nil
		}
	}
	return false, nil
}

// settle returns the named container once Docker is done removing it, or
// reports it gone.
func (e *Executor) settle(ctx context.Context, name string) (model.Container, bool, error) {
	for waited := time.Duration(0); ; waited += settlePoll {
		c, found, err := e.running(ctx, name)
		if err != nil || !found || c.State != "removing" {
			return c, found, err
		}
		if waited >= settleTimeout {
			return c, true, fmt.Errorf("the old container is still being removed after %s", settleTimeout)
		}
		if err := e.sleep(ctx, settlePoll); err != nil {
			return c, true, err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// verify checks that the container now runs the newest image the last check
// found, or at least another image than before when that digest is unknown.
func verify(st model.UpdateStatus, before, after model.Container) error {
	if st.NewestDigest != "" && after.HasDigest(st.NewestDigest) {
		return nil
	}
	if sharesImage(before, after) {
		return ErrNoChange
	}
	if st.NewestDigest == "" {
		return nil
	}
	return fmt.Errorf("now runs %s instead of the expected %s", shortDigest(after.Digest), shortDigest(st.NewestDigest))
}

// running returns the named container as Docker sees it now. Without Docker it
// reports nothing and no error.
func (e *Executor) running(ctx context.Context, name string) (model.Container, bool, error) {
	if e.docker == nil {
		return model.Container{}, false, nil
	}
	cs, err := e.docker.List(ctx)
	if err != nil {
		return model.Container{}, false, err
	}
	for _, c := range cs {
		if c.Name == name {
			return c, true, nil
		}
	}
	return model.Container{}, false, nil
}

// sharesImage reports whether a and b have a manifest digest in common; a
// mirror pull carries one per registry.
func sharesImage(a, b model.Container) bool {
	if b.Digest != "" && a.HasDigest(b.Digest) {
		return true
	}
	for _, d := range b.Digests {
		if d != "" && a.HasDigest(d) {
			return true
		}
	}
	return false
}

// shortDigest cuts a digest to the 12 hex characters docker images shows.
func shortDigest(d string) string {
	d = strings.TrimPrefix(d, "sha256:")
	switch {
	case d == "":
		return "?"
	case len(d) > 12:
		return d[:12]
	}
	return d
}
