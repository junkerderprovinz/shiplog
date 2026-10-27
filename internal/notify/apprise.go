package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Apprise posts to the stateful notify endpoint of an Apprise API server
// (https://github.com/caronc/apprise-api), such as http://host:8000/notify/shiplog,
// which passes each message on to every service saved under that key.
type Apprise struct {
	url    string
	client *http.Client
}

// NewApprise returns an Apprise notifier, or nil when target is empty or not an
// http(s) URL.
func NewApprise(target string) *Apprise {
	target = strings.TrimSpace(target)
	u, err := url.Parse(target)
	if target == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil
	}
	return &Apprise{url: target, client: &http.Client{Timeout: 20 * time.Second}}
}

// Where names the server without the key in the path or a login in the URL,
// for logs and errors.
func (a *Apprise) Where() string {
	u, _ := url.Parse(a.url)
	return u.Scheme + "://" + u.Host
}

// Notify posts a per-update message. A nil receiver does nothing.
func (a *Apprise) Notify(ctx context.Context, st model.UpdateStatus) error {
	if a == nil {
		return nil
	}
	text, html := format(st)
	return a.SendMessage(ctx, text, html)
}

// SendMessage posts the plain text; Apprise renders it for each service. A nil
// receiver does nothing.
func (a *Apprise) SendMessage(ctx context.Context, text, _ string) error {
	if a == nil {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"body": text, "type": "info", "format": "text"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apprise %s: invalid request", a.Where())
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		// The client's error quotes the whole URL, key included.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("apprise %s: %w", a.Where(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apprise %s: status %d", a.Where(), resp.StatusCode)
	}
	return nil
}
