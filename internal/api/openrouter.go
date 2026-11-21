package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/summarize"
)

// OpenRouterKeys keeps the key the settings page writes and never reads.
type OpenRouterKeys interface {
	Save(key string) error
	Remove() error
	Hint() (saved bool, last4 string)
}

// OpenRouter wires the settings page's OpenRouter rows: the key, the public
// model list and the two checks, each run for the model the page names.
type OpenRouter struct {
	Keys   OpenRouterKeys
	Models func(context.Context) ([]summarize.Model, error)
	// Check proves the key and finds the model in the list. Try runs one
	// small summary.
	Check func(ctx context.Context, model string) error
	Try   func(ctx context.Context, model string) error
}

// WithOpenRouter serves /api/openrouter and the routes under it.
func (a *API) WithOpenRouter(o OpenRouter) *API {
	a.openRouter = &openRouterRoutes{OpenRouter: o}
	return a
}

type openRouterRoutes struct {
	OpenRouter
	mu     sync.Mutex
	models []summarize.Model
	at     time.Time
}

// modelsFresh is how long the model list is kept. It changes a few times a
// week, and the picker asks for it on every visit to the page.
const modelsFresh = time.Hour

var modelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,199}$`)

func (o *openRouterRoutes) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/openrouter", o.state)
	mux.HandleFunc("PUT /api/openrouter/key", o.saveKey)
	mux.HandleFunc("DELETE /api/openrouter/key", o.removeKey)
	mux.HandleFunc("GET /api/openrouter/models", o.listModels)
	mux.HandleFunc("POST /api/openrouter/check", o.run("key check", o.Check))
	mux.HandleFunc("POST /api/openrouter/try", o.run("trial summary", o.Try))
}

// state says whether a key is saved and its last four characters, and nothing
// more of it.
func (o *openRouterRoutes) state(w http.ResponseWriter, _ *http.Request) {
	saved, hint := o.Keys.Hint()
	writeJSON(w, http.StatusOK, struct {
		Saved bool   `json:"key_saved"`
		Hint  string `json:"key_hint"`
	}{saved, hint})
}

// saveKey keeps the saved key when the field was left empty.
func (o *openRouterRoutes) saveKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if strings.TrimSpace(req.Key) != "" {
		err := o.Keys.Save(req.Key)
		switch {
		case errors.Is(err, summarize.ErrBadKey):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad_key"})
			return
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save_failed"})
			return
		}
	}
	o.state(w, r)
}

func (o *openRouterRoutes) removeKey(w http.ResponseWriter, r *http.Request) {
	if err := o.Keys.Remove(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save_failed"})
		return
	}
	o.state(w, r)
}

func (o *openRouterRoutes) listModels(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.models == nil || time.Since(o.at) > modelsFresh {
		list, err := o.Models(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": summarize.ProblemCode(err)})
			return
		}
		o.models, o.at = list, time.Now()
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": o.models})
}

type checked struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// run answers a check with ok or its code alone. The detail quotes what
// OpenRouter said, so it goes to the log only.
func (o *openRouterRoutes) run(name string, check func(context.Context, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
			return
		}
		if !modelID.MatchString(req.Model) {
			writeJSON(w, http.StatusOK, checked{Error: "no_model"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		if err := check(ctx, req.Model); err != nil {
			log.Printf("shiplog: OpenRouter %s with %s: %v", name, req.Model, err)
			writeJSON(w, http.StatusOK, checked{Error: summarize.ProblemCode(err)})
			return
		}
		writeJSON(w, http.StatusOK, checked{OK: true})
	}
}
