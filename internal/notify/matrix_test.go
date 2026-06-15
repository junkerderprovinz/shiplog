package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestNew_DisabledWhenUnset(t *testing.T) {
	if New("", "", "") != nil {
		t.Error("expected nil when fully unconfigured")
	}
	if New("https://hs", "tok", "") != nil {
		t.Error("expected nil without a room")
	}
	if New("https://hs", "", "#r:hs") != nil {
		t.Error("expected nil without a token")
	}
}

func TestNotify_ResolvesAliasAndSends(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/directory/room/"):
			_ = json.NewEncoder(w).Encode(map[string]string{"room_id": "!abc:hs"})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/send/m.room.message/"):
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "$x"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(srv.URL, "tok", "#room:hs")
	if m == nil {
		t.Fatal("New returned nil for a configured notifier")
	}
	st := model.UpdateStatus{
		Container: model.Container{ID: "containerid123456", Name: "immich", Tag: "1.0",
			Source: "https://github.com/x/immich"},
		NewestTag: "1.2", Risk: model.RiskMedium,
	}
	if err := m.Notify(context.Background(), st); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("authorization = %q, want Bearer tok", gotAuth)
	}
	if !strings.Contains(gotPath, "!abc:hs") {
		t.Errorf("send path should use the resolved room id, got %q", gotPath)
	}
	if !strings.Contains(gotBody, "immich") || !strings.Contains(gotBody, "formatted_body") {
		t.Errorf("message body missing name or html: %q", gotBody)
	}
}

func TestWhoami(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/account/whoami") {
			_ = json.NewEncoder(w).Encode(map[string]string{"user_id": "@bot:hs"})
			return
		}
		http.NotFound(w, r)
	}))
	defer ok.Close()
	if err := New(ok.URL, "tok", "!r:hs").Whoami(context.Background()); err != nil {
		t.Errorf("Whoami on a good token: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer bad.Close()
	if err := New(bad.URL, "tok", "!r:hs").Whoami(context.Background()); err == nil {
		t.Error("Whoami should error on a 401")
	}
}
