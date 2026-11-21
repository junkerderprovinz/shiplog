package summarize

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestNewLlamaSwap_DisabledWhenUnset(t *testing.T) {
	if NewLlamaSwap("", "") != nil {
		t.Error("expected nil when fully unconfigured")
	}
	if NewLlamaSwap("http://x", "") != nil {
		t.Error("expected nil without a model")
	}
	if NewLlamaSwap("", "gemma4-e4b") != nil {
		t.Error("expected nil without a url")
	}
}

func TestLlamaSwapSummarize_ParsesChatCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		// The model's JSON answer sits in choices[0].message.content.
		inner := `{"bullets":["new UI","faster scan"],"breaking":["needs PostgreSQL 15"],"risk":"medium, DB migration"}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": inner}},
			},
		})
	}))
	defer srv.Close()

	l := NewLlamaSwap(srv.URL, "gemma4-e4b")
	if l == nil {
		t.Fatal("NewLlamaSwap returned nil for a configured summariser")
	}
	sum, ok := l.Summarize(context.Background(), model.Container{Repo: "x/y"}, "1.0.0", "2.0.0", "## changelog\n- stuff")
	if !ok {
		t.Fatal("Summarize returned not-ok")
	}
	if len(sum.Bullets) != 2 || len(sum.Breaking) != 1 || sum.Breaking[0] != "needs PostgreSQL 15" || sum.Model != "gemma4-e4b" {
		t.Errorf("unexpected summary: %+v", sum)
	}
}

func TestLlamaSwapSummarize_EmptyRawSkips(t *testing.T) {
	l := NewLlamaSwap("http://127.0.0.1:1", "m")
	if _, ok := l.Summarize(context.Background(), model.Container{}, "a", "b", "   "); ok {
		t.Error("expected skip on empty raw")
	}
}

func TestLlamaSwapSummarize_EmptyJSONFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": `{"bullets":[],"breaking":[],"risk":""}`}}},
		})
	}))
	defer srv.Close()
	if _, ok := NewLlamaSwap(srv.URL, "m").Summarize(context.Background(), model.Container{}, "a", "b", "notes"); ok {
		t.Error("an answer with nothing in it must fall back to the raw changelog")
	}
}

func TestLlamaSwapPing_FindsModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "gemma4-e4b"}, {"id": "qwen3.5-4b"}},
		})
	}))
	defer srv.Close()

	if err := NewLlamaSwap(srv.URL, "gemma4-e4b").Ping(context.Background()); err != nil {
		t.Errorf("Ping with present model: %v", err)
	}
	if err := NewLlamaSwap(srv.URL, "missing").Ping(context.Background()); err == nil {
		t.Error("expected an error when the model is absent")
	}
}

func TestOpenRouterCheckAndSummary(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "openrouter.key")
	router := NewOpenRouter("openrouter/free", keyFile)
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer sk-or-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/key":
			w.WriteHeader(http.StatusOK)
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"openrouter/free"}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"risk\":\"low\"}"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	router.url = srv.URL
	ctx := context.Background()
	if err := router.Ping(ctx); err == nil || len(calls) != 0 {
		t.Fatal("missing key should fail locally")
	}
	if err := os.WriteFile(keyFile, []byte("sk-or-rejected"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := router.Ping(ctx); err == nil || len(calls) != 1 || calls[0] != "/v1/key" {
		t.Fatal("rejected key must fail before model listing")
	}
	calls = nil
	if err := os.WriteFile(keyFile, []byte("sk-or-test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := router.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "/v1/key" || calls[1] != "/v1/models" {
		t.Fatalf("unexpected check calls %v", calls)
	}
	sum, ok := router.Summarize(ctx, model.Container{Repo: "example/image"}, "1", "2", "Release notes")
	if !ok || sum.Risk != "low" {
		t.Fatal("expected a parsed summary")
	}
}
