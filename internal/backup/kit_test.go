package backup

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
)

type fakeKit struct {
	members []fortress.Member
	acts    []string
	answer  []byte
	err     error
}

func (k *fakeKit) Members() []fortress.Member { return k.members }

func (k *fakeKit) Act(_ context.Context, id, action string, body []byte) ([]byte, error) {
	k.acts = append(k.acts, id+" "+action+" "+string(body))
	return k.answer, k.err
}

func TestKitCarriesBothCallsAsTheBackupAction(t *testing.T) {
	k := &fakeKit{answer: []byte(`{"run":"7"}`)}
	f := Kit{Fortress: k}
	code, out, err := f.Call(context.Background(), "m1", http.MethodPost, RequestPath, []byte(`{"container":"web","reason":"update"}`))
	if err != nil || code != http.StatusOK || string(out) != `{"run":"7"}` {
		t.Fatalf("request = %d %s %v", code, out, err)
	}
	if _, _, err := f.Call(context.Background(), "m1", http.MethodGet, RunsPath+"7", nil); err != nil {
		t.Fatal(err)
	}
	want := []string{`m1 backup.request {"container":"web","reason":"update"}`, `m1 backup.request {"run":"7"}`}
	if len(k.acts) != 2 || k.acts[0] != want[0] || k.acts[1] != want[1] {
		t.Errorf("actions = %q, want %q", k.acts, want)
	}
}

func TestKitAnswersARefusalWithItsStatusAndCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		body string
	}{
		{&fortress.Refusal{Status: http.StatusConflict, Code: "not_backed_up"}, http.StatusConflict, `{"error":"not_backed_up"}`},
		{fortress.ErrPermissionOff, http.StatusForbidden, `{"error":"permission_off"}`},
		{fortress.ErrNoSuchAction, http.StatusNotFound, `{"error":"no_such_action"}`},
	} {
		code, out, err := Kit{Fortress: &fakeKit{err: tc.err}}.Call(context.Background(), "m1", http.MethodPost, RequestPath, nil)
		if err != nil || code != tc.code || string(out) != tc.body {
			t.Errorf("%v: got %d %s %v, want %d %s", tc.err, code, out, err, tc.code, tc.body)
		}
	}
}

func TestKitPassesOnAMemberThatDoesNotAnswer(t *testing.T) {
	gone := errors.New("unreachable")
	if _, _, err := (Kit{Fortress: &fakeKit{err: gone}}).Call(context.Background(), "m1", http.MethodGet, RunsPath+"7", nil); !errors.Is(err, gone) {
		t.Fatalf("err = %v, want it passed on", err)
	}
}

func TestKitKnowsNoOtherRoute(t *testing.T) {
	k := &fakeKit{}
	code, _, err := Kit{Fortress: k}.Call(context.Background(), "m1", http.MethodDelete, RunsPath+"7", nil)
	if err != nil || code != http.StatusNotFound || len(k.acts) != 0 {
		t.Fatalf("got %d %v with %d actions, want 404 and none", code, err, len(k.acts))
	}
}

func TestKitListsInstancesOnly(t *testing.T) {
	k := &fakeKit{members: []fortress.Member{
		{ID: "m1", Kind: member.KindInstance, App: fortress.App{ID: "vault"}, Device: "Tower", Relay: true, Actions: []string{Action}},
		{ID: "v1", Kind: member.KindViewer, Device: "Tower", Relay: true},
		{ID: "m2", Kind: member.KindInstance, App: fortress.App{ID: "vault"}, Device: "Tower"},
	}}
	got := Kit{Fortress: k}.Members()
	if len(got) != 2 || got[0].ID != "m1" || !got[0].Online || got[0].App != "vault" || got[0].Actions[0] != Action || got[1].Online {
		t.Fatalf("members = %+v", got)
	}
}
