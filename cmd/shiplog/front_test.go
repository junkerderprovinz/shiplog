package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/config"
)

func engineStub() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("engine")) })
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestThePluginSocketHasNoSignIn(t *testing.T) {
	h, closer, err := front(config.Config{Socket: "/var/run/shiplog.sock", DataDir: t.TempDir()}, engineStub())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer() }()
	if w := get(t, h, "/api/signin/v1/options"); w.Body.String() != "engine" {
		t.Errorf("the socket answers the sign-in: %d %s", w.Code, w.Body)
	}
}

func TestThePortOpensWithTheSignInAndNoPassword(t *testing.T) {
	h, closer, err := front(config.Config{Port: "8484", DataDir: t.TempDir()}, engineStub())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer() }()
	if w := get(t, h, "/api/signin/v1/options"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"open":true`) {
		t.Errorf("options on the port: %d %s", w.Code, w.Body)
	}
	if w := get(t, h, "/api/containers"); w.Body.String() != "engine" {
		t.Errorf("an open ShipLog does not reach the engine: %d %s", w.Code, w.Body)
	}
}
