package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

type fakeSource struct{ rows []model.UpdateStatus }

func (f fakeSource) List() ([]model.UpdateStatus, error) { return f.rows, nil }
func (f fakeSource) Get(id string) (model.UpdateStatus, error) {
	for _, r := range f.rows {
		if r.Container.ID == id {
			return r, nil
		}
	}
	return model.UpdateStatus{}, store.ErrNotFound
}

type fakeRefresher struct{ called int }

func (f *fakeRefresher) Refresh() { f.called++ }

func testAPI() (http.Handler, *fakeRefresher) {
	src := fakeSource{rows: []model.UpdateStatus{
		{Container: model.Container{ID: "a", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.2.0"},
			NewestTag: "1.4.0", Kind: model.KindMinor, Risk: model.RiskMedium},
	}}
	ref := &fakeRefresher{}
	return New(src, ref).Handler(), ref
}

func TestListJSON(t *testing.T) {
	h, _ := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/containers", nil))
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
	var out []model.UpdateStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Container.Name != "immich" {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestOneJSONFound404(t *testing.T) {
	h, _ := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/container/a", nil))
	if rr.Code != 200 {
		t.Fatalf("found: code %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/container/zzz", nil))
	if rr.Code != 404 {
		t.Fatalf("missing: want 404, got %d", rr.Code)
	}
}

func TestRefresh(t *testing.T) {
	h, ref := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/refresh", nil))
	if rr.Code != 202 || ref.called != 1 {
		t.Fatalf("refresh: code %d called %d", rr.Code, ref.called)
	}
}

func TestStatusPage(t *testing.T) {
	h, _ := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != 200 || !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status page: code %d type %q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "immich") {
		t.Fatal("status page missing container name")
	}
}

func TestUnknownPath404(t *testing.T) {
	h, _ := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/nope", nil))
	if rr.Code != 404 {
		t.Fatalf("unknown path: want 404, got %d", rr.Code)
	}
}
