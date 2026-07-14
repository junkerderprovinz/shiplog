// Package api serves the engine's REST endpoints and the embedded status page.
package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/sources"
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

// OverrideStore reads and writes the user's changelog-source overrides.
type OverrideStore interface {
	SourceOverrides() (map[string]string, error)
	SetSourceOverride(repo, source string) error
	DeleteSourceOverride(repo string) error
}

// API bundles the HTTP handlers.
type API struct {
	src StatusSource
	ovr OverrideStore
	ref Refresher
	// verify reports whether a normalised github source repo exists (exists) and
	// whether the check was conclusive (checked). Injectable so tests don't hit
	// the network; the default calls the GitHub API.
	verify func(ctx context.Context, source string) (exists, checked bool)
}

// New builds the API over a status source, an override store and a refresher.
// ghToken (optional) is used to verify a manual source repo against the GitHub
// API when the user saves an override.
func New(src StatusSource, ovr OverrideStore, ref Refresher, ghToken string) *API {
	a := &API{src: src, ovr: ovr, ref: ref}
	a.verify = func(ctx context.Context, source string) (bool, bool) {
		return verifyGitHubRepo(ctx, ghToken, source)
	}
	return a
}

// verifyGitHubRepo GETs api.github.com/repos/owner/repo to check a source exists.
// Returns (exists, checked): checked is false on a transport error or a
// rate-limit/other non-200/404 status, so an inconclusive check never blocks a
// save. source must be the normalised "https://github.com/owner/repo" form.
func verifyGitHubRepo(ctx context.Context, token, source string) (exists, checked bool) {
	path := strings.TrimPrefix(source, "https://github.com/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return false, false
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+parts[0]+"/"+parts[1], nil)
	if err != nil {
		return false, false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, false
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, true
	case http.StatusNotFound:
		return false, true
	default:
		return false, false // rate limit / transient → don't block the save
	}
}

// Handler returns the routed http.Handler.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers", a.listJSON)
	mux.HandleFunc("GET /api/container/{id}", a.oneJSON)
	mux.HandleFunc("POST /api/refresh", a.refresh)
	mux.HandleFunc("GET /api/overrides", a.listOverrides)
	mux.HandleFunc("PUT /api/override", a.setOverride)
	mux.HandleFunc("DELETE /api/override", a.deleteOverride)
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

// listOverrides returns the repo→source override map.
func (a *API) listOverrides(w http.ResponseWriter, _ *http.Request) {
	m, err := a.ovr.SourceOverrides()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		m = map[string]string{}
	}
	writeJSON(w, http.StatusOK, m)
}

type overrideReq struct {
	Repo   string `json:"repo"`
	Source string `json:"source"`
}

// setOverride records a manual changelog source for an image repo. The source
// is normalised to a canonical github.com/owner/repo URL; a non-GitHub or
// malformed value is rejected. A successful change triggers a refresh so the
// corrected changelog appears without waiting for the poll interval.
func (a *API) setOverride(w http.ResponseWriter, r *http.Request) {
	var req overrideReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}
	source, ok := sources.NormalizeGitHubSource(req.Source)
	if !ok {
		http.Error(w, "source must be a GitHub repo, e.g. owner/repo or https://github.com/owner/repo", http.StatusBadRequest)
		return
	}
	// Verify the repo exists (best-effort): reject a clear 404 so a typo is caught
	// at save time, but never block on an inconclusive check (rate limit, offline).
	if a.verify != nil {
		if exists, checked := a.verify(r.Context(), source); checked && !exists {
			http.Error(w, "that GitHub repo was not found — check the owner/repo", http.StatusBadRequest)
			return
		}
	}
	if err := a.ovr.SetSourceOverride(repo, source); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.ref.Refresh()
	writeJSON(w, http.StatusOK, map[string]string{"repo": repo, "source": source})
}

// deleteOverride removes the override for a repo (given as ?repo= or a JSON
// body) and triggers a refresh so the default source resolves again.
func (a *API) deleteOverride(w http.ResponseWriter, r *http.Request) {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo == "" {
		var req overrideReq
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req)
		repo = strings.TrimSpace(req.Repo)
	}
	if repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}
	if err := a.ovr.DeleteSourceOverride(repo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.ref.Refresh()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"override cleared"}`))
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
