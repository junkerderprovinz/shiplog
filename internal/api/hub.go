package api

import (
	"context"
	"net/http"
)

// WithHub serves /api/notify-hub. active reports whether a TollRoll of the
// fortress sends ShipLog's messages now.
func (a *API) WithHub(active func(context.Context) bool) *API {
	a.hub = active
	return a
}

func (a *API) notifyHub(w http.ResponseWriter, r *http.Request) {
	out := struct {
		Hub bool `json:"hub"`
	}{}
	if a.hub != nil {
		out.Hub = a.hub(r.Context())
	}
	writeJSON(w, http.StatusOK, out)
}
