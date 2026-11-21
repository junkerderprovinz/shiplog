package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// LlamaSwap summarises changelogs against llama-swap or any other server that
// speaks the OpenAI API (/v1/models and /v1/chat/completions).
type LlamaSwap struct {
	provider string
	keyFile  string
	url      string
	model    string
	client   *http.Client
}

// NewLlamaSwap returns nil when url or model is empty, which turns it off.
func NewLlamaSwap(url, model string) *LlamaSwap {
	url, model = strings.TrimSpace(url), strings.TrimSpace(model)
	if url == "" || model == "" {
		return nil
	}
	return &LlamaSwap{
		provider: "llama-swap",
		url:      strings.TrimRight(url, "/"),
		model:    model,
		client:   &http.Client{Timeout: 90 * time.Second},
	}
}

// NewOpenRouter uses a fixed HTTPS origin for requests carrying the key.
func NewOpenRouter(model, keyFile string) *LlamaSwap {
	l := NewLlamaSwap("https://openrouter.ai/api", model)
	if l != nil {
		l.provider = "OpenRouter"
		l.keyFile = keyFile
		l.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	return l
}

func (l *LlamaSwap) do(req *http.Request) (*http.Response, error) {
	if l.provider == "OpenRouter" {
		key, err := os.ReadFile(l.keyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read openrouter.key in the ShipLog data directory")
		}
		token := strings.TrimSpace(string(key))
		if !strings.HasPrefix(token, "sk-or-") || strings.ContainsAny(token, " \r\n\t") {
			return nil, fmt.Errorf("openrouter.key must contain one OpenRouter API key")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return l.client.Do(req)
}

// Ping checks that the server answers and lists the model, so startup can log
// whether summaries will work.
func (l *LlamaSwap) Ping(ctx context.Context) error {
	if l == nil {
		return fmt.Errorf("llama-swap not configured")
	}
	if l.provider == "OpenRouter" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.url+"/v1/key", nil)
		if err != nil {
			return err
		}
		resp, err := l.do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("OpenRouter key check returned HTTP %d", resp.StatusCode)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.url+"/v1/models", nil)
	if err != nil {
		return err
	}
	resp, err := l.do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s /v1/models status %d", l.provider, resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	for _, m := range body.Data {
		if m.ID == l.model {
			return nil
		}
	}
	return fmt.Errorf("model %q not found on %s", l.model, l.provider)
}

// Summarize asks the server to condense raw into bullets, breaking changes and
// a risk line. On any error the engine shows the raw changelog instead.
func (l *LlamaSwap) Summarize(ctx context.Context, c model.Container, fromTag, toTag, raw string) (*model.AISummary, bool) {
	if l == nil || strings.TrimSpace(raw) == "" {
		return nil, false
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "user", "content": buildPrompt(c, fromTag, toTag, raw)},
		},
		"response_format": map[string]string{"type": "json_object"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.url+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.do(req)
	if err != nil {
		log.Printf("shiplog: %s %s: request failed: %v", l.provider, c.Name, err)
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode != http.StatusOK {
		if l.keyFile == "" {
			log.Printf("shiplog: llama-swap %s: HTTP %d: %s", c.Name, resp.StatusCode, snippet(string(body)))
		} else {
			log.Printf("shiplog: OpenRouter %s: HTTP %d", c.Name, resp.StatusCode)
		}
		return nil, false
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		log.Printf("shiplog: %s %s: cannot decode /v1/chat/completions response: %v", l.provider, c.Name, err)
		return nil, false
	}
	if len(completion.Choices) == 0 {
		log.Printf("shiplog: %s %s: response has no choices", l.provider, c.Name)
		return nil, false
	}
	answer := strings.TrimSpace(completion.Choices[0].Message.Content)
	if answer == "" {
		log.Printf("shiplog: %s %s: empty model response (the model produced nothing)", l.provider, c.Name)
		return nil, false
	}
	sum, err := parseSummary(answer, l.model)
	if err != nil {
		if l.keyFile == "" {
			log.Printf("shiplog: llama-swap %s: %v; got: %s", c.Name, err, snippet(answer))
		} else {
			log.Printf("shiplog: OpenRouter %s: %v", c.Name, err)
		}
		return nil, false
	}
	return sum, true
}
