package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/summarize"
)

// openRouterReady is a config with an OpenRouter model and a saved key.
func openRouterReady(t *testing.T) config.Config {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "openrouter.key")
	if err := os.WriteFile(keyFile, []byte("sk-or-v1-0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return config.Config{OpenRouterModel: "z-ai/glm-4.6", OpenRouterKeyFile: keyFile}
}

func TestTheSummariesStayLocalByDefault(t *testing.T) {
	cfg := openRouterReady(t)
	cfg.AIProvider = "local"
	cfg.LlamaSwapURL, cfg.LlamaSwapModel = "http://127.0.0.1:9292", "gemma4-e4b"
	if ai := pickSummarizer(cfg); ai.name != "llama-swap" {
		t.Errorf("with Local chosen and a key saved, the summaries went to %q", ai.name)
	}
	cfg.LlamaSwapURL = ""
	cfg.OllamaURL, cfg.OllamaModel = "http://127.0.0.1:11434", "llama3.1"
	if ai := pickSummarizer(cfg); ai.name != "Ollama" {
		t.Errorf("got %q, want Ollama", ai.name)
	}
	cfg.OllamaURL = ""
	if ai := pickSummarizer(cfg); ai.s != nil {
		t.Errorf("without a local model there are no summaries, got %q", ai.name)
	}
}

func TestOpenRouterSummarisesOnceItIsChosen(t *testing.T) {
	cfg := openRouterReady(t)
	cfg.AIProvider = "openrouter"
	cfg.LlamaSwapURL, cfg.LlamaSwapModel = "http://127.0.0.1:9292", "gemma4-e4b"
	ai := pickSummarizer(cfg)
	if _, ok := ai.s.(*summarize.OpenRouter); !ok || ai.model != "z-ai/glm-4.6" {
		t.Errorf("picked %q (%T), want OpenRouter with its model", ai.name, ai.s)
	}
	cfg.OpenRouterModel = ""
	if ai := pickSummarizer(cfg); ai.s != nil {
		t.Errorf("OpenRouter without a model summarises nothing, got %q", ai.name)
	}
}

func TestTheDefaultConfigSummarisesNothing(t *testing.T) {
	for _, k := range []string{"AI_PROVIDER", "OPENROUTER_MODEL", "OPENROUTER_KEY_FILE", "OLLAMA_URL", "OLLAMA_MODEL", "LLAMASWAP_URL", "LLAMASWAP_MODEL"} {
		t.Setenv(k, "")
	}
	if ai := pickSummarizer(config.Load()); ai.s != nil {
		t.Errorf("an empty config picked %q", ai.name)
	}
}
