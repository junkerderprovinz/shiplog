package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/junkerderprovinz/shiplog/internal/fortress"
)

// Door lets the settings page see ShipLog's place in a fortress, join one
// with the phrase or leave it, take another member out and ask another
// member how it is doing.
type Door interface {
	State() fortress.State
	Enter(words string) error
	Leave() error
	Remove(id string) error
	Status(ctx context.Context, id, lang string) (fortress.MemberStatus, error)
}

// WithDoor serves /api/fortress and the paths below it from d. Like the
// status contract, they answer only on the engine's unix socket, since the
// phrase opens the fortress to whoever sends it and a removal shuts a member
// out for good.
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

// fortressLeave answers notMember while ShipLog is in no fortress.
func (a *API) fortressLeave(w http.ResponseWriter, r *http.Request) {
	if fromSocket(r) != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "socket"})
		return
	}
	err := a.door.Leave()
	switch {
	case errors.Is(err, fortress.ErrNotMember):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "notMember"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, a.door.State())
	}
}

// fortressMemberStatus answers with the codes of a removal, and unreachable
// when the member does not answer.
func (a *API) fortressMemberStatus(w http.ResponseWriter, r *http.Request) {
	if fromSocket(r) != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "socket"})
		return
	}
	st, err := a.door.Status(r.Context(), r.PathValue("id"), r.URL.Query().Get("lang"))
	switch {
	case errors.Is(err, fortress.ErrNoMember):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "noMember"})
	case errors.Is(err, fortress.ErrNotMember):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "noFortress"})
	case err != nil:
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "unreachable"})
	default:
		writeJSON(w, http.StatusOK, st)
	}
}

// fortressRemove answers with a code the page can word: noMember for an id
// no other member has, noFortress while ShipLog is in none.
func (a *API) fortressRemove(w http.ResponseWriter, r *http.Request) {
	if fromSocket(r) != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "socket"})
		return
	}
	err := a.door.Remove(r.PathValue("id"))
	switch {
	case errors.Is(err, fortress.ErrNoMember):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "noMember"})
	case errors.Is(err, fortress.ErrNotMember):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "noFortress"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, a.door.State())
	}
}
