package api

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/status"
	"github.com/junkerderprovinz/parleyport/kit/statustest"

	"github.com/junkerderprovinz/shiplog/internal/fortress"
	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

// fortressAPI builds the real API over a store with an update that waits, a
// risky one and two auto-updates in the log.
func fortressAPI(t *testing.T) *API {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "shiplog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	at := time.Date(2026, 11, 9, 4, 0, 0, 0, time.UTC)
	for _, st := range []model.UpdateStatus{
		{Container: model.Container{ID: "a1", Name: "plex", Tag: "1.40.0"}, NewestTag: "1.41.0", Kind: model.KindMinor, Risk: model.RiskMedium, CheckedAt: at},
		{Container: model.Container{ID: "b2", Name: "nextcloud", Tag: "27.1.4", Source: "https://github.com/nextcloud/server"}, NewestTag: "28.0.0", Kind: model.KindMajor, Risk: model.RiskHigh, CheckedAt: at},
	} {
		if err := db.Upsert(st); err != nil {
			t.Fatal(err)
		}
	}
	for _, rec := range []store.AutoUpdateRecord{
		{Name: "radarr", FromVer: "5.1.0", ToVer: "5.1.1", Level: "patch", Success: true, At: at.Unix()},
		{Name: "sonarr", FromVer: "4.0.0", ToVer: "4.0.1", Level: "patch", Err: "pull error", At: at.Add(time.Minute).Unix()},
	} {
		if err := db.LogAutoUpdate(rec); err != nil {
			t.Fatal(err)
		}
	}
	src := &fortress.Source{
		App:      status.App{ID: "shiplog", Name: "ShipLog", Version: "3.8.0"},
		Instance: status.Instance{ID: "4f2a9c", Name: "Tower"},
		Store:    db,
		Run:      &fortress.Run{},
		Now:      time.Now,
	}
	return New(db, db, &fakeRefresher{}, "").WithFortress(src)
}

func TestStatusContract(t *testing.T) {
	statustest.Run(t, fortressAPI(t).Handler(), statustest.Options{
		Authenticate: func(r *http.Request) { *r = *r.WithContext(onSocket(r.Context())) },
	})
}

// shortSocket returns a socket path short enough for the 108 bytes a unix
// socket path may have, which a test's temp dir can exceed.
func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

func serveOn(t *testing.T, a *API, ln net.Listener) {
	t.Helper()
	srv := a.Server()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

func TestTheStatusAnswersOnTheSocket(t *testing.T) {
	sock := shortSocket(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	serveOn(t, fortressAPI(t), ln)
	hc := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}}
	for _, path := range []string{status.StatusPath, status.LogPath} {
		resp, err := hc.Get("http://shiplog" + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s on the socket: %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestTheStatusRefusesATCPConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveOn(t, fortressAPI(t), ln)
	hc := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{status.StatusPath, status.LogPath} {
		req, _ := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+path, nil)
		// What the proxy would add must not open the door on TCP.
		req.Header.Set("X-ShipLog-Proxy", "/plugins/shiplog/server/status.php")
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s over TCP: %d, want 401", path, resp.StatusCode)
		}
	}
}
