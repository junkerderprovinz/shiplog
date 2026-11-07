package autoupdate

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/updater"
)

type fakeLister struct{ sts []model.UpdateStatus }

func (f fakeLister) List() ([]model.UpdateStatus, error) { return f.sts, nil }

type fakeUpdater struct {
	calls  []string
	failOn string
	sup    bool
}

func (f *fakeUpdater) Supported() bool { return f.sup }
func (f *fakeUpdater) Update(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, name)
	if name == f.failOn {
		return "script said boom", errors.New("boom")
	}
	return "script updated " + name, nil
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
		t.Fatalf("Updater.Update calls = %v, want only [b]; the blocked container must not be applied", upd.calls)
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
	res := e.Run(context.Background(), Policy{Level: LevelPatch}, false)
	if !reflect.DeepEqual(upd.calls, []string{"a"}) {
		t.Fatalf("with no exclude words configured the update must proceed normally, calls=%v", upd.calls)
	}
	if res.Outcomes[0].Blocked {
		t.Fatal("must not be marked Blocked when no exclude words are configured")
	}
}

// fakeHost is the updater and Docker in one: an update leaves the container on
// pulled[name], or keeps the old image when that is unset, and creates the
// container when it is missing, as Unraid's script does.
type fakeHost struct {
	fakeUpdater
	live   map[string]model.Container
	pulled map[string][]string
	// removing makes the next update leave the container half removed for that
	// many lists, after which it is gone. vanish does the same with the
	// container still looking as it did.
	removing map[string]int
	vanish   map[string]int
	left     map[string]int
	// stale makes the next update keep the old image, and orphaned leaves the
	// container on an image Docker can no longer inspect.
	stale    map[string]bool
	orphaned map[string]bool
	// created makes an update leave the container created but not started.
	created   map[string]bool
	exitErr   error  // returned once, after the update had its effect
	onSleep   func() // runs once, at the next sleep
	started   []string
	startErr  error
	slept     time.Duration
	listErr   error
	errAfter  bool // List fails once an update ran
	updatedAt int
}

func (h *fakeHost) Update(ctx context.Context, name string) (string, error) {
	out, err := h.fakeUpdater.Update(ctx, name)
	if err != nil {
		return out, err
	}
	h.updatedAt++
	c, ok := h.live[name]
	n, lose := h.removing[name]
	switch {
	case ok && lose:
		c.State = "removing"
		h.left[name] = n
	case !ok:
		c = model.Container{Name: name, State: "created"}
	case h.created[name]:
		c.State = "created"
	}
	switch {
	case c.State == "removing" || h.stale[name]:
	case h.orphaned[name]:
		c.Digest, c.Digests = "", nil
	default:
		if ds, ok := h.pulled[name]; ok {
			c.Digest, c.Digests = ds[0], ds
		}
	}
	if n, ok := h.vanish[name]; ok {
		h.left[name] = n
	}
	delete(h.removing, name)
	delete(h.vanish, name)
	delete(h.stale, name)
	delete(h.orphaned, name)
	h.live[name] = c
	if err := h.exitErr; err != nil {
		h.exitErr = nil
		return out, err
	}
	return out, nil
}

func (h *fakeHost) List(context.Context) ([]model.Container, error) {
	if h.listErr != nil || (h.errAfter && h.updatedAt > 0) {
		return nil, errors.New("docker socket gone")
	}
	var out []model.Container
	for name, c := range h.live {
		if n, ok := h.left[name]; ok {
			if n == 0 {
				delete(h.live, name)
				delete(h.left, name)
				continue
			}
			h.left[name] = n - 1
		}
		out = append(out, c)
	}
	return out, nil
}

func (h *fakeHost) Start(_ context.Context, name string) error {
	h.started = append(h.started, name)
	if h.startErr != nil {
		return h.startErr
	}
	h.setState(name, "running")
	return nil
}

func (h *fakeHost) sleep(_ context.Context, d time.Duration) error {
	h.slept += d
	if f := h.onSleep; f != nil {
		h.onSleep = nil
		f()
	}
	return nil
}

