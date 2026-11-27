package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/plugins"
)

type fakePlugins struct {
	list []plugins.Plugin
	err  error
}

func (f fakePlugins) Scan() ([]plugins.Plugin, error) { return f.list, f.err }

func pluginsGET(t *testing.T, a *API) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/plugins", nil))
	return rec
}

func TestThePluginsAreListedOnUnraid(t *testing.T) {
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithPlugins(fakePlugins{list: []plugins.Plugin{
		{File: "unassigned.devices.plg", Name: "unassigned.devices", Installed: "2025.08.07", Latest: "2026.10.12", State: plugins.StateAvailable},
	}})
	rec := pluginsGET(t, a)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got []plugins.Plugin
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != plugins.StateAvailable || got[0].Latest != "2026.10.12" {
		t.Errorf("plugins = %+v", got)
	}
}

func TestAFailedPluginScanIsAServerError(t *testing.T) {
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithPlugins(fakePlugins{err: errors.New("no dir")})
	if rec := pluginsGET(t, a); rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rec.Code)
	}
}

// Off Unraid there are no plugins, and the route is not there at all.
func TestWithoutUnraidThereIsNoPluginsRoute(t *testing.T) {
	a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "")
	if rec := pluginsGET(t, a); rec.Code == http.StatusOK && rec.Header().Get("Content-Type") == "application/json" {
		t.Errorf("status %d, %s", rec.Code, rec.Body)
	}
}
