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