func (h *fakeHost) setState(name, state string) {
	c := h.live[name]
	c.State = state
	h.live[name] = c
}

func newHost(running map[string]string) *fakeHost {
	h := &fakeHost{
		fakeUpdater: fakeUpdater{sup: true},
		live:        map[string]model.Container{},
		pulled:      map[string][]string{},
		removing:    map[string]int{},
		vanish:      map[string]int{},
		left:        map[string]int{},
		stale:       map[string]bool{},
		orphaned:    map[string]bool{},
		created:     map[string]bool{},
	}
	for name, d := range running {
		h.live[name] = model.Container{Name: name, Digest: d, State: "running"}
	}
	return h
}

func stWithDigest(name, newest string) model.UpdateStatus {
	s := stNamed(name, model.KindDigest)
	s.NewestDigest = newest
	return s
}

func runHost(h *fakeHost, dryRun bool, sts ...model.UpdateStatus) Result {
	return runWith(h, h, dryRun, sts...)
}

func runWith(u updater.Updater, h *fakeHost, dryRun bool, sts ...model.UpdateStatus) Result {
	e := NewExecutor(fakeLister{sts: sts}, u).WithDocker(h)
	e.sleep = h.sleep
	return e.Run(context.Background(), Policy{Digest: true}, dryRun)
}

