package updates

import (
	"context"
	"encoding/json"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
)

// ManagedAction names the containers ShipLog's auto-update looks after, so a
// backup app does not update them as well.
const ManagedAction = "updates.managed"

// Managed answers updates.managed.
type Managed struct {
	Store  Store
	Policy autoupdate.Policy
	// On is whether the scheduled auto-update applies updates at all: enabled,
	// on a schedule and not a dry run.
	On bool
}

// Action is updates.managed for fortress.Config.Actions.
func (m *Managed) Action() fortress.Action {
	return fortress.Action{Name: ManagedAction, Run: m.serve}
}

// serve lists every container the auto-update would update: not excluded,
// not pinned to a digest, not a local image, and with its waiting update, if
// any, within the level and free of a blocked word.
func (m *Managed) serve(context.Context, fortress.Caller, []byte) ([]byte, error) {
	names := []string{}
	if m.On {
		sts, err := m.Store.List()
		if err != nil {
			return nil, err
		}
		for _, st := range sts {
			c := st.Container
			switch {
			case c.PinnedDigest != "" || c.IsLocal || autoupdate.ContainerExcluded(c.Name, m.Policy.ExcludeContainers):
			case st.HasUpdate() && !autoupdate.WillApply(st, m.Policy):
			default:
				names = append(names, c.Name)
			}
		}
	}
	return json.Marshal(map[string]any{"autoUpdate": m.On, "containers": names})
}
