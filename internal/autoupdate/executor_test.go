package autoupdate

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

type fakeLister struct{ sts []model.UpdateStatus }

func (f fakeLister) List() ([]model.UpdateStatus, error) { return f.sts, nil }

type fakeUpdater struct {
	calls  []string
	failOn string
	sup    bool
}

func (f *fakeUpdater) Supported() bool { return f.sup }
func (f *fakeUpdater) Update(_ context.Context, name string) error {
	f.calls = append(f.calls, name)
	if name == f.failOn {
		return errors.New("boom")
	}
	return nil
}

func stNamed(name string, k model.Kind) model.UpdateStatus {
	return model.UpdateStatus{Container: model.Container{Name: name}, Kind: k, NewestTag: "new"}
}

type errLister struct{}

func (errLister) List() ([]model.UpdateStatus, error) { return nil, errors.New("db down") }

func TestExecutorListErrorEmptyResult(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(errLister{}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelMajor}, false)
	if len(upd.calls) != 0 || len(res.Outcomes) != 0 {
		t.Fatalf("a lister error must yield an empty run with no updates, got calls=%v outcomes=%+v", upd.calls, res.Outcomes)
	}
}

func TestExecutorSerialEligibleOnly(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stNamed("a", model.KindPatch), stNamed("b", model.KindMajor), stNamed("c", model.KindUnknown),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelPatch}, false)
	if !reflect.DeepEqual(upd.calls, []string{"a"}) {
		t.Fatalf("updated %v, want [a] (only patch under patch)", upd.calls)
	}
	if len(res.Outcomes) != 1 || !res.Outcomes[0].Updated {
		t.Fatalf("outcomes %+v", res.Outcomes)
	}
}

func TestExecutorDryRunAppliesNothing(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{stNamed("a", model.KindPatch)}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelMajor}, true)
	if len(upd.calls) != 0 {
		t.Fatal("dry-run must not call Update")
	}
	if !res.DryRun || len(res.Outcomes) != 1 || !res.Outcomes[0].Updated {
		t.Fatalf("dry-run outcome should mark would-update: %+v", res)
	}
}

func TestExecutorFailureDoesNotAbort(t *testing.T) {
	upd := &fakeUpdater{sup: true, failOn: "a"}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{stNamed("a", model.KindPatch), stNamed("d", model.KindPatch)}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelPatch}, false)
	if len(upd.calls) != 2 || upd.calls[1] != "d" {
		t.Fatalf("must continue past a failure to d, calls=%v", upd.calls)
	}
	if res.Outcomes[0].Err == nil || res.Outcomes[1].Err != nil {
		t.Fatalf("a should fail, d succeed: %+v", res.Outcomes)
	}
}

func TestExecutorUnsupportedNoop(t *testing.T) {
	upd := &fakeUpdater{sup: false}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{stNamed("a", model.KindPatch)}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelMajor}, false)
	if len(upd.calls) != 0 || len(res.Outcomes) != 0 {
		t.Fatal("an unsupported updater must be a no-op")
	}
}

func stWithChangelog(name string, k model.Kind, raw string) model.UpdateStatus {
	s := stNamed(name, k)
	s.Changelog = &model.Changelog{Raw: raw}
	return s
}

func TestExecutorExcludeWordBlocksRealUpdate(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stWithChangelog("a", model.KindPatch, "This release includes a BREAKING change."),
		stWithChangelog("b", model.KindPatch, "Just bug fixes."),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelPatch, ExcludeWords: []string{"breaking"}}, false)
	if !reflect.DeepEqual(upd.calls, []string{"b"}) {
		t.Fatalf("Updater.Update calls = %v, want only [b] — the blocked container must never be applied", upd.calls)
	}
	if len(res.Outcomes) != 2 {
		t.Fatalf("want 2 outcomes (one blocked, one updated), got %+v", res.Outcomes)
	}
	a, b := res.Outcomes[0], res.Outcomes[1]
	if !a.Blocked || a.BlockedWord != "breaking" || a.Updated || a.Err != nil {
		t.Fatalf("outcome a = %+v, want Blocked with word 'breaking', not Updated, no Err", a)
	}
	if b.Blocked || !b.Updated || b.Err != nil {
		t.Fatalf("outcome b = %+v, want plain Updated", b)
	}
}

func TestExecutorExcludeWordBlocksDryRunToo(t *testing.T) {
	// The safety switch must be visible in dry-run — an admin verifying it works
	// before arming real updates must see "would be blocked", not "would update".
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stWithChangelog("a", model.KindPatch, "BREAKING: config format changed"),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelPatch, ExcludeWords: []string{"breaking"}}, true)
	if len(upd.calls) != 0 {
		t.Fatal("dry-run must never call Update, blocked or not")
	}
	if len(res.Outcomes) != 1 || !res.Outcomes[0].Blocked || res.Outcomes[0].Updated {
		t.Fatalf("dry-run blocked outcome = %+v, want Blocked=true Updated=false", res.Outcomes)
	}
}

func TestExecutorNoExcludeWordsConfiguredUpdatesNormally(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stWithChangelog("a", model.KindPatch, "BREAKING: this text is irrelevant with no configured words"),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelPatch}, false) // ExcludeWords nil
	if !reflect.DeepEqual(upd.calls, []string{"a"}) {
		t.Fatalf("with no exclude words configured the update must proceed normally, calls=%v", upd.calls)
	}
	if res.Outcomes[0].Blocked {
		t.Fatal("must not be marked Blocked when no exclude words are configured")
	}
}
