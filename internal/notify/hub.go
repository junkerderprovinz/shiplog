package notify

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/status"
)

// DeliverAction is the action through which TollRoll takes the
// notifications of the other apps of the fortress, and HubCapability what its
// status report lists while it takes them.
const (
	DeliverAction = "notify.deliver"
	HubCapability = "notify.hub"
)

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
	// Report reads a member's status report, which says whether TollRoll
	// takes the messages now.
	Report func(ctx context.Context, id string) (status.Report, error)
	// Timeout caps each call, zero means ten seconds.
	Timeout time.Duration
}

type delivery struct {
	Kind     string       `json:"kind"`
	Severity kit.Severity `json:"severity"`
	Title    string       `json:"title"`
	Body     string       `json:"body"`
	Link     string       `json:"link,omitempty"`
	Key      string       `json:"key,omitempty"`
	Resolved bool         `json:"resolved,omitempty"`
}

// Take offers e to every TollRoll in reach, one after the other, and reports
// whether one took it. Any other answer leaves the message to ShipLog.
func (h *Hub) Take(ctx context.Context, e kit.Event) bool {
	return h.hand(ctx, delivery{Kind: e.Kind, Severity: e.Severity, Title: e.Title, Body: e.Body, Link: e.Link, Key: e.Key})
}

// Resolve tells TollRoll that what key was about is over.
func (h *Hub) Resolve(ctx context.Context, key string) bool {
	return h.hand(ctx, delivery{Key: key, Resolved: true})
}

// Active reports whether a TollRoll in reach takes the messages now.
func (h *Hub) Active(ctx context.Context) bool {
	if h.Report == nil {
		return false
	}
	for _, id := range h.tollrolls() {
		rep, err := h.Report(ctx, id)
		if err == nil && slices.Contains(rep.Capabilities, HubCapability) {
			return true
		}
	}
	return false
}

func (h *Hub) hand(ctx context.Context, d delivery) bool {
	body, err := json.Marshal(d)
	if err != nil {
		return false
	}
	for _, id := range h.tollrolls() {
		if h.offer(ctx, id, body) {
			return true
		}
	}
	return false
}

func (h *Hub) tollrolls() []string {
	var ids []string
	for _, m := range h.Fortress.Members() {
		if m.Kind == member.KindInstance && m.App.ID == hubApp && m.Online() && slices.Contains(m.Actions, DeliverAction) {
			ids = append(ids, m.ID)
		}
	}
	return ids
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
	switch {
	case h == nil:
		return n
	case n == nil:
		return &Notifier{hub: h}
	}
	n.hub = h
	return n
}
