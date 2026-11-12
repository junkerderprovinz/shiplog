package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/fortress"
)

type fakeDoor struct {
	state   fortress.State
	words   string
	removed string
	err     error
}

func (d *fakeDoor) State() fortress.State { return d.state }

func (d *fakeDoor) Enter(words string) error {
	d.words = words
	if d.err == nil {
		d.state = fortress.State{Joined: true, Device: "Tower"}
	}
	return d.err
}

func (d *fakeDoor) Remove(id string) error {
	if d.err == nil {
		d.removed = id
		d.state.RemovedMembers = append(d.state.RemovedMembers, fortress.Removed{App: "tollroll", Name: "TollRoll", Device: "Bottich"})
	}
	return d.err
}

func doorCall(a *API, method, path, body string, socket bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if socket {
		r = r.WithContext(onSocket(r.Context()))
	}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, r)
	return rr
}

func TestThePageHearsThatShipLogWasTakenOut(t *testing.T) {
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(&fakeDoor{state: fortress.State{Removed: true}})
	rr := doorCall(a, http.MethodGet, "/api/fortress", "", true)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"removed":true`) || !strings.Contains(rr.Body.String(), `"joined":false`) {
		t.Errorf("state = %d %s", rr.Code, rr.Body)
	}
}

func TestThePhraseJoinsShipLog(t *testing.T) {
	d := &fakeDoor{state: fortress.State{Removed: true}}
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(d)
	rr := doorCall(a, http.MethodPost, "/api/fortress/join", `{"phrase":"legal winner thank year"}`, true)
	if rr.Code != http.StatusOK || d.words != "legal winner thank year" || !strings.Contains(rr.Body.String(), `"joined":true`) {
		t.Errorf("join = %d %s, words %q", rr.Code, rr.Body, d.words)
	}
}

func TestAJoinThatFailsSaysWhyWithACode(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		body string
	}{
		{fortress.ErrPhrase, http.StatusBadRequest, `"error":"phrase"`},
		{fortress.ErrMember, http.StatusConflict, `"error":"member"`},
	} {
		a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(&fakeDoor{err: c.err})
		rr := doorCall(a, http.MethodPost, "/api/fortress/join", `{"phrase":"x"}`, true)
		if rr.Code != c.code || !strings.Contains(rr.Body.String(), c.body) {
			t.Errorf("%v: %d %s", c.err, rr.Code, rr.Body)
		}
	}
}

func TestTheFortressAnswersOnlyOnTheSocket(t *testing.T) {
	d := &fakeDoor{}
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(d)
	if rr := doorCall(a, http.MethodGet, "/api/fortress", "", false); rr.Code != http.StatusForbidden {
		t.Errorf("state over the network = %d", rr.Code)
	}
	if rr := doorCall(a, http.MethodPost, "/api/fortress/join", `{"phrase":"x"}`, false); rr.Code != http.StatusForbidden || d.words != "" {
		t.Errorf("join over the network = %d, words %q", rr.Code, d.words)
	}
}

func TestThePageRemovesAnotherMember(t *testing.T) {
	d := &fakeDoor{state: fortress.State{Joined: true, Device: "Tower"}}
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(d)
	rr := doorCall(a, http.MethodDelete, "/api/fortress/members/5f3a", "", true)
	if rr.Code != http.StatusOK || d.removed != "5f3a" || !strings.Contains(rr.Body.String(), `"removedMembers":[{"app":"tollroll","name":"TollRoll","device":"Bottich"}]`) {
		t.Errorf("remove = %d %s, removed %q", rr.Code, rr.Body, d.removed)
	}
}

func TestARemovalThatFailsSaysWhyWithACode(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		body string
	}{
		{fortress.ErrNoMember, http.StatusNotFound, `"error":"noMember"`},
		{fortress.ErrNotMember, http.StatusConflict, `"error":"noFortress"`},
	} {
		a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(&fakeDoor{err: c.err})
		rr := doorCall(a, http.MethodDelete, "/api/fortress/members/5f3a", "", true)
		if rr.Code != c.code || !strings.Contains(rr.Body.String(), c.body) {
			t.Errorf("%v: %d %s", c.err, rr.Code, rr.Body)
		}
	}
}

func TestOnlyTheSocketRemovesAMember(t *testing.T) {
	d := &fakeDoor{state: fortress.State{Joined: true}}
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithDoor(d)
	if rr := doorCall(a, http.MethodDelete, "/api/fortress/members/5f3a", "", false); rr.Code != http.StatusForbidden || d.removed != "" {
		t.Errorf("remove over the network = %d, removed %q", rr.Code, d.removed)
	}
}
