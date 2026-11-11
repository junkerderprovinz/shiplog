package updates

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/model"
)

type fakeStore []model.UpdateStatus

func (s fakeStore) List() ([]model.UpdateStatus, error) { return s, nil }

func status(name string, kind model.Kind) model.UpdateStatus {
	return model.UpdateStatus{Container: model.Container{Name: name}, Kind: kind, NewestTag: "2"}
}

// fakeExec answers each container with the outcome named in results, and
// waits for release before it does.
type fakeExec struct {
	mu       sync.Mutex
	results  map[string]autoupdate.Outcome
	asked    [][]string
	backedUp [][]string
	release  chan struct{}
	busy     int
	overlap  bool
}

func (f *fakeExec) Update(_ context.Context, sts []model.UpdateStatus, backedUp []string) autoupdate.Result {
	f.mu.Lock()
	f.busy++
	if f.busy > 1 {
		f.overlap = true
	}
	var names []string
	for _, st := range sts {
		names = append(names, st.Container.Name)
	}
	f.asked = append(f.asked, names)
	f.backedUp = append(f.backedUp, backedUp)
	f.mu.Unlock()
	if f.release != nil {
		<-f.release
	}
	var res autoupdate.Result
	for _, st := range sts {
		o, ok := f.results[st.Container.Name]
		if !ok {
			o = autoupdate.Outcome{Updated: true}
		}
		o.Name = st.Container.Name
		res.Outcomes = append(res.Outcomes, o)
	}
	f.mu.Lock()
	f.busy--
	f.mu.Unlock()
	return res
}

func newRuns(store fakeStore, exec *fakeExec) *Runs {
	return &Runs{Store: store, Exec: exec, Context: context.Background(), Lock: &sync.Mutex{}}
}

func act(t *testing.T, r *Runs, body string) ([]byte, error) {
	t.Helper()
	return r.Action().Run(context.Background(), fortress.Caller{ID: "m2", App: "towertorch", Device: "Tower"}, []byte(body))
}

type answer struct {
	Run        string `json:"run"`
	State      string `json:"state"`
	Error      string `json:"error"`
	Containers []struct {
		Name   string `json:"name"`
		Result string `json:"result"`
		Error  string `json:"error"`
	} `json:"containers"`
}

func read(t *testing.T, raw []byte, err error) answer {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var a answer
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return a
}

func ask(t *testing.T, r *Runs, body string) answer {
	t.Helper()
	raw, err := act(t, r, body)
	return read(t, raw, err)
}

