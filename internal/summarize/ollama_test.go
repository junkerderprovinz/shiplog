package summarize

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestNew_DisabledWhenUnset(t *testing.T) {
	if New("", "") != nil {
		t.Error("expected nil when fully unconfigured")
	}
	if New("http://x", "") != nil {
		t.Error("expected nil without a model")
	}
	if New("", "llama3") != nil {
		t.Error("expected nil without a url")
	}
}

func TestSummarize_ParsesOllamaJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		// Ollama wraps the model's JSON answer in {"response": "..."}.
		inner := `{"bullets":["new UI","faster scan"],"breaking":["needs PostgreSQL 15"],"risk":"medium — DB migration"}`
		_ = json.NewEncoder(w).Encode(map[string]any{"response": inner, "done": true})
	}))
	defer srv.Close()

	o := New(srv.URL, "llama3")
	if o == nil {
		t.Fatal("New returned nil for a configured summariser")
	}
	sum, ok := o.Summarize(context.Background(), model.Container{Repo: "x/y"}, "1.0.0", "2.0.0", "## changelog\n- stuff")
	if !ok {
		t.Fatal("Summarize returned not-ok")
	}
	if len(sum.Bullets) != 2 || len(sum.Breaking) != 1 || sum.Breaking[0] != "needs PostgreSQL 15" || sum.Model != "llama3" {
		t.Errorf("unexpected summary: %+v", sum)
	}
}

func TestSummarize_EmptyRawSkips(t *testing.T) {
	o := New("http://127.0.0.1:1", "m")
	if _, ok := o.Summarize(context.Background(), model.Container{}, "a", "b", "   "); ok {
		t.Error("expected skip on empty raw")
	}
}

func TestPing_FindsModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]string{{"name": "llama3:latest"}},
		})
	}))
	defer srv.Close()

	if err := New(srv.URL, "llama3").Ping(context.Background()); err != nil {
		t.Errorf("Ping with present model: %v", err)
	}
	if err := New(srv.URL, "missing").Ping(context.Background()); err == nil {
		t.Error("expected an error when the model is absent")
	}
}
