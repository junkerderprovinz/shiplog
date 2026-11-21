package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/summarize"
)

const orKey = "sk-or-v1-feedfacefeedfacefeedfacefeedface"

// fakeOpenRouter records the models it was asked about and answers with the
// problem codes it was given.
type fakeOpenRouter struct {
	checked, tried []string
	checkCode      string
	tryCode        string
	listed         int
}

func (f *fakeOpenRouter) wiring(keys summarize.KeyFile) OpenRouter {
	fail := func(code string) error {
		if code == "" {
			return nil
		}
		return &summarize.Problem{Code: code, Detail: "upstream refused"}
	}
	return OpenRouter{
		Keys: keys,
		Models: func(context.Context) ([]summarize.Model, error) {
			f.listed++
			return []summarize.Model{{ID: "z-ai/glm-4.6", Name: "GLM 4.6"}}, nil
		},
		Check: func(_ context.Context, m string) error { f.checked = append(f.checked, m); return fail(f.checkCode) },
		Try:   func(_ context.Context, m string) error { f.tried = append(f.tried, m); return fail(f.tryCode) },
	}
}

func openRouterAPI(t *testing.T, f *fakeOpenRouter) (*summarize.KeyFile, func(method, target, body string) (int, string)) {
	t.Helper()
	keys := &summarize.KeyFile{Path: filepath.Join(t.TempDir(), "openrouter.key")}
	h := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithOpenRouter(f.wiring(*keys)).Handler()
	return keys, func(method, target, body string) (int, string) { return call(h, method, target, body) }
}

func TestTheSettingsNeverReturnTheKey(t *testing.T) {
	_, do := openRouterAPI(t, &fakeOpenRouter{})
	if code, body := do("GET", "/api/openrouter", ""); code != 200 || body != `{"key_saved":false,"key_hint":""}`+"\n" {
		t.Errorf("GET before a key = %d %s", code, body)
	}
	code, body := do("PUT", "/api/openrouter/key", `{"key":"`+orKey+`"}`)
	if code != 200 || body != `{"key_saved":true,"key_hint":"face"}`+"\n" {
		t.Errorf("PUT = %d %s", code, body)
	}
	for _, path := range []string{"/api/openrouter", "/api/openrouter/models", "/api/containers", "/"} {
		if _, body := do("GET", path, ""); strings.Contains(body, orKey) || strings.Contains(body, orKey[:20]) {
			t.Errorf("GET %s returned the key: %s", path, body)
		}
	}
}

func TestAnEmptyKeyFieldKeepsTheSavedKey(t *testing.T) {
	keys, do := openRouterAPI(t, &fakeOpenRouter{})
	do("PUT", "/api/openrouter/key", `{"key":"`+orKey+`"}`)
	if code, body := do("PUT", "/api/openrouter/key", `{"key":"   "}`); code != 200 || !strings.Contains(body, `"key_saved":true`) {
		t.Errorf("empty PUT = %d %s", code, body)
	}
	if got, _ := keys.Load(); got != orKey {
		t.Errorf("the saved key changed to %q", got)
	}
}

func TestAKeyIsRemovedOnlyOnRequest(t *testing.T) {
	keys, do := openRouterAPI(t, &fakeOpenRouter{})
	do("PUT", "/api/openrouter/key", `{"key":"`+orKey+`"}`)
	if code, body := do("DELETE", "/api/openrouter/key", ""); code != 200 || !strings.Contains(body, `"key_saved":false`) {
		t.Errorf("DELETE = %d %s", code, body)
	}
	if got, _ := keys.Load(); got != "" {
		t.Error("the key is still saved")
	}
}

func TestAValueThatCannotBeAKeyIsRefused(t *testing.T) {
	_, do := openRouterAPI(t, &fakeOpenRouter{})
	code, body := do("PUT", "/api/openrouter/key", `{"key":"two words"}`)
	if code != 400 || body != `{"error":"bad_key"}`+"\n" {
		t.Errorf("PUT = %d %s", code, body)
	}
}

func TestTheTwoChecksAnswerApart(t *testing.T) {
	f := &fakeOpenRouter{tryCode: "no_credit"}
	_, do := openRouterAPI(t, f)
	if code, body := do("POST", "/api/openrouter/check", `{"model":"z-ai/glm-4.6"}`); code != 200 || body != `{"ok":true}`+"\n" {
		t.Errorf("check = %d %s", code, body)
	}
	if code, body := do("POST", "/api/openrouter/try", `{"model":"z-ai/glm-4.6"}`); code != 200 || body != `{"ok":false,"error":"no_credit"}`+"\n" {
		t.Errorf("try = %d %s", code, body)
	}
	if len(f.checked) != 1 || len(f.tried) != 1 {
		t.Errorf("checked %v, tried %v: each check runs once and alone", f.checked, f.tried)
	}
}

func TestACheckGivesACodeAndNoDetail(t *testing.T) {
	f := &fakeOpenRouter{checkCode: "invalid_key"}
	_, do := openRouterAPI(t, f)
	_, body := do("POST", "/api/openrouter/check", `{"model":"z-ai/glm-4.6"}`)
	if body != `{"ok":false,"error":"invalid_key"}`+"\n" {
		t.Errorf("check = %s", body)
	}
}

func TestACheckNeedsAModelThatCanBeOne(t *testing.T) {
	f := &fakeOpenRouter{}
	_, do := openRouterAPI(t, f)
	for _, m := range []string{"", "  ", "a b", "x\ny", strings.Repeat("m", 300)} {
		_, body := do("POST", "/api/openrouter/check", `{"model":`+quote(m)+`}`)
		if body != `{"ok":false,"error":"no_model"}`+"\n" {
			t.Errorf("model %q: %s", m, body)
		}
	}
	if len(f.checked) != 0 {
		t.Error("an unusable model must not reach OpenRouter")
	}
}

func TestTheModelListIsFetchedOnceForAWhile(t *testing.T) {
	f := &fakeOpenRouter{}
	_, do := openRouterAPI(t, f)
	for range 3 {
		if code, body := do("GET", "/api/openrouter/models", ""); code != 200 || body != `{"models":[{"id":"z-ai/glm-4.6","name":"GLM 4.6"}]}`+"\n" {
			t.Fatalf("models = %d %s", code, body)
		}
	}
	if f.listed != 1 {
		t.Errorf("the list was fetched %d times", f.listed)
	}
}

func TestTheOpenRouterRoutesNeedTheWiring(t *testing.T) {
	h := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").Handler()
	if code, _ := call(h, "PUT", "/api/openrouter/key", `{"key":"`+orKey+`"}`); code == 200 {
		t.Error("without OpenRouter wired, the key route must not exist")
	}
}

func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}
