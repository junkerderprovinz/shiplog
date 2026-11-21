package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// openRouterBase is the one place the key is sent to.
const openRouterBase = "https://openrouter.ai/api/v1"

// Problem is what went wrong with OpenRouter, as a code the settings page
// words in its own language. Detail is for the log and never holds the key.
type Problem struct {
	Code   string
	Detail string
}

func (p *Problem) Error() string {
	if p.Detail == "" {
		return p.Code
	}
	return p.Code + ": " + p.Detail
}

// Model is an entry of OpenRouter's public model list.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OpenRouter summarises changelogs through openrouter.ai. It reads the key
// for every request instead of holding it, so a new key works without a
// restart and the struct has no key to print.
type OpenRouter struct {
	base   string
	model  string
	keys   func() (string, error)
	client *http.Client
}

// NewOpenRouter returns nil without a model, which turns it off.
func NewOpenRouter(modelID string, keys func() (string, error)) *OpenRouter {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil
	}
	return &OpenRouter{base: openRouterBase, model: modelID, keys: keys, client: openRouterClient()}
}

// OpenRouterModels fetches OpenRouter's public model list, for the picker.
func OpenRouterModels(ctx context.Context) ([]Model, error) {
	return (&OpenRouter{base: openRouterBase, client: openRouterClient()}).Models(ctx)
}

func openRouterClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second, CheckRedirect: noRedirects}
}

// noRedirects keeps the Authorization header from following a redirect to
// another host.
func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// Models fetches the public model list. It needs no key and sends none.
func (o *OpenRouter) Models(ctx context.Context) ([]Model, error) {
	var body struct {
		Data []Model `json:"data"`
	}
	if err := o.call(ctx, http.MethodGet, "/models", "", nil, &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// CheckKey proves the key with an authenticated call that generates nothing,
// then looks the model up in the public list.
func (o *OpenRouter) CheckKey(ctx context.Context) error {
	key, err := o.key()
	if err != nil {
		return err
	}
	var info struct {
		Data struct {
			LimitRemaining *float64 `json:"limit_remaining"`
		} `json:"data"`
	}
	if err := o.call(ctx, http.MethodGet, "/key", key, nil, &info); err != nil {
		return err
	}
	if r := info.Data.LimitRemaining; r != nil && *r <= 0 {
		return &Problem{Code: "no_credit"}
	}
	list, err := o.Models(ctx)
	if err != nil {
		return err
	}
	for _, m := range list {
		if m.ID == o.model {
			return nil
		}
	}
	return &Problem{Code: "model_unavailable", Detail: "not in the model list: " + o.model}
}

// trialChangelog is what Try summarises: a real request at the cost of a few
// hundred tokens.
const trialChangelog = "## 1.1.0\n- Adds a dark mode.\n- Fixes a crash when the config file is empty."

// Try runs one small summary, which proves that the model answers and that
// its answer reads as a summary.
func (o *OpenRouter) Try(ctx context.Context) error {
	c := model.Container{Repo: "example/app"}
	answer, err := o.complete(ctx, buildPrompt(c, "1.0.0", "1.1.0", trialChangelog), 300)
	if err != nil {
		return err
	}
	if _, err := parseSummary(answer, o.model); err != nil {
		return &Problem{Code: "bad_answer", Detail: err.Error()}
	}
	return nil
}

// Summarize asks the model to condense raw. On any error the engine shows the
// raw changelog instead.
func (o *OpenRouter) Summarize(ctx context.Context, c model.Container, fromTag, toTag, raw string) (*model.AISummary, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	answer, err := o.complete(ctx, buildPrompt(c, fromTag, toTag, raw), 0)
	if err != nil {
		log.Printf("shiplog: OpenRouter %s: %v", c.Name, err)
		return nil, false
	}
	sum, err := parseSummary(answer, o.model)
	if err != nil {
		log.Printf("shiplog: OpenRouter %s: %v; got: %s", c.Name, err, snippet(answer))
		return nil, false
	}
	return sum, true
}

func (o *OpenRouter) key() (string, error) {
	key, err := o.keys()
	if err != nil {
		return "", &Problem{Code: "no_key", Detail: err.Error()}
	}
	if key == "" {
		return "", &Problem{Code: "no_key"}
	}
	return key, nil
}

// complete sends one chat completion and returns the answer's text. A cap of
// 0 leaves the length to the model.
func (o *OpenRouter) complete(ctx context.Context, prompt string, maxTokens int) (string, error) {
	key, err := o.key()
	if err != nil {
		return "", err
	}
	req := map[string]any{
		"model":           o.model,
		"messages":        []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]string{"type": "json_object"},
	}
	if maxTokens > 0 {
		req["max_tokens"] = maxTokens
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *apiError `json:"error"`
	}
	if err := o.call(ctx, http.MethodPost, "/chat/completions", key, req, &out); err != nil {
		return "", err
	}
	// A provider that fails after OpenRouter accepted the request answers 200
	// with the error in the body.
	if out.Error != nil {
		return "", problemFor(out.Error.Code, out.Error.Message, key)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", &Problem{Code: "bad_answer", Detail: "the model produced nothing"}
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call sends one request and decodes a 200 into out. Everything it reports
// passes through redact, since a server may echo what it was sent.
func (o *OpenRouter) call(ctx context.Context, method, path, key string, in, out any) error {
	if !strings.HasPrefix(o.base, "https://") {
		return &Problem{Code: "network", Detail: "refusing a base that is not HTTPS"}
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return &Problem{Code: "failed", Detail: err.Error()}
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.base+path, body)
	if err != nil {
		return &Problem{Code: "failed", Detail: redact(err.Error(), key)}
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return &Problem{Code: "network", Detail: redact(err.Error(), key)}
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return &Problem{Code: "network", Detail: redact(err.Error(), key)}
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error apiError `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = string(raw)
		}
		return problemFor(resp.StatusCode, msg, key)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &Problem{Code: "failed", Detail: "cannot decode the answer: " + redact(err.Error(), key)}
	}
	return nil
}

// problemFor turns an HTTP status of OpenRouter's into a code. 503 means no
// provider can serve the model right now. The key goes before the message is
// cut short, so no piece of it is left.
func problemFor(status int, msg, key string) *Problem {
	detail := fmt.Sprintf("HTTP %d: %s", status, snippet(redact(msg, key)))
	switch status {
	case http.StatusUnauthorized:
		return &Problem{Code: "invalid_key", Detail: detail}
	case http.StatusPaymentRequired:
		return &Problem{Code: "no_credit", Detail: detail}
	case http.StatusNotFound, http.StatusServiceUnavailable:
		return &Problem{Code: "model_unavailable", Detail: detail}
	case http.StatusTooManyRequests:
		return &Problem{Code: "rate_limited", Detail: detail}
	}
	return &Problem{Code: "failed", Detail: detail}
}

func redact(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[key]")
}

// ProblemCode reports err's code, or "failed" for an error of another kind.
func ProblemCode(err error) string {
	var p *Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return "failed"
}
