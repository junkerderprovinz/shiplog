package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// Inspector reads the containers as Docker sees them (the Docker client).
type Inspector interface {
	List(ctx context.Context) ([]model.Container, error)
}

// Executor applies eligible updates serially.
type Executor struct {
	list    Lister
	upd     updater.Updater
	inspect Inspector
}

// NewExecutor builds an Executor over a status lister and an updater.
func NewExecutor(l Lister, u updater.Updater) *Executor { return &Executor{list: l, upd: u} }

// WithInspector makes the run look at each container before and after its
// update: one already on the newest image is left alone, and an update counts
// only once the container runs the newest image.
func (e *Executor) WithInspector(i Inspector) *Executor {
	e.inspect = i
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
		case e.inspect != nil && !found:
			o.Err = errors.New("container not found")
		case e.inspect != nil && st.NewestDigest != "" && before.HasDigest(st.NewestDigest):
			o.UpToDate = true
		case dryRun:
			o.Updated = true
		default:
			o.Err = e.upd.Update(ctx, o.Name)
			if o.Err == nil {
				o.Err = e.verify(ctx, st, before)
			}
			o.Updated = o.Err == nil
		}
		res.Outcomes = append(res.Outcomes, o)
	}
	return res
}

// verify checks that the container now runs the newest image the last check
// found, or at least another image than before when that digest is unknown.
func (e *Executor) verify(ctx context.Context, st model.UpdateStatus, before model.Container) error {
	if e.inspect == nil {
		return nil
	}
	after, found, err := e.running(ctx, st.Container.Name)
	if err != nil {
		return fmt.Errorf("could not confirm the update: %w", err)
	}
	if !found {
		return errors.New("container gone after the update")
	}
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

// running returns the named container as Docker sees it now. Without an
// inspector it reports nothing and no error.
func (e *Executor) running(ctx context.Context, name string) (model.Container, bool, error) {
	if e.inspect == nil {
		return model.Container{}, false, nil
	}
	cs, err := e.inspect.List(ctx)
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
