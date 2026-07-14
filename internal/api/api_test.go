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

type fakeOverrides struct{ m map[string]string }

func (f *fakeOverrides) SourceOverrides() (map[string]string, error) { return f.m, nil }
func (f *fakeOverrides) SetSourceOverride(repo, source string) error {
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[repo] = source
	return nil
}
func (f *fakeOverrides) DeleteSourceOverride(repo string) error { delete(f.m, repo); return nil }

func testAPI() (http.Handler, *fakeRefresher) {
	src := fakeSource{rows: []model.UpdateStatus{
		{Container: model.Container{ID: "a", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.2.0"},
			NewestTag: "1.4.0", Kind: model.KindMinor, Risk: model.RiskMedium},
	}}
	ref := &fakeRefresher{}
	return New(src, &fakeOverrides{}, ref).Handler(), ref
}

func TestOverrideEndpoints(t *testing.T) {
	ovr := &fakeOverrides{}
	ref := &fakeRefresher{}
	h := New(fakeSource{}, ovr, ref).Handler()

	// PUT a valid override: bare owner/repo is normalised to a github URL.
	body := strings.NewReader(`{"repo":"lscr.io/linuxserver/radarr","source":"Radarr/Radarr"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", "/api/override", body))
	if rr.Code != 200 {
		t.Fatalf("PUT code %d: %s", rr.Code, rr.Body.String())
	}
	if got := ovr.m["lscr.io/linuxserver/radarr"]; got != "https://github.com/Radarr/Radarr" {
		t.Fatalf("stored source = %q", got)
	}
	if ref.called == 0 {
		t.Fatalf("PUT did not trigger a refresh")
	}

	// A non-GitHub source is rejected.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", "/api/override", strings.NewReader(`{"repo":"x/y","source":"gitlab.com/a/b"}`)))
	if rr.Code != 400 {
		t.Fatalf("bad source: want 400, got %d", rr.Code)
	}

	// GET lists it.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/overrides", nil))
	var m map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["lscr.io/linuxserver/radarr"] != "https://github.com/Radarr/Radarr" {
		t.Fatalf("GET overrides = %v", m)
	}

	// DELETE via query param clears it.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("DELETE", "/api/override?repo=lscr.io/linuxserver/radarr", nil))
	if rr.Code != 200 {
		t.Fatalf("DELETE code %d", rr.Code)
	}
	if _, ok := ovr.m["lscr.io/linuxserver/radarr"]; ok {
		t.Fatalf("override not deleted: %v", ovr.m)
	}
}

func TestStatusPageRenders(t *testing.T) {
	h, _ := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != 200 {
		t.Fatalf("status page code %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{`data-repo="ghcr.io/x/immich"`, "srcedit", "immich"} {
		if !strings.Contains(body, want) {
			t.Fatalf("status page missing %q", want)
		}
	}
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
	// The header must reference the served logo, not the old anchor glyph.
	if !strings.Contains(rr.Body.String(), `src="/logo.svg"`) {
		t.Error("status page header does not reference /logo.svg")
	}
	if strings.Contains(rr.Body.String(), "&#9875;") {
		t.Error("status page still carries the old anchor glyph")
	}
}

func TestLogo(t *testing.T) {
	h, _ := testAPI()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/logo.svg", nil))
	if rr.Code != 200 {
		t.Fatalf("logo: code %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "image/svg+xml") {
		t.Errorf("logo: Content-Type = %q, want image/svg+xml", ct)
	}
	if !strings.Contains(rr.Body.String(), "<svg") {
		t.Error("logo: body is not an SVG document")
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
