package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// testKey is long enough to pass for a key and plainly is not one.
var testKey = "sk-or-v1-" + strings.Repeat("x", 28) + "cdef"

// fakeOpenRouter answers like openrouter.ai/api/v1 and records what reached it.
type fakeOpenRouter struct {
	mu       sync.Mutex
	auth     map[string]string // path -> Authorization header
	bodies   []string
	keyCode  int    // status of /key, 200 when 0
	chatCode int    // status of /chat/completions, 200 when 0
	answer   string // content of the chat answer
	limit    string // limit_remaining of /key, null when ""
	echo     bool   // error bodies repeat the Authorization header
	served   string // model of the chat answer, absent when ""
	raw      string // metadata.raw of a chat error, absent when ""
	cutOff   bool   // the chat answer is empty, cut off at its token limit
}

func (f *fakeOpenRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.auth == nil {
		f.auth = map[string]string{}
	}
	f.auth[r.URL.Path] = r.Header.Get("Authorization")
	fail := func(code int) {
		msg := "refused"
		if f.echo {
			msg = "refused " + r.Header.Get("Authorization")
		}
		e := map[string]any{"code": code, "message": msg}
		if f.raw != "" {
			e["message"] = "Provider returned error"
			e["metadata"] = map[string]any{"raw": f.raw, "provider_name": "Google AI Studio"}
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": e})
	}
	switch r.URL.Path {
	case "/api/v1/models":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
			{"id": "openai/gpt-oss-20b:free", "name": "gpt-oss-20b (free)"},
			{"id": "z-ai/glm-4.6", "name": "GLM 4.6"},
		}})
	case "/api/v1/key":
		if f.keyCode != 0 && f.keyCode != 200 {
			fail(f.keyCode)
			return
		}
		limit := "null"
		if f.limit != "" {
			limit = f.limit
		}
		_, _ = fmt.Fprintf(w, `{"data":{"label":"sk-or-v1-012...def","limit_remaining":%s,"is_free_tier":false}}`, limit)
	case "/api/v1/chat/completions":
		var b bytes.Buffer
		_, _ = b.ReadFrom(r.Body)
		f.bodies = append(f.bodies, b.String())
		if f.chatCode != 0 && f.chatCode != 200 {
			fail(f.chatCode)
			return
		}
		answer := f.answer
		if answer == "" {
			answer = `{"bullets":["Adds a dark mode."],"breaking":[],"risk":"Low."}`
		}
		choice := map[string]any{"message": map[string]string{"content": answer}}
		if f.cutOff {
			choice = map[string]any{"finish_reason": "length", "message": map[string]string{"content": ""}}
		}
		resp := map[string]any{"choices": []map[string]any{choice}}
		if f.served != "" {
			resp["model"] = f.served
		}
		_ = json.NewEncoder(w).Encode(resp)
	default:
		http.NotFound(w, r)
	}
}

// openRouterAt points an OpenRouter at the fake.
func openRouterAt(t *testing.T, f *fakeOpenRouter, modelID, key string) *OpenRouter {
	t.Helper()
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)
	o := NewOpenRouter(modelID, func() (string, error) { return key, nil })
	if o == nil {
		t.Fatal("NewOpenRouter returned nil for a model")
	}
	o.base = srv.URL + "/api/v1"
	o.client = srv.Client()
	o.client.CheckRedirect = noRedirects
	return o
}

// captureLog collects what the package logs while fn runs.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	defer func() { log.SetOutput(prev); log.SetFlags(flags) }()
	fn()
	return buf.String()
}

func code(err error) string {
	var p *Problem
	if errors.As(err, &p) {
		return p.Code
	}
	if err == nil {
		return ""
	}
	return "untyped: " + err.Error()
}

func TestNewOpenRouterIsOffWithoutAModel(t *testing.T) {
	if NewOpenRouter("  ", func() (string, error) { return testKey, nil }) != nil {
		t.Error("an OpenRouter without a model summarises nothing, so it is off")
	}
}

