package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/store"
)

// GuardLister reads what the auto-update keeps about backups and rollbacks.
type GuardLister interface {
	Guards() ([]store.Guard, error)
}

// WithGuards serves /api/guards. backup reports whether the fortress has a
// backup app on this server, which is when the Docker tab offers to update a
// container without a backup.
func (a *API) WithGuards(g GuardLister, backup func() bool) *API {
	a.guards, a.backup = g, backup
	return a
}

type guardJSON struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
	Link     string `json:"link,omitempty"`
	// Pinned is the image a rolled back template runs now.
	Pinned string    `json:"pinned,omitempty"`
	Since  time.Time `json:"since"`
}

func (a *API) listGuards(w http.ResponseWriter, _ *http.Request) {
	out := struct {
		Backup bool        `json:"backup"`
		Guards []guardJSON `json:"guards"`
	}{Guards: []guardJSON{}}
	if a.guards != nil {
		out.Backup = a.backup()
		gs, err := a.guards.Guards()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, g := range gs {
			j := guardJSON{Name: g.Name, State: g.State, Reason: g.Reason, Snapshot: g.Snapshot, Link: g.Link,
				Since: time.Unix(g.At, 0).UTC()}
			if g.State == store.GuardRolledBack && g.Repository != "" && g.Digest != "" {
				ref, _, _ := strings.Cut(g.Repository, "@")
				j.Pinned = ref + "@" + g.Digest
			}
			out.Guards = append(out.Guards, j)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