func TestExecutorConfirmsUpdateAgainstRunningDigest(t *testing.T) {
	h := newHost(map[string]string{"new": "sha256:a", "old": "sha256:a"})
	h.pulled["new"] = []string{"sha256:x", "sha256:b"} // a mirror pull lists the newest digest second
	res := runHost(h, false, stWithDigest("new", "sha256:b"), stWithDigest("old", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || o.Err != nil {
		t.Errorf("container on the newest digest = %+v, want Updated", o)
	}
	if o := res.Outcomes[1]; o.Updated || !errors.Is(o.Err, ErrNoChange) {
		t.Errorf("container still on its old image = %+v, want ErrNoChange", o)
	}
}

func TestExecutorReportsAContainerItCouldNotRecreate(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	upd := &goneAfterUpdate{h: h}
	res := runWith(upd, h, false, stWithDigest("a", "sha256:b"))
	if upd.calls != 3 {
		t.Errorf("update script ran %d times, want a run to recreate the container after the update and another in the final check", upd.calls)
	}
	if o := res.Outcomes[0]; o.Updated || o.Recreated || o.Err == nil || o.Err.Error() != "container gone after the update and could not be recreated" {
		t.Fatalf("outcome = %+v, want the container reported gone", o)
	}
}

type goneAfterUpdate struct {
	h     *fakeHost
	calls int
}

func (g *goneAfterUpdate) Supported() bool { return true }
func (g *goneAfterUpdate) Update(_ context.Context, name string) (string, error) {
	g.calls++
	delete(g.h.live, name)
	return "", nil
}

func TestExecutorRecreatesAContainerTheUpdateLost(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.removing["a"] = 3
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if !o.Updated || !o.Recreated || !o.Started || o.Err != nil {
		t.Fatalf("outcome = %+v, want an update that recreated and started the container", o)
	}
	if !reflect.DeepEqual(h.calls, []string{"a", "a"}) || !reflect.DeepEqual(h.started, []string{"a"}) {
		t.Errorf("updates %v, starts %v, want two updates and one start", h.calls, h.started)
	}
	if c := h.live["a"]; c.State != "running" || c.Digest != "sha256:b" {
		t.Errorf("container = %+v, want it running on the newest image", c)
	}
	if h.slept == 0 {
		t.Error("the run did not wait for Docker to finish the removal")
	}
	if o.Output != "script updated a\nscript updated a" {
		t.Errorf("Output = %q, want what both script runs printed", o.Output)
	}
}

func TestExecutorRecreatesALostStoppedContainerWithoutStartingIt(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.setState("a", "exited")
	h.pulled["a"] = []string{"sha256:b"}
	h.removing["a"] = 0
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || !o.Recreated || o.Started || o.Err != nil {
		t.Fatalf("outcome = %+v, want an update that recreated the container", o)
	}
	if len(h.started) != 0 || h.live["a"].State != "created" {
		t.Errorf("starts %v, state %q, want a stopped container to stay stopped", h.started, h.live["a"].State)
	}
}

func TestExecutorRecreatesALostContainerEvenWhenTheScriptFailed(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.removing["a"] = 0
	h.exitErr = errors.New("exit status 1")
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if o.Updated || !o.Recreated || !o.Started || o.Err == nil || o.Err.Error() != "exit status 1" {
		t.Fatalf("outcome = %+v, want the script's failure with the container recreated and started", o)
	}
	if h.live["a"].State != "running" {
		t.Errorf("container state %q, want running", h.live["a"].State)
	}
}

func TestExecutorStartsARunningContainerTheUpdateLeftCreated(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.created["a"] = true
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if !o.Updated || !o.Started || o.Recreated || o.Err != nil {
		t.Fatalf("outcome = %+v, want an update that started the container", o)
	}
	if h.live["a"].State != "running" || len(h.calls) != 1 {
		t.Errorf("state %q after %d updates, want running after one", h.live["a"].State, len(h.calls))
	}
	if o.Output == "" {
		t.Error("an update that left the container stopped lost the script output")
	}
}

func TestExecutorLeavesAStoppedContainerStopped(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.setState("a", "exited")
	h.pulled["a"] = []string{"sha256:b"}
	h.created["a"] = true
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || o.Started || o.Recreated || o.Err != nil || o.Output != "" {
		t.Fatalf("outcome = %+v, want a plain update", o)
	}
	if len(h.started) != 0 {
		t.Errorf("started %v, want a stopped container left stopped", h.started)
	}
}

func TestExecutorReportsAContainerItCouldNotStart(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.created["a"] = true
	h.startErr = errors.New("port is already allocated")
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if o.Updated || o.Started || o.Err == nil || !strings.Contains(o.Err.Error(), "port is already allocated") {
		t.Fatalf("outcome = %+v, want the start failure", o)
	}
}

func TestExecutorGivesUpOnAContainerThatStaysInRemoval(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.removing["a"] = 1 << 30
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if o.Updated || o.Recreated || o.Err == nil || !strings.Contains(o.Err.Error(), "removed") {
		t.Fatalf("outcome = %+v, want a failure naming the removal", o)
	}
	if len(h.calls) != 1 {
		t.Errorf("update script ran %d times, want no recreation while the old container holds the name", len(h.calls))
	}
	if h.slept != 3*settleTimeout {
		t.Errorf("waited %s, want one settle period after the update and two for the final check", h.slept)
	}
}

func TestExecutorRecreatesAContainerThatVanishesAfterLookingUnchanged(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.stale["a"] = true
	h.vanish["a"] = 2
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if !o.Updated || !o.Recreated || !o.Started || o.Err != nil {
		t.Fatalf("outcome = %+v, want an update that recreated and started the container", o)
	}
	if !reflect.DeepEqual(h.calls, []string{"a", "a"}) {
		t.Errorf("updates %v, want a second run to recreate the container", h.calls)
	}
	if c := h.live["a"]; c.State != "running" || c.Digest != "sha256:b" {
		t.Errorf("container = %+v, want it running on the newest image", c)
	}
}

func TestExecutorRecreatesAContainerThatVanishesOnAnUnknownImage(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.orphaned["a"] = true
	h.vanish["a"] = 2
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || !o.Recreated || !o.Started || o.Err != nil {
		t.Fatalf("outcome = %+v, want an update that recreated and started the container", o)
	}
}

func TestExecutorWatchesAnUnchangedContainerForOneSettlePeriod(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; o.Updated || o.Recreated || !errors.Is(o.Err, ErrNoChange) {
		t.Fatalf("outcome = %+v, want ErrNoChange", o)
	}
	if h.slept != 2*settleTimeout {
		t.Errorf("waited %s, want one settle period of watching and one before the final check", h.slept)
	}
	if len(h.calls) != 1 {
		t.Errorf("update script ran %d times, want once for a container that stayed", len(h.calls))
	}
}

func TestExecutorFinalCheckRecreatesAContainerThatVanishedLater(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.onSleep = func() { delete(h.live, "a") }
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	o := res.Outcomes[0]
	if !o.Updated || !o.Recreated || !o.Started || o.Err != nil {
		t.Fatalf("outcome = %+v, want the final check to recreate and start the container", o)
	}
	if h.slept < settleTimeout {
		t.Errorf("final check waited %s, want one settle period", h.slept)
	}
	if c := h.live["a"]; c.State != "running" || c.Digest != "sha256:b" {
		t.Errorf("container = %+v, want it running on the newest image", c)
	}
	if o.Output != "script updated a\nscript updated a" {
		t.Errorf("Output = %q, want what both script runs printed", o.Output)
	}
}

func TestExecutorFinalCheckStartsAContainerThatStoppedLater(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.onSleep = func() { h.setState("a", "exited") }
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || !o.Started || o.Recreated || o.Err != nil {
		t.Fatalf("outcome = %+v, want the final check to start the container", o)
	}
	if h.live["a"].State != "running" || len(h.calls) != 1 {
		t.Errorf("state %q after %d updates, want running after one", h.live["a"].State, len(h.calls))
	}
}

func TestExecutorFinalCheckKeepsAStoppedContainerStopped(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.setState("a", "exited")
	h.pulled["a"] = []string{"sha256:b"}
	h.onSleep = func() { delete(h.live, "a") }
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || !o.Recreated || o.Started || o.Err != nil {
		t.Fatalf("outcome = %+v, want the final check to recreate the container only", o)
	}
	if len(h.started) != 0 || h.live["a"].State != "created" {
		t.Errorf("starts %v, state %q, want a stopped container to stay stopped", h.started, h.live["a"].State)
	}
}

func TestExecutorFinalCheckWaitsOnlyAfterAnUpdate(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	res := runHost(h, true, stWithDigest("a", "sha256:b"))
	if len(h.calls) != 0 || h.slept != 0 || !res.Outcomes[0].Updated {
		t.Fatalf("dry run: updates %v, waited %s, outcome %+v, want neither", h.calls, h.slept, res.Outcomes[0])
	}
	res = runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; !o.Updated || o.Recreated || o.Started || o.Output != "" {
		t.Fatalf("outcome = %+v, want a plain update", o)
	}
	if h.slept != settleTimeout {
		t.Errorf("waited %s, want exactly one settle period for the final check", h.slept)
	}
}

func TestExecutorLeavesAContainerOnTheNewestImageAlone(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		h := newHost(map[string]string{"a": "sha256:b"})
		res := runHost(h, dryRun, stWithDigest("a", "sha256:b"))
		if len(h.calls) != 0 {
			t.Fatalf("dryRun=%v: Update called for %v, want none", dryRun, h.calls)
		}
		if o := res.Outcomes[0]; !o.UpToDate || o.Updated || o.Err != nil {
			t.Fatalf("dryRun=%v: outcome = %+v, want UpToDate", dryRun, o)
		}
		if res.Notable() {
			t.Fatalf("dryRun=%v: a run with nothing to do is not worth a notification", dryRun)
		}
	}
}

func TestExecutorFailsAnUpdateToAnUnexpectedImage(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:c"}
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; o.Updated || o.Err == nil || errors.Is(o.Err, ErrNoChange) {
		t.Fatalf("outcome = %+v, want a mismatch error", o)
	}
}

func TestExecutorNeverUpdatesWhatItCannotInspect(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.listErr = errors.New("docker socket gone")
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if len(h.calls) != 0 {
		t.Fatalf("Update called for %v, want none", h.calls)
	}
	if o := res.Outcomes[0]; o.Updated || o.Err == nil {
		t.Fatalf("outcome = %+v, want a failure", o)
	}
}

func TestExecutorReportsAnUpdateItCannotConfirm(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.pulled["a"] = []string{"sha256:b"}
	h.errAfter = true
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; o.Updated || o.Err == nil {
		t.Fatalf("outcome = %+v, want a failure", o)
	}
}

func TestExecutorWithoutNewestDigestNeedsAnotherImage(t *testing.T) {
	h := newHost(map[string]string{"same": "sha256:a", "moved": "sha256:a"})
	h.pulled["moved"] = []string{"sha256:z"}
	res := runHost(h, false, stWithDigest("same", ""), stWithDigest("moved", ""))
	if o := res.Outcomes[0]; o.Updated || !errors.Is(o.Err, ErrNoChange) {
		t.Errorf("unchanged image = %+v, want ErrNoChange", o)
	}
	if o := res.Outcomes[1]; !o.Updated || o.Err != nil {
		t.Errorf("changed image = %+v, want Updated", o)
	}
}

func TestExecutorSkipsConfirmationOnUpdateError(t *testing.T) {
	h := newHost(map[string]string{"a": "sha256:a"})
	h.failOn = "a"
	res := runHost(h, false, stWithDigest("a", "sha256:b"))
	if o := res.Outcomes[0]; o.Updated || o.Err == nil || o.Err.Error() != "boom" {
		t.Fatalf("outcome = %+v, want the updater's own error", o)
	}
}

func TestExecutorKeepsTheScriptOutputOfAnUpdateThatDidNotSucceed(t *testing.T) {
	h := newHost(map[string]string{"done": "sha256:a", "stale": "sha256:a", "broken": "sha256:a"})
	h.pulled["done"] = []string{"sha256:b"}
	h.failOn = "broken"
	res := runHost(h, false, stWithDigest("done", "sha256:b"), stWithDigest("stale", "sha256:b"), stWithDigest("broken", "sha256:b"))
	want := map[string]string{"done": "", "stale": "script updated stale", "broken": "script said boom"}
	for _, o := range res.Outcomes {
		if o.Output != want[o.Name] {
			t.Errorf("%s: Output = %q, want %q", o.Name, o.Output, want[o.Name])
		}
	}
}

func TestExecutorExcludedContainerNeverUpdated(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stNamed("gitea", model.KindMinor),
		stNamed("plex", model.KindMinor),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelMajor, ExcludeContainers: []string{"gitea"}}, false)
	if !reflect.DeepEqual(upd.calls, []string{"plex"}) {
		t.Fatalf("Update calls = %v, want only [plex]", upd.calls)
	}
	if len(res.Outcomes) != 2 {
		t.Fatalf("want one skipped and one updated outcome, got %+v", res.Outcomes)
	}
	gitea, plex := res.Outcomes[0], res.Outcomes[1]
	if !gitea.Skipped || gitea.Updated || gitea.Blocked || gitea.Err != nil {
		t.Fatalf("gitea = %+v, want only Skipped", gitea)
	}
	if plex.Skipped || !plex.Updated || plex.Err != nil {
		t.Fatalf("plex = %+v, want a plain update", plex)
	}
}

