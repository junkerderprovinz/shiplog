package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"
)

func matrixChannel(hs, token string) kit.Channel {
	return kit.Channel{ID: "matrix", Kind: kit.KindMatrix, Enabled: true, On: kit.OnAlways,
		Matrix: &kit.Matrix{Homeserver: hs, Room: "!r:example.org", Token: token}}
}

func TestTheSettingsPageNeverSeesASecret(t *testing.T) {
	n, err := Open(filepath.Join(t.TempDir(), "notify.json"), []kit.Channel{matrixChannel("https://m.example.org", "syt_secret")}, kit.Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(n.Public())
	if strings.Contains(string(b), "syt_secret") || !strings.Contains(string(b), "https://m.example.org") {
		t.Errorf("Public = %s", b)
	}
}

func TestASavedListKeepsTheSecretItDidNotShow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	n, err := Open(path, []kit.Channel{matrixChannel("https://m.example.org", "syt_secret")}, kit.Options{})
	if err != nil {
		t.Fatal(err)
	}
	next := n.Public()
	next[0].Name = "Alerts"
	if _, err := n.Replace(next); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var kept []kit.Channel
	if err := json.Unmarshal(raw, &kept); err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].Name != "Alerts" || kept[0].Matrix.Token != "syt_secret" {
		t.Errorf("notify.json = %s", raw)
	}
}

func TestAnInvalidListChangesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.json")
	n, err := Open(path, nil, kit.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = n.Replace([]kit.Channel{{ID: "d", Kind: kit.KindDiscord, Enabled: true, Discord: &kit.Discord{URL: "not a url"}}})
	var ps kit.Problems
	if !errors.As(err, &ps) || len(ps) == 0 {
		t.Fatalf("Replace = %v, want problems", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an invalid list was written: %v", err)
	}
	if len(n.Public()) != 0 {
		t.Errorf("Public = %+v, want the old list", n.Public())
	}
}

func TestAReplacedListSendsAtOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	n, err := Open(filepath.Join(t.TempDir(), "notify.json"), nil, kit.Options{HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Send(context.Background(), NotApplied("planka")); err != nil || hits.Load() != 0 {
		t.Fatalf("Send without channels = %v, %d calls", err, hits.Load())
	}
	ch := kit.Channel{ID: "apprise", Kind: kit.KindApprise, Enabled: true, On: kit.OnAlways, Attempts: 1, Apprise: &kit.Apprise{URL: srv.URL + "/notify/k"}}
	if _, err := n.Replace([]kit.Channel{ch}); err != nil {
		t.Fatal(err)
	}
	if err := n.Send(context.Background(), NotApplied("planka")); err != nil || hits.Load() != 1 {
		t.Errorf("Send after the save = %v, %d calls, want one", err, hits.Load())
	}
}

func TestATestReachesAChannelNotYetSaved(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	n, err := Open(filepath.Join(t.TempDir(), "notify.json"), nil, kit.Options{HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ch := kit.Channel{ID: "apprise", Kind: kit.KindApprise, Enabled: true, Attempts: 1, Apprise: &kit.Apprise{URL: srv.URL + "/notify/k"}}
	if r := n.Test(context.Background(), ch); !r.OK() || hits.Load() != 1 {
		t.Errorf("Test = %+v after %d calls", r, hits.Load())
	}
}

func TestWithoutAFileTheListCannotBeSaved(t *testing.T) {
	n, err := Open("", nil, kit.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.Replace(nil); err == nil {
		t.Error("Replace without a file = nil")
	}
}