// finished asks about run until it is done.
func finished(t *testing.T, r *Runs, run string) answer {
	t.Helper()
	q, _ := json.Marshal(map[string]string{"run": run})
	for range 500 {
		a := ask(t, r, string(q))
		if a.State != "running" {
			return a
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("the run never finished")
	return answer{}
}

func TestTheActionIsUpdatesRun(t *testing.T) {
	if got := newRuns(nil, &fakeExec{}).Action().Name; got != "updates.run" {
		t.Fatalf("action = %q", got)
	}
}

func TestARunUpdatesTheChosenContainersAndReportsEachOne(t *testing.T) {
	store := fakeStore{status("web", model.KindPatch), status("db", model.KindMajor), status("cache", model.KindMinor),
		status("mail", model.KindMinor), status("log", model.KindMinor), status("calm", model.KindNone)}
	exec := &fakeExec{results: map[string]autoupdate.Outcome{
		"db":    {Err: errors.New("unhealthy after the update"), RolledBack: true},
		"cache": {NoBackup: true, Err: errors.New("held back without a backup: the backup failed")},
		"mail":  {Err: errors.New("container not found")},
		"log":   {UpToDate: true},
	}}
	r := newRuns(store, exec)
	start := ask(t, r, `{"containers":["web","DB","cache","mail","log","calm","gone"],"backedUp":["web"],"reason":"maintenance"}`)
	if start.Run == "" {
		t.Fatal("no run id")
	}
	done := finished(t, r, start.Run)
	if done.State != "done" {
		t.Fatalf("state = %q (%s)", done.State, done.Error)
	}
	want := map[string][2]string{
		"web": {"updated", ""}, "db": {"rolledBack", "unhealthy after the update"},
		"cache": {"held", "held back without a backup: the backup failed"}, "mail": {"failed", "container not found"},
		"log": {"current", ""}, "calm": {"current", ""}, "gone": {"failed", "ShipLog does not know this container"},
	}
	if len(done.Containers) != len(want) {
		t.Fatalf("containers = %+v", done.Containers)
	}
	for i, c := range done.Containers {
		if w := want[c.Name]; c.Result != w[0] || c.Error != w[1] {
			t.Errorf("%s = %s %q, want %s %q", c.Name, c.Result, c.Error, w[0], w[1])
		}
		if order := []string{"web", "db", "cache", "mail", "log", "calm", "gone"}; c.Name != order[i] {
			t.Errorf("result %d is %s, want %s: the order asked for", i, c.Name, order[i])
		}
	}
	if len(exec.asked) != 1 || len(exec.asked[0]) != 5 || exec.backedUp[0][0] != "web" {
		t.Errorf("updated %v with backups %v, want the five with an update and web backed up", exec.asked, exec.backedUp)
	}
}

func TestAnEmptyListMeansEveryContainerWithAnUpdateButTheExcludedOnes(t *testing.T) {
	store := fakeStore{status("web", model.KindPatch), status("db", model.KindMajor), status("calm", model.KindNone), status("plex", model.KindMinor)}
	exec := &fakeExec{}
	r := newRuns(store, exec)
	r.Exclude = []string{"Plex"}
	done := finished(t, r, ask(t, r, `{"containers":[],"backedUp":[],"reason":"maintenance"}`).Run)
	if len(done.Containers) != 2 || done.Containers[0].Name != "web" || done.Containers[1].Name != "db" {
		t.Fatalf("containers = %+v, want web and db", done.Containers)
	}
}

func TestRunsTakeTurnsWithEachOtherAndTheAutoUpdate(t *testing.T) {
	exec := &fakeExec{release: make(chan struct{})}
	r := newRuns(fakeStore{status("web", model.KindPatch)}, exec)
	r.Lock.Lock()
	first := ask(t, r, `{"containers":["web"]}`).Run
	second := ask(t, r, `{"containers":["web"]}`).Run
	if first == second {
		t.Fatalf("two runs share the id %q", first)
	}
	if a := ask(t, r, `{"run":"`+first+`"}`); a.State != "running" {
		t.Fatalf("state while the auto-update holds the lock = %q, want running", a.State)
	}
	asked := func() int {
		exec.mu.Lock()
		defer exec.mu.Unlock()
		return len(exec.asked)
	}
	time.Sleep(50 * time.Millisecond)
	if n := asked(); n != 0 {
		t.Fatalf("%d runs updated while the auto-update held the lock", n)
	}
	r.Lock.Unlock()
	for asked() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if n := asked(); n != 1 {
		t.Fatalf("%d runs update at once, want one after the other", n)
	}
	close(exec.release)
	finished(t, r, first)
	finished(t, r, second)
	if exec.overlap {
		t.Error("two runs updated at the same time")
	}
}

func TestAnUnknownRunIsRefused(t *testing.T) {
	_, err := act(t, newRuns(nil, &fakeExec{}), `{"run":"u9"}`)
	var no *fortress.Refusal
	if !errors.As(err, &no) || no.Code != "no_such_run" {
		t.Fatalf("err = %v, want the refusal no_such_run", err)
	}
}

func TestAnUnreadableRequestIsRefused(t *testing.T) {
	_, err := act(t, newRuns(nil, &fakeExec{}), `[1,2]`)
	var no *fortress.Refusal
	if !errors.As(err, &no) || no.Code != "bad_request" {
		t.Fatalf("err = %v, want the refusal bad_request", err)
	}
}

func restarted(t *testing.T, file string) *Runs {
	t.Helper()
	r := newRuns(fakeStore{status("web", model.KindPatch)}, &fakeExec{})
	r.File = file
	if err := r.Restore(); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAFinishedRunIsStillKnownAfterARestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "runs.json")
	r := newRuns(fakeStore{status("web", model.KindPatch)}, &fakeExec{})
	r.File = file
	id := ask(t, r, `{"containers":["web"]}`).Run
	finished(t, r, id)

	a := ask(t, restarted(t, file), `{"run":"`+id+`"}`)
	if a.State != "done" || len(a.Containers) != 1 || a.Containers[0].Result != "updated" {
		t.Fatalf("after a restart = %+v, want the finished run", a)
	}
	entries, _ := os.ReadDir(filepath.Dir(file))
	if len(entries) != 1 {
		t.Errorf("the directory holds %d files, want runs.json alone", len(entries))
	}
}

func TestARunCutShortByARestartFailedWithItsReason(t *testing.T) {
	file := filepath.Join(t.TempDir(), "runs.json")
	// The run never ends, as if ShipLog stopped in the middle of it.
	exec := &fakeExec{release: make(chan struct{})}
	r := newRuns(fakeStore{status("web", model.KindPatch)}, exec)
	r.File = file
	id := ask(t, r, `{"containers":["web"]}`).Run

	again := restarted(t, file)
	a := ask(t, again, `{"run":"`+id+`"}`)
	if a.State != "failed" || a.Error != "ShipLog restarted during the run" {
		t.Fatalf("after a restart = %+v, want failed with the reason", a)
	}
	if a := ask(t, restarted(t, file), `{"run":"`+id+`"}`); a.State != "failed" {
		t.Fatalf("after a second restart = %+v, want it failed still", a)
	}
}

func TestOnlyTheLastRunsAreKept(t *testing.T) {
	file := filepath.Join(t.TempDir(), "runs.json")
	r := newRuns(fakeStore{status("web", model.KindPatch)}, &fakeExec{})
	r.File = file
	var ids []string
	for range keep + 3 {
		id := ask(t, r, `{"containers":["web"]}`).Run
		finished(t, r, id)
		ids = append(ids, id)
	}
	again := restarted(t, file)
	if _, err := act(t, again, `{"run":"`+ids[2]+`"}`); err == nil {
		t.Error("an old run outlived the limit")
	}
	if a := ask(t, again, `{"run":"`+ids[3]+`"}`); a.State != "done" {
		t.Errorf("the oldest kept run = %+v", a)
	}
}

func TestAMissingRunsFileIsAFreshStart(t *testing.T) {
	r := restarted(t, filepath.Join(t.TempDir(), "runs.json"))
	if _, err := act(t, r, `{"run":"x"}`); err == nil {
		t.Fatal("want no_such_run")
	}
}

func TestARunTellsWhenItBeganAndEnded(t *testing.T) {
	r := newRuns(fakeStore{status("web", model.KindPatch)}, &fakeExec{})
	var began, ended int
	var got autoupdate.Result
	r.Began = func(time.Time) { began++ }
	r.Ended = func(res autoupdate.Result) { ended++; got = res }
	finished(t, r, ask(t, r, `{"containers":["web"]}`).Run)
	if began != 1 || ended != 1 || len(got.Outcomes) != 1 || got.Outcomes[0].Name != "web" {
		t.Fatalf("began %d, ended %d with %+v", began, ended, got)
	}
}
