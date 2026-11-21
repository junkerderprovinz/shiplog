package main

import (
	"context"

	"github.com/junkerderprovinz/shiplog/internal/api"
	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/engine"
	"github.com/junkerderprovinz/shiplog/internal/summarize"
)

// aiPick is the summariser the settings ask for, with what the startup log
// says about it.
type aiPick struct {
	s           engine.Summarizer
	ping        func(context.Context) error
	name, where string
	model       string
}

// pickSummarizer takes OpenRouter only when it is chosen. Otherwise a local
// server summarises, llama-swap before Ollama, and nothing leaves the network.
func pickSummarizer(cfg config.Config) aiPick {
	if cfg.AIProvider == "openrouter" {
		keys := summarize.KeyFile{Path: cfg.OpenRouterKeyFile}
		if o := summarize.NewOpenRouter(cfg.OpenRouterModel, keys.Load); o != nil {
			return aiPick{s: o, ping: o.CheckKey, name: "OpenRouter", where: "openrouter.ai", model: cfg.OpenRouterModel}
		}
		return aiPick{name: "OpenRouter"}
	}
	if ls := summarize.NewLlamaSwap(cfg.LlamaSwapURL, cfg.LlamaSwapModel); ls != nil {
		return aiPick{s: ls, ping: ls.Ping, name: "llama-swap", where: cfg.LlamaSwapURL, model: cfg.LlamaSwapModel}
	}
	if o := summarize.New(cfg.OllamaURL, cfg.OllamaModel); o != nil {
		return aiPick{s: o, ping: o.Ping, name: "Ollama", where: cfg.OllamaURL, model: cfg.OllamaModel}
	}
	return aiPick{}
}

// openRouterSettings serves the settings page's OpenRouter rows. Each check
// runs for the model the page names, which may not be saved yet.
func openRouterSettings(keys summarize.KeyFile) api.OpenRouter {
	open := func(m string) *summarize.OpenRouter { return summarize.NewOpenRouter(m, keys.Load) }
	return api.OpenRouter{
		Keys:   keys,
		Models: summarize.OpenRouterModels,
		Check:  func(ctx context.Context, m string) error { return open(m).CheckKey(ctx) },
		Try:    func(ctx context.Context, m string) error { return open(m).Try(ctx) },
	}
}
