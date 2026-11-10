package backup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeVault speaks the backup contract the way the backup app does: a request
// starts a run, and the run reports running until it has been asked about
// polls times.
type fakeVault struct {
	polls    int
	final    string // "done" or "failed"
	runID    any
	refuse   int
	asked    []map[string]string
	runsSeen int
}

func (v *fakeVault) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+RequestPath, func(w http.ResponseWriter, r *http.Request) {
		if v.refuse != 0 {
			w.WriteHeader(v.refuse)
			_, _ = w.Write([]byte(`{"error":"permission_off"}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var req map[string]string
		_ = json.Unmarshal(raw, &req)
		v.asked = append(v.asked, req)
		id := v.runID
		if id == nil {
			id = "run-8812"
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"run": id})
	})
	mux.HandleFunc("GET "+RunsPath+"{id}", func(w http.ResponseWriter, r *http.Request) {
		v.runsSeen++
		run := map[string]any{"id": r.PathValue("id"), "state": "running"}
		if v.runsSeen > v.polls {
			run["state"] = v.final
			if v.final == "done" {
				run["snapshot"] = "4f2a9c1e"
				run["link"] = "https://tower.lan:3443/snapshots/4f2a9c1e"
			} else {
				run["error"] = "repository locked"
			}
		}
		_ = json.NewEncoder(w).Encode(run)
	})
	return mux
}

// fakeFortress hands every call to the handler of the member it names.
type fakeFortress struct {
	members []Member
	serve   map[string]http.Handler
	calls   []string
}

func (f *fakeFortress) Members() []Member { return f.members }

func (f *fakeFortress) Call(_ context.Context, id, method, path string, body []byte) (int, []byte, error) {
	f.calls = append(f.calls, id+" "+method+" "+path)
	h, ok := f.serve[id]
	if !ok {
		return 0, nil, errors.New("unreachable")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(string(body))))
	return rec.Code, rec.Body.Bytes(), nil
}

func vaultMember(id, device string) Member {
	return Member{ID: id, App: "vault", Device: device, Online: true, Actions: []string{"backup.export", Action}}
}

func newClient(f Fortress) (*Client, *time.Duration) {
	var slept time.Duration
	c := &Client{Fortress: f, Device: "Tower", Timeout: 30 * time.Minute, Poll: 10 * time.Second}
	c.sleep = func(_ context.Context, d time.Duration) error {
		slept += d
		return nil
	}
	return c, &slept
}

func TestSaveBacksUpTheContainerAndWaitsForTheRun(t *testing.T) {
	v := &fakeVault{polls: 2, final: "done"}
	f := &fakeFortress{members: []Member{vaultMember("m1", "Tower")}, serve: map[string]http.Handler{"m1": v.handler()}}
	c, slept := newClient(f)

	snap, err := c.Save(context.Background(), "nextcloud")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.asked) != 1 || v.asked[0]["container"] != "nextcloud" || v.asked[0]["reason"] != "update" {
		t.Errorf("backup requests = %v, want one for nextcloud with reason update", v.asked)
	}
	want := Snapshot{Member: "m1", Run: "run-8812", ID: "4f2a9c1e", Link: "https://tower.lan:3443/snapshots/4f2a9c1e"}
	if snap != want {
		t.Errorf("snapshot = %+v, want %+v", snap, want)
	}
	if v.runsSeen != 3 || *slept != 20*time.Second {
		t.Errorf("asked about the run %d times and waited %s, want 3 times and 20s", v.runsSeen, *slept)
	}
	if f.calls[1] != "m1 GET /api/fortress/v1/runs/run-8812" {
		t.Errorf("second call = %q", f.calls[1])
	}
}

func TestSaveTakesANumericRunID(t *testing.T) {
	v := &fakeVault{final: "done", runID: 8812}
	f := &fakeFortress{members: []Member{vaultMember("m1", "Tower")}, serve: map[string]http.Handler{"m1": v.handler()}}
	c, _ := newClient(f)
	snap, err := c.Save(context.Background(), "nextcloud")
	if err != nil || snap.Run != "8812" {
		t.Fatalf("Save = %+v, %v, want run 8812", snap, err)
	}
}

func TestSaveWithoutABackupAppReportsErrNoMember(t *testing.T) {
	other := Member{ID: "m2", App: "loader", Device: "Tower", Online: true, Actions: []string{"jobs.run"}}
	f := &fakeFortress{members: []Member{other}}
	c, _ := newClient(f)
	if _, err := c.Save(context.Background(), "nextcloud"); !errors.Is(err, ErrNoMember) {
		t.Fatalf("err = %v, want ErrNoMember", err)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %v, want none", f.calls)
	}
}

func TestSaveFailsWhenTheBackupAppIsOffline(t *testing.T) {
	m := vaultMember("m1", "Tower")
	m.Online = false
	c, _ := newClient(&fakeFortress{members: []Member{m}})
	_, err := c.Save(context.Background(), "nextcloud")
	if err == nil || errors.Is(err, ErrNoMember) {
		t.Fatalf("err = %v, want a failure that is not ErrNoMember", err)
	}
}

func TestSaveFailsWhenTheBackupAppDoesNotAnswer(t *testing.T) {
	c, _ := newClient(&fakeFortress{members: []Member{vaultMember("m1", "Tower")}})
	if _, err := c.Save(context.Background(), "nextcloud"); err == nil || errors.Is(err, ErrNoMember) {
		t.Fatalf("err = %v, want a failure that is not ErrNoMember", err)
	}
}

func TestSaveFailsWhenTheRunFails(t *testing.T) {
	v := &fakeVault{polls: 1, final: "failed"}
	f := &fakeFortress{members: []Member{vaultMember("m1", "Tower")}, serve: map[string]http.Handler{"m1": v.handler()}}
	c, _ := newClient(f)
	_, err := c.Save(context.Background(), "nextcloud")
	if err == nil || !strings.Contains(err.Error(), "repository locked") {
		t.Fatalf("err = %v, want the run's error", err)
	}
}

func TestSaveFailsWhenTheBackupAppRefuses(t *testing.T) {
	v := &fakeVault{refuse: http.StatusForbidden}
	f := &fakeFortress{members: []Member{vaultMember("m1", "Tower")}, serve: map[string]http.Handler{"m1": v.handler()}}
	c, _ := newClient(f)
	_, err := c.Save(context.Background(), "nextcloud")
	if err == nil || !strings.Contains(err.Error(), "permission_off") {
		t.Fatalf("err = %v, want the refusal's code", err)
	}
}

func TestSaveGivesUpAfterTheTimeout(t *testing.T) {
	v := &fakeVault{polls: 1 << 20, final: "done"}
	f := &fakeFortress{members: []Member{vaultMember("m1", "Tower")}, serve: map[string]http.Handler{"m1": v.handler()}}
	c, slept := newClient(f)
	_, err := c.Save(context.Background(), "nextcloud")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if *slept != 30*time.Minute {
		t.Errorf("waited %s, want the 30m timeout", *slept)
	}
}

func TestSavePrefersTheBackupAppOnThisServer(t *testing.T) {
	here, there := &fakeVault{final: "done"}, &fakeVault{final: "done"}
	f := &fakeFortress{
		members: []Member{vaultMember("a", "Elsewhere"), vaultMember("b", "tower")},
		serve:   map[string]http.Handler{"a": there.handler(), "b": here.handler()},
	}
	c, _ := newClient(f)
	snap, err := c.Save(context.Background(), "nextcloud")
	if err != nil || snap.Member != "b" || len(there.asked) != 0 {
		t.Fatalf("Save = %+v, %v; the other server was asked %d times", snap, err, len(there.asked))
	}
}

func TestAvailableCountsABackupAppOnThisServerEvenWhileOffline(t *testing.T) {
	off := vaultMember("m1", "Tower")
	off.Online = false
	for _, tc := range []struct {
		members []Member
		want    bool
	}{
		{nil, false},
		{[]Member{vaultMember("a", "Elsewhere")}, false},
		{[]Member{off}, true},
		{[]Member{vaultMember("m1", "tower.lan")}, true},
	} {
		c, _ := newClient(&fakeFortress{members: tc.members})
		if got := c.Available(); got != tc.want {
			t.Errorf("Available with %+v = %v, want %v", tc.members, got, tc.want)
		}
	}
}

func TestSaveLeavesABackupAppOnAnotherServerAlone(t *testing.T) {
	there := &fakeVault{final: "done"}
	f := &fakeFortress{members: []Member{vaultMember("a", "Elsewhere"), vaultMember("c", "Third")},
		serve: map[string]http.Handler{"a": there.handler(), "c": there.handler()}}
	c, _ := newClient(f)
	if _, err := c.Save(context.Background(), "nextcloud"); !errors.Is(err, ErrNoMember) {
		t.Fatalf("err = %v, want ErrNoMember while no backup app runs on this server", err)
	}
}
