// Package summarize turns a raw changelog into a short AI summary via Ollama.
// It is optional: when OLLAMA_URL/OLLAMA_MODEL are unset New returns nil and the
// engine simply skips summarisation, falling back to the raw changelog.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

const maxRawChars = 6000

// Ollama summarises changelogs against an Ollama server's REST API.
type Ollama struct {
	url    string
	model  string
	client *http.Client
}

// New returns an Ollama summariser, or nil if url or model is empty (feature off).
func New(url, model string) *Ollama {
	url, model = strings.TrimSpace(url), strings.TrimSpace(model)
	if url == "" || model == "" {
		return nil
	}
	return &Ollama{
		url:    strings.TrimRight(url, "/"),
		model:  model,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

// Ping checks the server is reachable and the model is present, so startup can
// log plainly whether AI summaries will work. nil receiver → not configured.
func (o *Ollama) Ping(ctx context.Context) error {
	if o == nil {
		return fmt.Errorf("ollama not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.url+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama /api/tags status %d", resp.StatusCode)
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	for _, m := range body.Models {
		if m.Name == o.model || strings.HasPrefix(m.Name, o.model+":") {
			return nil
		}
	}
	return fmt.Errorf("model %q not found on the Ollama server (pull it first)", o.model)
}

// Summarize asks Ollama to condense raw into bullets/breaking/risk. Returns
// (nil,false) on any error so the engine falls back to the raw changelog.
func (o *Ollama) Summarize(ctx context.Context, c model.Container, fromTag, toTag, raw string) (*model.AISummary, bool) {
	if o == nil || strings.TrimSpace(raw) == "" {
		return nil, false
	}
	if len(raw) > maxRawChars {
		raw = raw[:maxRawChars]
	}
	prompt := "You summarise a Docker image changelog for a homelab admin. " +
		"Image " + c.Repo + " from " + fromTag + " to " + toTag + ". " +
		`Reply ONLY with JSON: {"bullets":[3-5 short strings of what changes],` +
		`"breaking":[strings of breaking changes or required migration steps, empty if none],` +
		`"risk":"one short sentence"}. Be concise and factual. Changelog:` + "\n" + raw

	reqBody, _ := json.Marshal(map[string]any{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
		"format": "json",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		log.Printf("shiplog: ollama %s: request failed: %v", c.Name, err)
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode != http.StatusOK {
		log.Printf("shiplog: ollama %s: HTTP %d: %s", c.Name, resp.StatusCode, snippet(string(body)))
		return nil, false
	}
	var gen struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &gen); err != nil {
		log.Printf("shiplog: ollama %s: cannot decode /api/generate response: %v", c.Name, err)
		return nil, false
	}
	r := strings.TrimSpace(gen.Response)
	if r == "" {
		log.Printf("shiplog: ollama %s: empty model response (the model produced nothing)", c.Name)
		return nil, false
	}
	// With format:"json" the model's answer is itself a JSON document in Response.
	var out struct {
		Bullets  []string `json:"bullets"`
		Breaking []string `json:"breaking"`
		Risk     string   `json:"risk"`
	}
	if err := json.Unmarshal([]byte(r), &out); err != nil {
		log.Printf("shiplog: ollama %s: model output is not the expected JSON: %v — got: %s", c.Name, err, snippet(r))
		return nil, false
	}
	if len(out.Bullets) == 0 && len(out.Breaking) == 0 && out.Risk == "" {
		log.Printf("shiplog: ollama %s: parsed JSON but bullets/breaking/risk are all empty — got: %s", c.Name, snippet(r))
		return nil, false
	}
	return &model.AISummary{
		Bullets:  out.Bullets,
		Breaking: out.Breaking,
		Risk:     out.Risk,
		Model:    o.model,
	}, true
}

// snippet collapses whitespace and caps a string for a single-line log message.
func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 240 {
		s = s[:240] + "…"
	}
	if s == "" {
		s = "(empty)"
	}
	return s
}
