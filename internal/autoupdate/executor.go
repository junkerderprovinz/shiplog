package autoupdate

import (
	"context"
	"errors"
	"fmt"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/updater"
)

// Lister supplies the current per-container update statuses (the store).
type Lister interface {
	List() ([]model.UpdateStatus, error)
}

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
		if !o.Skipped {
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

// WithInspector makes every applied update count only once the container runs
// the newest image.
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
		if dryRun {
			o.Updated = true
		} else {
			o.Err = e.upd.Update(ctx, st.Container.Name)
			if o.Err == nil {
				o.Err = e.verify(ctx, st)
			}
			o.Updated = o.Err == nil
		}
		res.Outcomes = append(res.Outcomes, o)
	}
	return res
}

// verify checks that the container now runs the digest the last check found.
// Unraid's update script exits 0 even when the pull fails or the container is
// not recreated, so its exit code alone proves nothing.
func (e *Executor) verify(ctx context.Context, st model.UpdateStatus) error {
	if e.inspect == nil || st.NewestDigest == "" {
		return nil
	}
	cs, err := e.inspect.List(ctx)
	if err != nil {
		return fmt.Errorf("could not confirm the update: %w", err)
	}
	for _, c := range cs {
		if c.Name != st.Container.Name {
			continue
		}
		if c.HasDigest(st.NewestDigest) {
			return nil
		}
		return errors.New("still on the old image after the update")
	}
	return errors.New("container gone after the update")
}
