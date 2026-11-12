package backup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
)

// KitFortress is the part of the kit's fortress the backup uses.
type KitFortress interface {
	Members() []fortress.Member
	Act(ctx context.Context, id, action string, body []byte) ([]byte, error)
}

// Kit reaches the backup app through the kit's fortress. The fortress carries
// actions between apps and no routes, so both calls of the contract travel as
// the action backup.request under its one permission: the request with its
// body, the question about a run as {"run": id}.
type Kit struct {
	Fortress KitFortress
}

// Members lists the instances; a viewer takes no backups.
func (k Kit) Members() []Member {
	var out []Member
	for _, m := range k.Fortress.Members() {
		if m.Kind != member.KindInstance {
			continue
		}
		out = append(out, Member{ID: m.ID, App: m.App.ID, Device: m.Device, Host: m.Host, Online: m.Online(), Actions: m.Actions})
	}
	return out
}

// Call carries one call of the backup contract to member id.
func (k Kit) Call(ctx context.Context, id, method, path string, body []byte) (int, []byte, error) {
	switch {
	case method == http.MethodPost && path == RequestPath:
	case method == http.MethodGet && strings.HasPrefix(path, RunsPath) && len(path) > len(RunsPath):
		body, _ = json.Marshal(map[string]string{"run": path[len(RunsPath):]})
	default:
		return http.StatusNotFound, []byte(`{"error":"not_found"}`), nil
	}
	out, err := k.Fortress.Act(ctx, id, Action, body)
	var no *fortress.Refusal
	switch {
	case err == nil:
		return http.StatusOK, out, nil
	case errors.As(err, &no):
		return no.Status, refusal(no.Code), nil
	case errors.Is(err, fortress.ErrPermissionOff):
		return http.StatusForbidden, refusal("permission_off"), nil
	case errors.Is(err, fortress.ErrNoSuchAction):
		return http.StatusNotFound, refusal("no_such_action"), nil
	}
	return 0, nil, err
}

func refusal(code string) []byte {
	b, _ := json.Marshal(map[string]string{"error": code})
	return b
}
