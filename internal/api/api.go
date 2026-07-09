// Package api serves the engine's REST endpoints and the embedded status page.
package api

import (
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/store"
)

//go:embed status.html
var statusHTML string

// logoSVG is the ShipLog mark — a gold anchor in a white ring (the shiplog-hell
// variant), so it reads on the dark Carbon status page. Embedded so the standalone
// binary serves it self-contained.
//
//go:embed logo.svg
var logoSVG []byte

var statusTmpl = template.Must(template.New("status").Parse(statusHTML))

// StatusSource is the read side of the store the API exposes.
type StatusSource interface {
	List() ([]model.UpdateStatus, error)
	Get(id string) (model.UpdateStatus, error)
}

// Refresher triggers an out-of-band sweep (the engine).
type Refresher interface {
	Refresh()
}

// API bundles the HTTP handlers.
type API struct {
	src StatusSource
	ref Refresher
}

// New builds the API over a status source and a refresher.
func New(src StatusSource, ref Refresher) *API { return &API{src: src, ref: ref} }

// Handler returns the routed http.Handler.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers", a.listJSON)
	mux.HandleFunc("GET /api/container/{id}", a.oneJSON)
	mux.HandleFunc("POST /api/refresh", a.refresh)
	mux.HandleFunc("GET /logo.svg", a.logo)
	mux.HandleFunc("GET /", a.statusPage)
	return mux
}

// logo serves the embedded ShipLog mark for the status page header.
func (a *API) logo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(logoSVG)
}

func (a *API) listJSON(w http.ResponseWriter, _ *http.Request) {
	rows, err := a.src.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *API) oneJSON(w http.ResponseWriter, r *http.Request) {
	st, err := a.src.Get(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) refresh(w http.ResponseWriter, _ *http.Request) {
	a.ref.Refresh()
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"sweep triggered"}`))
}

type pageData struct {
	Statuses []model.UpdateStatus
	Count    int
	Updates  int
	Mono     bool
}

func (a *API) statusPage(w http.ResponseWriter, r *http.Request) {
	// "/" only — the catch-all pattern also matches unknown paths.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rows, err := a.src.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updates := 0
	for _, s := range rows {
		if s.HasUpdate() {
			updates++
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = statusTmpl.Execute(w, pageData{
		Statuses: rows,
		Count:    len(rows),
		Updates:  updates,
		Mono:     r.URL.Query().Get("mono") == "1",
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
