package notify

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
	kit "github.com/junkerderprovinz/parleyport/kit/notify"
)

// DeliverAction is the action through which TollRoll takes the
// notifications of the other apps of the fortress.
const DeliverAction = "notify.deliver"

const hubApp = "tollroll"

// HubFortress is the part of the kit's fortress the hub uses.
type HubFortress interface {
	Members() []fortress.Member
	Act(ctx context.Context, id, action string, body []byte) ([]byte, error)
}

// Hub hands ShipLog's messages to a TollRoll of the fortress while it takes
// them over.
type Hub struct {
	Fortress HubFortress
	// Timeout caps each call; zero means ten seconds.
	Timeout time.Duration
}

type delivery struct {
	Kind     string       `json:"kind"`
	Severity kit.Severity `json:"severity"`
	Title    string       `json:"title"`
	Body     string       `json:"body"`
	Link     string       `json:"link,omitempty"`
	Key      string       `json:"key,omitempty"`
}

// Take offers e to every TollRoll in reach, one after the other, and reports
// whether one took it. Any other answer leaves the message to ShipLog.
func (h *Hub) Take(ctx context.Context, e kit.Event) bool {
	body, err := json.Marshal(delivery{Kind: e.Kind, Severity: e.Severity, Title: e.Title, Body: e.Body, Link: e.Link, Key: e.Key})
	if err != nil {
		return false
	}
	for _, m := range h.Fortress.Members() {
		if m.Kind != member.KindInstance || m.App.ID != hubApp || !m.Online() || !slices.Contains(m.Actions, DeliverAction) {
			continue
		}
		if h.offer(ctx, m.ID, body) {
			return true
		}
	}
	return false
}

func (h *Hub) offer(ctx context.Context, id string, body []byte) bool {
	ctx, cancel := context.WithTimeout(ctx, h.timeout())
	defer cancel()
	out, err := h.Fortress.Act(ctx, id, DeliverAction, body)
	if err != nil {
		return false
	}
	var answer struct {
		Taken bool `json:"taken"`
	}
	return json.Unmarshal(out, &answer) == nil && answer.Taken
}

func (h *Hub) timeout() time.Duration {
	if h.Timeout > 0 {
		return h.Timeout
	}
	return 10 * time.Second
}

// WithHub lets h carry n's messages while a TollRoll takes them. Without own
// channels the hub alone carries them.
func WithHub(n *Notifier, h *Hub) *Notifier {
	if h == nil {
		return n
	}
	out := &Notifier{hub: h}
	if n != nil {
		out.d = n.d
	}
	return out
}