func TestExecutorExcludedMatchIgnoresCase(t *testing.T) {
	for _, configured := range []string{"gitea", "GITEA", "Gitea"} {
		upd := &fakeUpdater{sup: true}
		e := NewExecutor(fakeLister{sts: []model.UpdateStatus{stNamed("Gitea", model.KindPatch)}}, upd)
		res := e.Run(context.Background(), Policy{Level: LevelPatch, ExcludeContainers: []string{configured}}, false)
		if len(upd.calls) != 0 {
			t.Errorf("configured %q: Update called for %v, want none", configured, upd.calls)
		}
		if len(res.Outcomes) != 1 || !res.Outcomes[0].Skipped {
			t.Errorf("configured %q: outcomes = %+v, want one Skipped", configured, res.Outcomes)
		}
	}
}

func TestExecutorExcludedContainerSkippedInDryRun(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stNamed("gitea", model.KindMinor), stNamed("plex", model.KindMinor),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelMinor, ExcludeContainers: []string{"gitea"}}, true)
	if len(upd.calls) != 0 {
		t.Fatalf("a dry run must never call Update, got %v", upd.calls)
	}
	if !res.DryRun || len(res.Outcomes) != 2 {
		t.Fatalf("dry-run result = %+v", res)
	}
	if !res.Outcomes[0].Skipped || res.Outcomes[0].Updated {
		t.Fatalf("gitea = %+v, want Skipped, not a would-update", res.Outcomes[0])
	}
	if res.Outcomes[1].Skipped || !res.Outcomes[1].Updated {
		t.Fatalf("plex = %+v, want a would-update", res.Outcomes[1])
	}
}

