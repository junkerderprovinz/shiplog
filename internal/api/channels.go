package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"
)

// Channels keeps the notification channels the settings page edits, as
// notify.Notifier does.
type Channels interface {
	Public() []kit.Channel
	Replace(next []kit.Channel) ([]kit.Channel, error)
	Test(ctx context.Context, ch kit.Channel) kit.Result
}

// WithChannels serves /api/notify and /api/notify/test.
func (a *API) WithChannels(c Channels) *API {
	a.channels = c
	return a
}

// maxChannels caps a saved list, well above the channels anyone keeps.
const maxChannels = 256 << 10

func (a *API) listChannels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"channels": a.channels.Public()})
}

func (a *API) replaceChannels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channels []kit.Channel `json:"channels"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChannels)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	pub, err := a.channels.Replace(req.Channels)
	var ps kit.Problems
	switch {
	case errors.As(err, &ps):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid", "problems": ps})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save_failed"})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"channels": pub})
	}
}

func (a *API) testChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channel kit.Channel `json:"channel"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChannels)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if res := a.channels.Test(r.Context(), req.Channel); !res.OK() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": res.Code})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
