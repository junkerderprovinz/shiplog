package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/store"
)

type fakeGuards []store.Guard

func (f fakeGuards) Guards() ([]store.Guard, error) { return f, nil }

func TestGuardsTellTheDockerTabAboutBackups(t *testing.T) {
	guards := fakeGuards{
		{Name: "mariadb", State: store.GuardNoBackup, Reason: "the backup failed: locked", At: 100},
		{Name: "nextcloud", State: store.GuardRolledBack, Repository: "lscr.io/linuxserver/nextcloud", Digest: "sha256:a",
			Snapshot: "4f2a9c1e", Reason: "unhealthy after the update", At: 200},
	}
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithGuards(guards, func() bool { return true })
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/guards", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got struct {
		Backup bool `json:"backup"`
		Guards []struct {
			Name     string `json:"name"`
			State    string `json:"state"`
			Reason   string `json:"reason"`
			Snapshot string `json:"snapshot"`
			Link     string `json:"link"`
			Pinned   string `json:"pinned"`
			Since    string `json:"since"`
		} `json:"guards"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Backup || len(got.Guards) != 2 {
		t.Fatalf("answer = %+v", got)
	}
	nc := got.Guards[1]
	if nc.State != "rolled_back" || nc.Link != "" || nc.Pinned != "lscr.io/linuxserver/nextcloud@sha256:a" || nc.Since != "1970-01-01T00:03:20Z" {
		t.Errorf("rolled back guard = %+v", nc)
	}
}

func TestGuardsWithoutAFortressSayThereIsNoBackup(t *testing.T) {
	rr := httptest.NewRecorder()
	New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/guards", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "{\"backup\":false,\"guards\":[]}\n" {
		t.Fatalf("answer = %d %s", rr.Code, rr.Body)
	}
}
