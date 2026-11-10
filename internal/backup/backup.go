// Package backup asks the app of the fortress that takes backups to back up a
// container before ShipLog updates it, and waits until the backup is done.
package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Action is the permission a backup app grants to back up one container.
const Action = "backup.request"

// The backup contract: a POST to RequestPath starts a run and answers its id,
// RunsPath plus the id reports the run.
const (
	RequestPath = "/api/fortress/v1/backup"
	RunsPath    = "/api/fortress/v1/runs/"
)

// Member is an instance of the fortress.
type Member struct {
	ID     string
	App    string
	Device string
	Online bool
	// Actions are what its app does for the other members.
	Actions []string
}

// Fortress reaches the other members.
type Fortress interface {
	Members() []Member
	Call(ctx context.Context, member, method, path string, body []byte) (int, []byte, error)
}

// Snapshot is the backup taken before an update.
type Snapshot struct {
	Member string // who took it
	Run    string
	ID     string
	Link   string // where the backup app shows it
}

var (
	// ErrNoMember means no app on this server takes backups, so updates go on
	// as they did before there was a fortress.
	ErrNoMember = errors.New("no app in the fortress backs up this server")
	ErrTimeout  = errors.New("the backup did not finish in time")
)

// Client backs up containers through the fortress.
type Client struct {
	Fortress Fortress
	// Device is the server ShipLog runs on. Only a backup app on the same
	// server can back up its containers.
	Device  string
	Timeout time.Duration
	Poll    time.Duration
	sleep   func(context.Context, time.Duration) error
}

// Save backs up container and returns the snapshot once the run is done.
func (c *Client) Save(ctx context.Context, container string) (Snapshot, error) {
	m, err := c.member()
	if err != nil {
		return Snapshot{}, err
	}
	body, _ := json.Marshal(map[string]string{"container": container, "reason": "update"})
	var started struct {
		Run runID `json:"run"`
		ID  runID `json:"id"`
	}
	if err := c.call(ctx, m, http.MethodPost, RequestPath, body, &started); err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{Member: m, Run: string(started.Run)}
	if snap.Run == "" {
		snap.Run = string(started.ID)
	}
	if snap.Run == "" {
		return Snapshot{}, errors.New("the backup app named no run")
	}
	sleep := c.sleep
	if sleep == nil {
		sleep = wait
	}
	for waited := time.Duration(0); ; waited += c.Poll {
		var run struct {
			State    string `json:"state"`
			Snapshot string `json:"snapshot"`
			Link     string `json:"link"`
			Error    string `json:"error"`
		}
		if err := c.call(ctx, m, http.MethodGet, RunsPath+url.PathEscape(snap.Run), nil, &run); err != nil {
			return Snapshot{}, err
		}
		switch strings.ToLower(run.State) {
		case "done", "success":
			snap.ID, snap.Link = run.Snapshot, run.Link
			return snap, nil
		case "failed", "error", "cancelled", "canceled":
			if run.Error == "" {
				run.Error = run.State
			}
			return Snapshot{}, fmt.Errorf("the backup failed: %s", run.Error)
		}
		if waited >= c.Timeout {
			return Snapshot{}, fmt.Errorf("%w (%s)", ErrTimeout, c.Timeout)
		}
		if err := sleep(ctx, c.Poll); err != nil {
			return Snapshot{}, err
		}
	}
}

// Available reports whether the fortress knows a backup app on this server,
// reachable or not.
func (c *Client) Available() bool {
	_, err := c.member()
	return !errors.Is(err, ErrNoMember)
}

// member picks the backup app on this server. One that is known but offline
// still counts, so the update waits for it instead of going on without a
// backup.
func (c *Client) member() (string, error) {
	var here []Member
	for _, m := range c.Fortress.Members() {
		if offers(m, Action) && sameDevice(m.Device, c.Device) {
			here = append(here, m)
		}
	}
	if len(here) == 0 {
		return "", ErrNoMember
	}
	sort.Slice(here, func(i, j int) bool { return here[i].ID < here[j].ID })
	for _, m := range here {
		if m.Online {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("the backup app on %s is not reachable", c.Device)
}

func (c *Client) call(ctx context.Context, member, method, path string, body []byte, out any) error {
	code, raw, err := c.Fortress.Call(ctx, member, method, path, body)
	if err != nil {
		return fmt.Errorf("the backup app did not answer: %w", err)
	}
	if code < 200 || code > 299 {
		var refusal struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &refusal) != nil || refusal.Error == "" {
			refusal.Error = strings.TrimSpace(string(bytes.ToValidUTF8(raw, nil)))
		}
		return fmt.Errorf("the backup app refused (%d %s)", code, refusal.Error)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("the backup app's answer is unreadable: %w", err)
	}
	return nil
}

func offers(m Member, action string) bool {
	for _, a := range m.Actions {
		if a == action {
			return true
		}
	}
	return false
}

// sameDevice compares server names the way people write them: "Tower",
// "tower" and "tower.lan" are one server.
func sameDevice(a, b string) bool {
	host := func(s string) string {
		s, _, _ = strings.Cut(strings.ToLower(strings.TrimSpace(s)), ".")
		return s
	}
	return host(a) != "" && host(a) == host(b)
}

// runID reads a run id the backup app sends as a string or as a number.
type runID string

func (r *runID) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*r = runID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*r = runID(n.String())
	return nil
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
