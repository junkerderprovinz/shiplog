package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// fakeFortress answers notify.deliver per member id and keeps what it got.
type fakeFortress struct {
	members []fortress.Member
	answer  map[string]func() ([]byte, error)
	calls   []string
	bodies  []map[string]any
}

func (f *fakeFortress) Members() []fortress.Member { return f.members }

func (f *fakeFortress) Act(ctx context.Context, id, action string, body []byte) ([]byte, error) {
	f.calls = append(f.calls, id+" "+action)
	var b map[string]any
	_ = json.Unmarshal(body, &b)
	f.bodies = append(f.bodies, b)
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("called without a timeout")
	}
	return f.answer[id]()
}

func tollroll(id string, online bool, actions ...string) fortress.Member {
	return fortress.Member{ID: id, Kind: member.KindInstance, App: fortress.App{ID: "tollroll"}, Direct: online, Actions: actions}
}

func taken() ([]byte, error) { return []byte(`{"taken":true}`), nil }

func hubOff() ([]byte, error) {
	return nil, &fortress.Refusal{Status: http.StatusConflict, Code: "hub_off"}
}

// ownChannel counts what ShipLog sends on its own.
func ownChannel(t *testing.T) (*Notifier, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { n.Add(1) }))
	t.Cleanup(srv.Close)
	notifier, err := New([]kit.Channel{{ID: "apprise", Kind: kit.KindApprise, Enabled: true, On: kit.OnAlways, Attempts: 1,
		Apprise: &kit.Apprise{URL: srv.URL + "/notify/k"}}}, kit.Options{HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return notifier, &n
}

var planka = model.UpdateStatus{
	Container: model.Container{Name: "planka", Tag: "2.0.0", Source: "https://github.com/plankanban/planka"},
	NewestTag: "2.1.0", NewestDigest: "sha256:0123456789abcdef0123", Kind: model.KindMinor, Risk: model.RiskMedium,
}

func TestTollRollSendsWhatItTakes(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": taken}}
	n := WithHub(own, &Hub{Fortress: f})
	if err := n.Notify(context.Background(), planka); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 0 {
		t.Errorf("ShipLog sent %d messages itself, want none while TollRoll takes them", sent.Load())
	}
	if len(f.calls) != 1 || f.calls[0] != "tr notify.deliver" {
		t.Fatalf("calls = %v", f.calls)
	}
	b := f.bodies[0]
	want := map[string]any{"kind": "update.available", "severity": "info", "title": "planka: update available",
		"body": "2.0.0 → 2.1.0 (risk: MEDIUM)", "link": "https://github.com/plankanban/planka", "key": "update:planka@0123456789ab"}
	for k, v := range want {
		if b[k] != v {
			t.Errorf("%s = %v, want %v", k, b[k], v)
		}
	}
}

func TestShipLogSendsItselfWhenTollRollDoesNotTakeIt(t *testing.T) {
	for name, answer := range map[string]func() ([]byte, error){
		"hub off":        hubOff,
		"permission off": func() ([]byte, error) { return nil, fortress.ErrPermissionOff },
		"timeout":        func() ([]byte, error) { return nil, context.DeadlineExceeded },
		"other answer":   func() ([]byte, error) { return []byte(`{"taken":false}`), nil },
	} {
		own, sent := ownChannel(t)
		f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": answer}}
		if err := WithHub(own, &Hub{Fortress: f}).Send(context.Background(), NotApplied("planka")); err != nil {
			t.Fatal(err)
		}
		if sent.Load() != 1 {
			t.Errorf("%s: ShipLog sent %d messages itself, want 1", name, sent.Load())
		}
	}
}

func TestOnlyAnOnlineTollRollWithTheActionIsAsked(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{members: []fortress.Member{
		tollroll("away", false, DeliverAction),
		tollroll("old", true),
		{ID: "bv", Kind: member.KindInstance, App: fortress.App{ID: "bombvault"}, Direct: true, Actions: []string{DeliverAction}},
	}}
	if err := WithHub(own, &Hub{Fortress: f}).Send(context.Background(), NotApplied("planka")); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 || sent.Load() != 1 {
		t.Errorf("calls = %v, own messages = %d; want no call and ShipLog sending itself", f.calls, sent.Load())
	}
}

func TestTheNextTollRollIsAskedWhenOneRefuses(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{
		members: []fortress.Member{tollroll("a", true, DeliverAction), tollroll("b", true, DeliverAction)},
		answer:  map[string]func() ([]byte, error){"a": hubOff, "b": taken},
	}
	if err := WithHub(own, &Hub{Fortress: f}).Send(context.Background(), NotApplied("planka")); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || sent.Load() != 0 {
		t.Errorf("calls = %v, own messages = %d; want b to take it", f.calls, sent.Load())
	}
}

func TestTheHubAloneCarriesMessagesWithoutOwnChannels(t *testing.T) {
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": taken}}
	n := WithHub(nil, &Hub{Fortress: f})
	if n == nil {
		t.Fatal("WithHub without own channels = nil")
	}
	if err := n.Send(context.Background(), Summary("Auto-updated 1: planka", "", false)); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.bodies[0]["severity"] != "success" || f.bodies[0]["kind"] != "autoupdate.run" {
		t.Errorf("calls = %v, bodies = %v", f.calls, f.bodies)
	}
	f.answer["tr"] = hubOff
	if err := n.Send(context.Background(), NotApplied("planka")); err != nil {
		t.Errorf("a refused message without own channels fails: %v", err)
	}
}

func TestWithoutAHubNothingChanges(t *testing.T) {
	own, _ := ownChannel(t)
	if WithHub(own, nil) != own || WithHub(nil, nil) != nil {
		t.Error("WithHub without a hub changes the notifier")
	}
}

func TestAKeyStaysWithinWhatTollRollTakes(t *testing.T) {
	st := planka
	st.Container.Name = "a-container-with-a-really-long-name-that-goes-on-and-on-and-on-and-on-and-on-forever"
	if k := update(st).Key; len(k) > 100 || k == "" {
		t.Errorf("key %q has %d bytes, want 1 to 100", k, len(k))
	}
}

func TestTheHubIsActiveWhileTollRollSaysSo(t *testing.T) {
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}}
	caps := []string{}
	h := &Hub{Fortress: f, Report: func(_ context.Context, id string) (status.Report, error) {
		return status.Report{Capabilities: caps}, nil
	}}
	if h.Active(context.Background()) {
		t.Error("Active without the capability")
	}
	caps = []string{HubCapability}
	if !h.Active(context.Background()) {
		t.Error("Active = false while TollRoll lists the capability")
	}
	f.members[0].Direct = false
	if h.Active(context.Background()) {
		t.Error("Active for a TollRoll that is away")
	}
}

