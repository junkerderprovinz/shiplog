// Package notify sends an optional Matrix message the first time a new container
// update is sighted. It is off unless MATRIX_HOMESERVER/TOKEN/ROOM are all set.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Matrix posts to a room via the client-server API using an access token.
type Matrix struct {
	hs     string // homeserver base URL, no trailing slash
	token  string
	room   string // room id (!..:hs) or alias (#..:hs)
	client *http.Client
}

// New returns a Matrix notifier, or nil if any of homeserver/token/room is empty.
func New(homeserver, token, room string) *Matrix {
	homeserver = strings.TrimRight(strings.TrimSpace(homeserver), "/")
	token = strings.TrimSpace(token)
	room = strings.TrimSpace(room)
	if homeserver == "" || token == "" || room == "" {
		return nil
	}
	return &Matrix{hs: homeserver, token: token, room: room, client: &http.Client{Timeout: 20 * time.Second}}
}

func (m *Matrix) auth(req *http.Request) { req.Header.Set("Authorization", "Bearer "+m.token) }

// Whoami verifies the token works, for a plain startup log. nil → not configured.
func (m *Matrix) Whoami(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("matrix not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.hs+"/_matrix/client/v3/account/whoami", nil)
	if err != nil {
		return err
	}
	m.auth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("whoami status %d", resp.StatusCode)
	}
	return nil
}

// roomID returns the room as-is when it's an id, or resolves a #alias.
func (m *Matrix) roomID(ctx context.Context) (string, error) {
	if !strings.HasPrefix(m.room, "#") {
		return m.room, nil // already a room id (or best effort)
	}
	u := m.hs + "/_matrix/client/v3/directory/room/" + url.PathEscape(m.room)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	m.auth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve alias status %d", resp.StatusCode)
	}
	var b struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return "", err
	}
	if b.RoomID == "" {
		return "", fmt.Errorf("alias resolved to empty room_id")
	}
	return b.RoomID, nil
}

// Notify posts an update message. nil receiver → no-op (caller-friendly).
func (m *Matrix) Notify(ctx context.Context, st model.UpdateStatus) error {
	if m == nil {
		return nil
	}
	rid, err := m.roomID(ctx)
	if err != nil {
		return err
	}
	text, html := format(st)
	body, _ := json.Marshal(map[string]any{
		"msgtype":        "m.text",
		"body":           text,
		"format":         "org.matrix.custom.html",
		"formatted_body": html,
	})
	txn := fmt.Sprintf("shiplog-%s-%d", safeTxn(st.Container.ID), time.Now().UnixNano())
	u := m.hs + "/_matrix/client/v3/rooms/" + url.PathEscape(rid) + "/send/m.room.message/" + txn
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	m.auth(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send status %d", resp.StatusCode)
	}
	return nil
}

func safeTxn(s string) string {
	if len(s) > 12 {
		s = s[:12]
	}
	if s == "" {
		s = "x"
	}
	return s
}

// format builds the plain + HTML message bodies for an update.
func format(st model.UpdateStatus) (text, html string) {
	name := st.Container.Name
	from := st.Container.Tag
	to := st.NewestTag
	if st.Changelog != nil && len(st.Changelog.Entries) > 0 && st.Changelog.Entries[0].Tag != "" {
		to = st.Changelog.Entries[0].Tag
	}
	risk := strings.ToUpper(string(st.Risk))

	link := ""
	if st.Changelog != nil {
		link = repoRoot(st.Changelog.URL)
	}
	if link == "" {
		link = repoRoot(st.Container.Source)
	}

	text = fmt.Sprintf("🚢 ShipLog · %s: update %s → %s (risk: %s)", name, from, to, risk)
	html = fmt.Sprintf("🚢 <b>ShipLog</b> · <b>%s</b>: update <code>%s</code> → <code>%s</code> (risk: %s)",
		esc(name), esc(from), esc(to), esc(risk))
	if link != "" {
		text += "\n" + link
		html += `<br/><a href="` + esc(link) + `">` + esc(link) + `</a>`
	}
	return text, html
}

// repoRoot keeps only scheme://host/owner/repo from a URL (drops /compare/...).
func repoRoot(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	parts := strings.SplitN(u, "/", 7) // scheme,"",host,owner,repo,...
	if len(parts) < 5 || !strings.HasPrefix(u, "http") {
		return strings.TrimSuffix(u, ".git")
	}
	return strings.TrimSuffix(strings.Join(parts[:5], "/"), ".git")
}

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
