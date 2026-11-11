// Package updates offers the fortress the action updates.run: another app,
// such as a maintenance plan, has ShipLog update containers and asks about
// the run until it is done.
package updates

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Action is the name other apps call it by.
const Action = "updates.run"

// keep bounds the finished runs held for the caller's last question.
const keep = 32

// Store lists the containers with what the last check found.
type Store interface {
	List() ([]model.UpdateStatus, error)
}

// Executor applies updates the way the auto-update does, with its backup and
// rollback.
type Executor interface {
	Update(ctx context.Context, sts []model.UpdateStatus, backedUp []string) autoupdate.Result
}

// Runs carries out updates.run.
type Runs struct {
	Store Store
	Exec  Executor
	// Exclude are the containers ShipLog leaves to the admin. A run for
	// every container passes them over, a run that names one does not.
	Exclude []string
	// Context outlives the call that started a run.
	Context context.Context
	// Lock is shared with the scheduled auto-update, so two runs never
	// update at the same time.
	Lock  sync.Locker
	Began func(time.Time)
	Ended func(autoupdate.Result)

	mu    sync.Mutex
	runs  map[string]*run
	order []string
}

type run struct {
	State      string   `json:"state"`
	Containers []result `json:"containers"`
	Error      string   `json:"error,omitempty"`
}

type result struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// The results of one container.
const (
	updated    = "updated"
	current    = "current"
	held       = "held"
	rolledBack = "rolledBack"
	failed     = "failed"
)

// Action is updates.run for fortress.Config.Actions.
func (r *Runs) Action() fortress.Action {
	return fortress.Action{Name: Action, Run: r.serve}
}

type request struct {
	Run        json.RawMessage `json:"run"`
	Containers []string        `json:"containers"`
	BackedUp   []string        `json:"backedUp"`
	Reason     string          `json:"reason"`
}

func (r *Runs) serve(_ context.Context, _ fortress.Caller, body []byte) ([]byte, error) {
	var req request
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) || json.Unmarshal(body, &req) != nil {
		return nil, refuse("bad_request")
	}
	if req.Run != nil {
		return r.answer(runID(req.Run))
	}
	return r.start(req)
}

func (r *Runs) answer(id string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ru, ok := r.runs[id]
	if !ok {
		return nil, refuse("no_such_run")
	}
	return json.Marshal(ru)
}

func (r *Runs) start(req request) ([]byte, error) {
	id := newID()
	ru := &run{State: "running", Containers: []result{}}
	r.mu.Lock()
	if r.runs == nil {
		r.runs = map[string]*run{}
	}
	r.runs[id] = ru
	r.order = append(r.order, id)
	if len(r.order) > keep {
		delete(r.runs, r.order[0])
		r.order = r.order[1:]
	}
	r.mu.Unlock()
	go r.carryOut(ru, req)
	return json.Marshal(map[string]string{"run": id})
}

// carryOut waits its turn, updates the containers and fills in ru.
func (r *Runs) carryOut(ru *run, req request) {
	r.Lock.Lock()
	defer r.Lock.Unlock()
	sts, err := r.Store.List()
	if err != nil {
		r.finish(ru, nil, err.Error())
		return
	}
	results, due := r.plan(sts, req.Containers)
	if r.Began != nil {
		r.Began(time.Now())
	}
	res := r.Exec.Update(r.Context, due, req.BackedUp)
	if r.Ended != nil {
		r.Ended(res)
	}
	for _, o := range res.Outcomes {
		for i := range results {
			if strings.EqualFold(results[i].Name, o.Name) {
				results[i].Result, results[i].Error = resultOf(o)
			}
		}
	}
	r.finish(ru, results, "")
}

// plan answers each container asked for in the order asked, and returns the
// ones with an update to apply. Without a list, it takes every container with
// an update that is not excluded.
func (r *Runs) plan(sts []model.UpdateStatus, names []string) ([]result, []model.UpdateStatus) {
	var results []result
	var due []model.UpdateStatus
	if len(names) == 0 {
		for _, st := range sts {
			if st.HasUpdate() && !autoupdate.ContainerExcluded(st.Container.Name, r.Exclude) {
				results = append(results, result{Name: st.Container.Name})
				due = append(due, st)
			}
		}
		return results, due
	}
	for _, name := range names {
		res := result{Name: name, Result: failed, Error: "ShipLog does not know this container"}
		for _, st := range sts {
			if !strings.EqualFold(st.Container.Name, name) {
				continue
			}
			res = result{Name: st.Container.Name, Result: current}
			if st.HasUpdate() {
				res.Result = ""
				due = append(due, st)
			}
		}
		results = append(results, res)
	}
	return results, due
}

func resultOf(o autoupdate.Outcome) (string, string) {
	msg := ""
	if o.Err != nil {
		msg = o.Err.Error()
	}
	switch {
	case o.Updated:
		return updated, ""
	case o.UpToDate:
		return current, ""
	case o.RolledBack:
		return rolledBack, msg
	case o.NoBackup || o.Skipped || o.Blocked:
		return held, msg
	}
	return failed, msg
}

func (r *Runs) finish(ru *run, results []result, err string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if results != nil {
		ru.Containers = results
	}
	ru.State, ru.Error = "done", err
	if err != "" {
		ru.State = "failed"
	}
}

func refuse(code string) error {
	return &fortress.Refusal{Status: http.StatusConflict, Code: code}
}

// runID reads a run id sent as a string or as a number.
func runID(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// newID is random, so a caller that asks after ShipLog restarted does not get
// another run's answer.
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
