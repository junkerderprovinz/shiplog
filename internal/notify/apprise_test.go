package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestNewApprise_DisabledWithoutHTTPURL(t *testing.T) {
	for _, target := range []string{"", "   ", "apprise.local:8000/notify/x", "mailto://me@example.com", "http://"} {
		if NewApprise(target) != nil {
			t.Errorf("NewApprise(%q) should be nil", target)
		}
	}
	if NewApprise(" http://apprise:8000/notify/shiplog ") == nil {
		t.Error("NewApprise should accept an http URL")
	}
}

func TestApprise_PostsTheUpdateText(t *testing.T) {
	var gotPath, gotType string
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotType = r.URL.Path, r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	a := NewApprise(srv.URL + "/notify/shiplog")
	st := model.UpdateStatus{
		Container: model.Container{Name: "immich", Tag: "1.0"},
		NewestTag: "1.1",
		Risk:      model.RiskLevel("minor"),
	}
	if err := a.Notify(context.Background(), st); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	want, _ := format(st)
	if gotPath != "/notify/shiplog" || gotType != "application/json" {
		t.Errorf("posted to %q as %q", gotPath, gotType)
	}
	if got["body"] != want || got["type"] != "info" || got["format"] != "text" {
		t.Errorf("payload = %v, want body %q", got, want)
	}
}

func TestApprise_FailureNamesTheServerButNotTheKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFailedDependency)
	}))
	a := NewApprise(srv.URL + "/notify/s3cretkey")
	err := a.SendMessage(context.Background(), "hello", "")
	if err == nil || !strings.Contains(err.Error(), "424") {
		t.Fatalf("want a 424 error, got %v", err)
	}
	if strings.Contains(err.Error(), "s3cretkey") || !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error = %q", err)
	}

	srv.Close()
	err = a.SendMessage(context.Background(), "hello", "")
	if err == nil || strings.Contains(err.Error(), "s3cretkey") {
		t.Errorf("unreachable server error = %v", err)
	}
}

func TestApprise_WhereDropsTheLogin(t *testing.T) {
	a := NewApprise("https://user:pass@apprise.example:8443/notify/key")
	if got := a.Where(); got != "https://apprise.example:8443" {
		t.Errorf("Where() = %q", got)
	}
}

func TestApprise_NilDoesNothing(t *testing.T) {
	var a *Apprise
	if err := a.Notify(context.Background(), model.UpdateStatus{}); err != nil {
		t.Error(err)
	}
}