func TestExecutorEmptyExcludeListChangesNothing(t *testing.T) {
	for _, list := range [][]string{nil, {}, {"other"}} {
		upd := &fakeUpdater{sup: true}
		e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
			stNamed("gitea", model.KindMinor), stNamed("plex", model.KindMajor), stNamed("old", model.KindUnknown),
		}}, upd)
		res := e.Run(context.Background(), Policy{Level: LevelMajor, ExcludeContainers: list}, false)
		if !reflect.DeepEqual(upd.calls, []string{"gitea", "plex"}) {
			t.Fatalf("list=%v: updated %v, want [gitea plex]", list, upd.calls)
		}
		if len(res.Outcomes) != 2 || res.Outcomes[0].Skipped || res.Outcomes[1].Skipped {
			t.Fatalf("list=%v: outcomes %+v, want two updates", list, res.Outcomes)
		}
	}
}

// Skipped means an update was due and held back, so an excluded container
// without an eligible update reports nothing.
func TestExecutorExcludedWithoutEligibleUpdateReportsNothing(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stNamed("gitea", model.KindNone), stNamed("plex", model.KindMajor),
	}}, upd)
	res := e.Run(context.Background(), Policy{Level: LevelMinor, ExcludeContainers: []string{"gitea", "plex"}}, false)
	if len(upd.calls) != 0 || len(res.Outcomes) != 0 {
		t.Fatalf("calls=%v outcomes=%+v, want neither", upd.calls, res.Outcomes)
	}
}

