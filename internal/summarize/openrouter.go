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
	base  string
	model string
	// fallbacks are the models OpenRouter routes to when model fails, a
	// rate-limited free model among them.
	fallbacks []string
	// freeRouter appends openrouter/free to a list of free models only, so a
	// lone free model has somewhere to go that still costs nothing.
	freeRouter bool
	keys       func() (string, error)
	client     *http.Client
}

// NewOpenRouter returns nil without a model, which turns it off. modelIDs is
// one model ID or a comma-separated list: the first is asked first, and
// OpenRouter moves on to the next when one fails or is rate-limited.
func NewOpenRouter(modelIDs string, keys func() (string, error)) *OpenRouter {
	ids := ParseModels(modelIDs)
	if len(ids) == 0 {
		return nil
	}
	return &OpenRouter{base: openRouterBase, model: ids[0], fallbacks: ids[1:], freeRouter: allFree(ids), keys: keys, client: openRouterClient()}
}

// freeRouterID is OpenRouter's router across its free models.
const freeRouterID = "openrouter/free"

// allFree reports whether every listed model is free and the list does not
// already end in OpenRouter's free router.
func allFree(ids []string) bool {
	for _, id := range ids {
		if id == freeRouterID || !strings.HasSuffix(id, ":free") {
			return false
		}
	}
	return true
}

// ParseModels splits a comma-separated model list, dropping blanks and
// repeats.
func ParseModels(s string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, id := range strings.Split(s, ",") {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// listed is the models as the settings name them, the first one first.
func (o *OpenRouter) listed() []string {
	return append([]string{o.model}, o.fallbacks...)
}

// models is every model a request may route to, the first one first.
func (o *OpenRouter) models() []string {
	if o.freeRouter {
		return append(o.listed(), freeRouterID)
	}
	return o.listed()
}

// Writes reports whether a summary credited to id could come from the models
// set now, so the engine drops a summary kept from another model or provider.
// A router's answer names the model it picked, not the router.
func (o *OpenRouter) Writes(id string) bool {
	for _, m := range o.models() {
		switch {
		case m == id, m == "openrouter/auto":
			return true
		case m == freeRouterID && strings.HasSuffix(id, ":free"):
			return true
		}
	}
	return false
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
	listed := map[string]bool{}
	for _, m := range list {
		listed[m.ID] = true
	}
	for _, id := range o.listed() {
		if !listed[id] {
			return &Problem{Code: "model_unavailable", Detail: "not in the model list: " + id}
		}
	}
	return nil
}

// trialChangelog is what Try summarises: a real request at the cost of a few
// hundred tokens.
const trialChangelog = "## 1.1.0\n- Adds a dark mode.\n- Fixes a crash when the config file is empty."

// maxTokens bounds what one request may cost, Try and summaries alike. A
// reasoning model, which most free ones are, counts its thinking against the
// cap, about a thousand tokens for the trial changelog, and a smaller cap cuts
// it off before it answers. Low reasoning effort keeps the thinking short.
const maxTokens = 4096

// Try runs one small summary, which proves that the model answers and that
// its answer reads as a summary.
func (o *OpenRouter) Try(ctx context.Context) error {
	c := model.Container{Repo: "example/app"}
	answer, served, err := o.complete(ctx, buildPrompt(c, "1.0.0", "1.1.0", trialChangelog))
	if err != nil {
		return err
	}
	if _, err := parseSummary(answer, served); err != nil {
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
	answer, served, err := o.complete(ctx, buildPrompt(c, fromTag, toTag, raw))
	if err != nil {
		log.Printf("shiplog: OpenRouter %s: %v", c.Name, err)
		return nil, false
	}
	sum, err := parseSummary(answer, served)
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

// complete sends one chat completion and returns the answer's text and the
// model that wrote it, which is a fallback when the first model failed.
func (o *OpenRouter) complete(ctx context.Context, prompt string) (string, string, error) {
	key, err := o.key()
	if err != nil {
		return "", "", err
	}
	req := map[string]any{
		"model":           o.model,
		"messages":        []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]string{"type": "json_object"},
		"max_tokens":      maxTokens,
		// A model that does not reason ignores this.
		"reasoning": map[string]string{"effort": "low"},
	}
	if m := o.models(); len(m) > 1 {
		// OpenRouter tries these in order after model, on a rate limit too.
		req["models"] = m
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *apiError `json:"error"`
	}
	if err := o.call(ctx, http.MethodPost, "/chat/completions", key, req, &out); err != nil {
		return "", "", err
	}
	// A provider that fails after OpenRouter accepted the request answers 200
	// with the error in the body.
	if out.Error != nil {
		return "", "", problemFor(out.Error.Code, out.Error.message(), key)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		detail := "the model produced nothing"
		if len(out.Choices) > 0 && out.Choices[0].FinishReason == "length" {
			detail = "the model reached its token limit before it answered"
		}
		return "", "", &Problem{Code: "bad_answer", Detail: detail}
	}
	served := out.Model
	if served == "" {
		served = o.model
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), served, nil
}

type apiError struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Metadata struct {
		// Raw is the provider's own words. For a free model it tells a
		// provider's exhausted shared pool apart from the key's daily cap,
		// which the message alone ("Provider returned error") does not.
		Raw          json.RawMessage `json:"raw"`
		ProviderName string          `json:"provider_name"`
	} `json:"metadata"`
}

// message is the error's message with the provider's reason after it.
func (e *apiError) message() string {
	msg := e.Message
	raw := strings.TrimSpace(string(e.Metadata.Raw))
	var text string
	if json.Unmarshal(e.Metadata.Raw, &text) == nil {
		raw = strings.TrimSpace(text)
	}
	if raw == "" || raw == "null" || raw == msg {
		return msg
	}
	if p := e.Metadata.ProviderName; p != "" {
		raw = p + ": " + raw
	}
	if msg == "" {
		return raw
	}
	return msg + " (" + raw + ")"
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
		msg := e.Error.message()
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