func TestOpenRouterTalksToOpenRouterOverHTTPSOnly(t *testing.T) {
	o := NewOpenRouter("z-ai/glm-4.6", func() (string, error) { return testKey, nil })
	if o.base != "https://openrouter.ai/api/v1" {
		t.Errorf("base = %q", o.base)
	}
	o.base = "http://openrouter.ai/api/v1"
	if got := code(o.CheckKey(context.Background())); got != "network" {
		t.Errorf("a plain HTTP base must not be used, got %q", got)
	}
}

func TestTheKeyCheckProvesTheKeyAndFindsTheModel(t *testing.T) {
	f := &fakeOpenRouter{}
	o := openRouterAt(t, f, "z-ai/glm-4.6", testKey)
	if err := o.CheckKey(context.Background()); err != nil {
		t.Fatalf("CheckKey: %v", err)
	}
	if f.auth["/api/v1/key"] != "Bearer "+testKey {
		t.Errorf("the key check sent %q, want the key as a bearer token", f.auth["/api/v1/key"])
	}
	if f.auth["/api/v1/models"] != "" {
		t.Error("the public model list needs no key, so it must not get one")
	}
	if len(f.bodies) != 0 {
		t.Error("the key check must not generate anything")
	}
}

func TestTheKeyCheckNamesWhatIsWrong(t *testing.T) {
	for _, tc := range []struct {
		name  string
		f     *fakeOpenRouter
		model string
		key   string
		want  string
	}{
		{"no key", &fakeOpenRouter{}, "z-ai/glm-4.6", "", "no_key"},
		{"rejected key", &fakeOpenRouter{keyCode: 401}, "z-ai/glm-4.6", testKey, "invalid_key"},
		{"no credit left", &fakeOpenRouter{limit: "0"}, "z-ai/glm-4.6", testKey, "no_credit"},
		{"unknown model", &fakeOpenRouter{}, "nobody/nothing", testKey, "model_unavailable"},
		{"rate limited", &fakeOpenRouter{keyCode: 429}, "z-ai/glm-4.6", testKey, "rate_limited"},
		{"server trouble", &fakeOpenRouter{keyCode: 502}, "z-ai/glm-4.6", testKey, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := openRouterAt(t, tc.f, tc.model, tc.key)
			if got := code(o.CheckKey(context.Background())); got != tc.want {
				t.Errorf("CheckKey = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTheKeyCheckSaysNetworkWhenOpenRouterIsUnreachable(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	o := NewOpenRouter("z-ai/glm-4.6", func() (string, error) { return testKey, nil })
	o.base = srv.URL + "/api/v1"
	o.client = srv.Client()
	srv.Close()
	if got := code(o.CheckKey(context.Background())); got != "network" {
		t.Errorf("CheckKey = %q, want network", got)
	}
}

func TestTheTrialGeneratesATinySummary(t *testing.T) {
	f := &fakeOpenRouter{}
	o := openRouterAt(t, f, "z-ai/glm-4.6", testKey)
	if err := o.Try(context.Background()); err != nil {
		t.Fatalf("Try: %v", err)
	}
	if len(f.bodies) != 1 {
		t.Fatalf("Try sent %d generations, want 1", len(f.bodies))
	}
	var req struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.Unmarshal([]byte(f.bodies[0]), &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "z-ai/glm-4.6" || req.MaxTokens < 2000 || req.MaxTokens > 4096 {
		t.Errorf("the trial asked for %+v, want the model and a cap that leaves a reasoning model room to answer", req)
	}
}

func TestTheTrialNamesWhatIsWrong(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    *fakeOpenRouter
		want string
	}{
		{"rejected key", &fakeOpenRouter{chatCode: 401}, "invalid_key"},
		{"no credit", &fakeOpenRouter{chatCode: 402}, "no_credit"},
		{"model gone", &fakeOpenRouter{chatCode: 404}, "model_unavailable"},
		{"rate limited", &fakeOpenRouter{chatCode: 429}, "rate_limited"},
		{"no provider for the model", &fakeOpenRouter{chatCode: 503}, "model_unavailable"},
		{"not a summary", &fakeOpenRouter{answer: "Sure! Here is a summary."}, "bad_answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := openRouterAt(t, tc.f, "z-ai/glm-4.6", testKey)
			if got := code(o.Try(context.Background())); got != tc.want {
				t.Errorf("Try = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOpenRouterSummarizesAChangelog(t *testing.T) {
	f := &fakeOpenRouter{}
	o := openRouterAt(t, f, "z-ai/glm-4.6", testKey)
	sum, ok := o.Summarize(context.Background(), model.Container{Name: "nextcloud", Repo: "ghcr.io/x/y"}, "1.0.0", "1.1.0", "## 1.1.0\n- dark mode")
	if !ok || len(sum.Bullets) != 1 || sum.Model != "z-ai/glm-4.6" {
		t.Fatalf("Summarize = %+v, %v", sum, ok)
	}
	if f.auth["/api/v1/chat/completions"] != "Bearer "+testKey {
		t.Error("the summary must carry the key as a bearer token")
	}
	if strings.Contains(f.bodies[0], "nextcloud") {
		t.Error("the container's name must stay on the server")
	}
}

func TestOpenRouterSkipsAnEmptyChangelogAndAMissingKey(t *testing.T) {
	f := &fakeOpenRouter{}
	if _, ok := openRouterAt(t, f, "m/x", testKey).Summarize(context.Background(), model.Container{}, "a", "b", "  "); ok {
		t.Error("an empty changelog has nothing to summarise")
	}
	if _, ok := openRouterAt(t, f, "m/x", "").Summarize(context.Background(), model.Container{}, "a", "b", "notes"); ok {
		t.Error("without a key there is no summary")
	}
	if len(f.bodies) != 0 {
		t.Error("nothing may reach OpenRouter in either case")
	}
}

func TestTheKeyStaysOutOfLogsAndErrors(t *testing.T) {
	f := &fakeOpenRouter{keyCode: 401, chatCode: 401, echo: true}
	o := openRouterAt(t, f, "z-ai/glm-4.6", testKey)
	var errs []string
	logged := captureLog(t, func() {
		errs = append(errs, o.CheckKey(context.Background()).Error(), o.Try(context.Background()).Error())
		_, _ = o.Summarize(context.Background(), model.Container{Name: "n", Repo: "r"}, "a", "b", "notes")
		errs = append(errs, fmt.Sprintf("%v %+v %#v", o, o, o))
	})
	for _, s := range append(errs, logged) {
		if strings.Contains(s, testKey) || strings.Contains(s, testKey[10:]) {
			t.Errorf("the key leaked: %s", s)
		}
	}
	if !strings.Contains(logged, "invalid_key") {
		t.Errorf("the log should still say what went wrong: %q", logged)
	}
}

func TestAFailingRequestKeepsTheKeyOutOfItsError(t *testing.T) {
	o := NewOpenRouter("z-ai/glm-4.6", func() (string, error) { return testKey, nil })
	o.base = "https://127.0.0.1:1/" + testKey
	err := o.CheckKey(context.Background())
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Errorf("error = %v", err)
	}
}

func TestOpenRouterFollowsNoRedirect(t *testing.T) {
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("the key followed a redirect to another host")
		}
	}))
	defer other.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	o := NewOpenRouter("z-ai/glm-4.6", func() (string, error) { return testKey, nil })
	o.base = srv.URL + "/api/v1"
	o.client = srv.Client()
	o.client.CheckRedirect = noRedirects
	if got := code(o.CheckKey(context.Background())); got != "failed" {
		t.Errorf("a redirect should fail the check, got %q", got)
	}
}

func TestTheModelListComesWithoutAKey(t *testing.T) {
	f := &fakeOpenRouter{}
	o := openRouterAt(t, f, "z-ai/glm-4.6", testKey)
	list, err := o.Models(context.Background())
	if err != nil || len(list) != 2 || list[1].ID != "z-ai/glm-4.6" || list[1].Name != "GLM 4.6" {
		t.Fatalf("Models = %+v, %v", list, err)
	}
	if f.auth["/api/v1/models"] != "" {
		t.Error("the model list went out with the key")
	}
}

func TestAModelListRoutesToTheFallbacksInOrder(t *testing.T) {
	f := &fakeOpenRouter{served: "openai/gpt-oss-20b:free"}
	o := openRouterAt(t, f, " z-ai/glm-4.6 , openai/gpt-oss-20b:free,,z-ai/glm-4.6", testKey)
	sum, ok := o.Summarize(context.Background(), model.Container{Repo: "ghcr.io/x/y"}, "1.0.0", "1.1.0", "## 1.1.0\n- dark mode")
	if !ok {
		t.Fatal("Summarize failed")
	}
	var req struct {
		Model  string   `json:"model"`
		Models []string `json:"models"`
	}
	if err := json.Unmarshal([]byte(f.bodies[0]), &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "z-ai/glm-4.6" || strings.Join(req.Models, ",") != "z-ai/glm-4.6,openai/gpt-oss-20b:free" {
		t.Errorf("request routes model=%q models=%v", req.Model, req.Models)
	}
	if sum.Model != "openai/gpt-oss-20b:free" {
		t.Errorf("the summary names %q, want the fallback that wrote it", sum.Model)
	}
}

func TestASingleModelSendsNoFallbackList(t *testing.T) {
	f := &fakeOpenRouter{}
	o := openRouterAt(t, f, "z-ai/glm-4.6", testKey)
	if _, ok := o.Summarize(context.Background(), model.Container{}, "1.0.0", "1.1.0", "notes"); !ok {
		t.Fatal("Summarize failed")
	}
	if strings.Contains(f.bodies[0], `"models"`) {
		t.Errorf("one model needs no fallback list: %s", f.bodies[0])
	}
}

func TestTheKeyCheckFindsEveryListedModel(t *testing.T) {
	f := &fakeOpenRouter{}
	if err := openRouterAt(t, f, "z-ai/glm-4.6,openai/gpt-oss-20b:free", testKey).CheckKey(context.Background()); err != nil {
		t.Errorf("CheckKey: %v", err)
	}
	err := openRouterAt(t, f, "z-ai/glm-4.6,gone/model:free", testKey).CheckKey(context.Background())
	if code(err) != "model_unavailable" || !strings.Contains(err.Error(), "gone/model:free") {
		t.Errorf("CheckKey = %v, want the missing fallback named", err)
	}
}

func TestARateLimitNamesTheProvidersReason(t *testing.T) {
	f := &fakeOpenRouter{chatCode: http.StatusTooManyRequests, raw: "google/gemma:free is temporarily rate-limited upstream."}
	o := openRouterAt(t, f, "google/gemma:free", testKey)
	out := captureLog(t, func() { o.Summarize(context.Background(), model.Container{Name: "app"}, "1", "2", "notes") })
	if !strings.Contains(out, "rate_limited") || !strings.Contains(out, "Google AI Studio: google/gemma:free is temporarily rate-limited upstream.") {
		t.Errorf("the log should say why the provider refused: %s", out)
	}
}

func TestAnAnswerCutOffAtItsLimitSaysSo(t *testing.T) {
	f := &fakeOpenRouter{cutOff: true}
	err := openRouterAt(t, f, "z-ai/glm-4.6", testKey).Try(context.Background())
	if code(err) != "bad_answer" || !strings.Contains(err.Error(), "token limit") {
		t.Errorf("Try = %v, want bad_answer naming the token limit", err)
	}
}
