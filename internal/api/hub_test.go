package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTheSettingsPageHearsWhetherTollRollSendsTheMessages(t *testing.T) {
	for _, c := range []struct {
		active func(context.Context) bool
		want   string
	}{
		{nil, "{\"hub\":false}\n"},
		{func(context.Context) bool { return false }, "{\"hub\":false}\n"},
		{func(context.Context) bool { return true }, "{\"hub\":true}\n"},
	} {
		a := New(fakeSource{}, &fakeOverrides{}, &fakeRefresher{}, "").WithHub(c.active)
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/notify-hub", nil))
		if rr.Code != http.StatusOK || rr.Body.String() != c.want {
			t.Errorf("answer = %d %q, want %q", rr.Code, rr.Body, c.want)
		}
	}
}
