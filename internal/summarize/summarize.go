// Package summarize turns a raw changelog into a short AI summary, through
// Ollama or through an OpenAI-compatible server such as llama-swap.
package summarize

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

const maxRawChars = 6000

// buildPrompt is the one prompt both providers send.
func buildPrompt(c model.Container, fromTag, toTag, raw string) string {
	if len(raw) > maxRawChars {
		raw = raw[:maxRawChars]
	}
	return "You summarise a Docker image changelog for a homelab admin. " +
		"Image " + c.Repo + " from " + fromTag + " to " + toTag + ". " +
		`Reply ONLY with JSON: {"bullets":[3-5 short strings of what changes],` +
		`"breaking":[strings of breaking changes or required migration steps, empty if none],` +
		`"risk":"one short sentence"}. Be concise and factual. Changelog:` + "\n" + raw
}

// parseSummary decodes the model's JSON answer. An answer with nothing in it
// is an error, so the caller logs it and shows the raw changelog instead.
func parseSummary(answer, modelName string) (*model.AISummary, error) {
	var out struct {
		Bullets  []string `json:"bullets"`
		Breaking []string `json:"breaking"`
		Risk     string   `json:"risk"`
	}
	if err := json.Unmarshal([]byte(answer), &out); err != nil {
		return nil, fmt.Errorf("model output is not the expected JSON: %w", err)
	}
	if len(out.Bullets) == 0 && len(out.Breaking) == 0 && out.Risk == "" {
		return nil, errors.New("parsed JSON but bullets/breaking/risk are all empty")
	}
	return &model.AISummary{
		Bullets:  out.Bullets,
		Breaking: out.Breaking,
		Risk:     out.Risk,
		Model:    modelName,
	}, nil
}

// snippet fits a string on one log line.
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
