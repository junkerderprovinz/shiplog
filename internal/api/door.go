package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/junkerderprovinz/shiplog/internal/fortress"
)

// Door lets the settings page see ShipLog's place in a fortress and join one
// with the phrase.
type Door interface {
	State() fortress.State
	Enter(words string) error
}

// WithDoor serves /api/fortress and /api/fortress/join from d. Like the status
// contract, both answer only on the engine's unix socket, since the phrase
// opens the fortress to whoever sends it.
func (a *API) WithDoor(d Door) *API {
	a.door = d
	return a
}

func (a *API) fortressState(w http.ResponseWriter, r *http.Request) {
	if fromSocket(r) != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "socket"})
		return
	}
	writeJSON(w, http.StatusOK, a.door.State())
}

// fortressJoin answers with a code the page can word: phrase for words that
// are no fortress phrase, member when ShipLog is in one already.
func (a *API) fortressJoin(w http.ResponseWriter, r *http.Request) {
	if fromSocket(r) != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "socket"})
		return
	}
	var req struct {
		Phrase string `json:"phrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phrase"})
		return
	}
	err := a.door.Enter(req.Phrase)
	switch {
	case errors.Is(err, fortress.ErrPhrase):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phrase"})
	case errors.Is(err, fortress.ErrMember):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "member"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, a.door.State())
	}
}