func TestAnAppliedUpdateClosesItsMessage(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": taken}}
	WithHub(own, &Hub{Fortress: f}).Resolve(context.Background(), planka)
	if len(f.bodies) != 1 || f.bodies[0]["key"] != "update:planka@0123456789ab" || f.bodies[0]["resolved"] != true {
		t.Errorf("bodies = %v, want the update's key resolved", f.bodies)
	}
	if sent.Load() != 0 {
		t.Errorf("ShipLog sent %d messages itself, want none for a resolved key", sent.Load())
	}
	own.Resolve(context.Background(), planka)
}

// TollRoll keeps one open message per update, so it hears of each one.
func TestTollRollHearsOfEachUpdateOfABatch(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": taken}}
	n := WithHub(own, &Hub{Fortress: f, Report: func(context.Context, string) (status.Report, error) {
		return status.Report{Capabilities: []string{HubCapability}}, nil
	}})
	other := planka
	other.Container.Name, other.NewestDigest = "other", "sha256:fedcba9876543210"
	if err := n.NotifyBatch(context.Background(), []model.UpdateStatus{planka, other}); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 2 || f.bodies[0]["key"] == f.bodies[1]["key"] || sent.Load() != 0 {
		t.Errorf("tollroll got %v, own channels %d, want two keyed messages and nothing of its own", f.bodies, sent.Load())
	}
}

func TestABatchGoesToTheOwnChannelsWithoutATollRoll(t *testing.T) {
	own, sent := ownChannel(t)
	n := WithHub(own, &Hub{Fortress: &fakeFortress{}, Report: func(context.Context, string) (status.Report, error) {
		return status.Report{}, nil
	}})
	other := planka
	other.Container.Name = "other"
	if err := n.NotifyBatch(context.Background(), []model.UpdateStatus{planka, other}); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 1 {
		t.Errorf("own channels got %d messages, want one batch", sent.Load())
	}
}

// Without a status report TollRoll may still take the messages, and each
// update it takes keeps its key.
func TestTollRollTakesABatchWhenItsStatusReportFails(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": taken}}
	n := WithHub(own, &Hub{Fortress: f, Report: func(context.Context, string) (status.Report, error) {
		return status.Report{}, context.DeadlineExceeded
	}})
	other := planka
	other.Container.Name, other.NewestDigest = "other", "sha256:fedcba9876543210"
	if err := n.NotifyBatch(context.Background(), []model.UpdateStatus{planka, other}); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 2 || f.bodies[0]["key"] == "" || sent.Load() != 0 {
		t.Errorf("tollroll got %v, own channels %d, want two keyed messages and nothing of its own", f.bodies, sent.Load())
	}
}

func TestWhatTollRollRefusesGoesOutAsOneBatch(t *testing.T) {
	own, sent := ownChannel(t)
	f := &fakeFortress{members: []fortress.Member{tollroll("tr", true, DeliverAction)}, answer: map[string]func() ([]byte, error){"tr": hubOff}}
	n := WithHub(own, &Hub{Fortress: f})
	other := planka
	other.Container.Name = "other"
	if err := n.NotifyBatch(context.Background(), []model.UpdateStatus{planka, other}); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 1 {
		t.Errorf("own channels got %d messages, want one batch", sent.Load())
	}
}