func TestExecutorExclusionWinsOverExcludeWord(t *testing.T) {
	upd := &fakeUpdater{sup: true}
	e := NewExecutor(fakeLister{sts: []model.UpdateStatus{
		stWithChangelog("gitea", model.KindPatch, "BREAKING: config format changed"),
	}}, upd)
	res := e.Run(context.Background(), Policy{
		Level: LevelPatch, ExcludeWords: []string{"breaking"}, ExcludeContainers: []string{"gitea"},
	}, false)
	if len(upd.calls) != 0 {
		t.Fatalf("Update called for %v, want none", upd.calls)
	}
	if len(res.Outcomes) != 1 || !res.Outcomes[0].Skipped || res.Outcomes[0].Blocked {
		t.Fatalf("outcomes = %+v, want Skipped and not Blocked", res.Outcomes)
	}
}

func TestResultNotable(t *testing.T) {
	cases := []struct {
		name string
		res  Result
		want bool
	}{
		{"empty run", Result{}, false},
		{"only skipped", Result{Outcomes: []Outcome{{Name: "gitea", Skipped: true}}}, false},
		{"skipped and updated", Result{Outcomes: []Outcome{{Name: "gitea", Skipped: true}, {Name: "plex", Updated: true}}}, true},
		{"skipped and blocked", Result{Outcomes: []Outcome{{Name: "gitea", Skipped: true}, {Name: "a", Blocked: true}}}, true},
		{"skipped and failed", Result{Outcomes: []Outcome{{Name: "gitea", Skipped: true}, {Name: "a", Err: errors.New("boom")}}}, true},
		{"only updated", Result{Outcomes: []Outcome{{Name: "plex", Updated: true}}}, true},
	}
	for _, c := range cases {
		if got := c.res.Notable(); got != c.want {
			t.Errorf("%s: Notable = %v, want %v", c.name, got, c.want)
		}
	}
}
