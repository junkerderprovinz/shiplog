package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kit "github.com/junkerderprovinz/parleyport/kit/notify"
)

// fakeChannels keeps a list and answers a test with ok, or with the code it
// was given.
type fakeChannels struct {
	list   []kit.Channel
	tested []string
	code   string
}

func (f *fakeChannels) Public() []kit.Channel { return append([]kit.Channel{}, f.list...) }

func (f *fakeChannels) Replace(next []kit.Channel) ([]kit.Channel, error) {
	for i, c := range next {
		if c.Kind == "" {
			return nil, kit.Problems{{Index: i, Field: "kind", Code: "required"}}
		}
	}
	f.list = next
	return next, nil
}

func (f *fakeChannels) Test(_ context.Context, ch kit.Channel) kit.Result {
	f.tested = append(f.tested, ch.ID)
	r := kit.Result{Channel: ch.ID, Code: f.code}
	if f.code != "" {
		r.Err = errTest
	}
	return r
}

type testError string

func (e testError) Error() string { return string(e) }

const errTest = testError("refused")

func call(h http.Handler, method, target, body string) (int, string) {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rr.Code, rr.Body.String()
}

func TestTheSettingsPageEditsTheChannels(t *testing.T) {
	ch := &fakeChannels{}
	h := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithChannels(ch).Handler()
	if code, body := call(h, "GET", "/api/notify", ""); code != 200 || body != "{\"channels\":[]}\n" {
		t.Errorf("GET = %d %s", code, body)
	}
	code, body := call(h, "PUT", "/api/notify", `{"channels":[{"id":"unraid","kind":"unraid","enabled":true}]}`)
	if code != 200 || !strings.Contains(body, `"id":"unraid"`) || len(ch.list) != 1 {
		t.Errorf("PUT = %d %s", code, body)
	}
	code, body = call(h, "PUT", "/api/notify", `{"channels":[{"id":"x"}]}`)
	if code != http.StatusBadRequest || !strings.Contains(body, `"error":"invalid"`) || !strings.Contains(body, `"problems"`) {
		t.Errorf("PUT of an invalid list = %d %s", code, body)
	}
}

func TestAChannelTestSaysHowItWent(t *testing.T) {
	ch := &fakeChannels{}
	h := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithChannels(ch).Handler()
	if code, body := call(h, "POST", "/api/notify/test", `{"channel":{"id":"discord","kind":"discord"}}`); code != 200 || body != "{\"ok\":true}\n" {
		t.Errorf("test = %d %s", code, body)
	}
	ch.code = "auth"
	if _, body := call(h, "POST", "/api/notify/test", `{"channel":{"id":"discord","kind":"discord"}}`); body != "{\"error\":\"auth\",\"ok\":false}\n" {
		t.Errorf("failed test = %s", body)
	}
	if len(ch.tested) != 2 || ch.tested[0] != "discord" {
		t.Errorf("tested = %v", ch.tested)
	}
}

func TestWithoutChannelsTheRoutesAreNotThere(t *testing.T) {
	h := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").Handler()
	if code, _ := call(h, "PUT", "/api/notify", `{"channels":[]}`); code == http.StatusOK {
		t.Error("PUT without a channel store succeeded")
	}
}
