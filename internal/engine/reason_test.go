package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/resolver"
)

func TestEveryStatusCarriesItsReasonAsACode(t *testing.T) {
	st := &fakeStore{}
	cl := &fakeChangelog{}
	e := New(fakeCollector{list: []model.Container{
		{ID: "patch", Name: "patch", Repo: "ghcr.io/x/patch", Tag: "latest", ImageVersion: "1.0.0", Digest: "sha256:p1"},
		{ID: "pin", Name: "pin", Repo: "ghcr.io/x/pin", PinnedDigest: "sha256:q", Digest: "sha256:q"},
		{ID: "gone", Name: "gone", Repo: "ghcr.io/x/gone", Tag: "1.0.0", Digest: "sha256:g"},
		{ID: "fail", Name: "fail", Repo: "ghcr.io/x/fail", Tag: "1.0.0", Digest: "sha256:f"},
	}}, fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/patch": {tag: "latest", dig: "sha256:p2", verTag: "1.0.1", verDig: "sha256:p2"},
		"ghcr.io/x/gone":  {err: resolver.ErrRepoNotFound},
		"ghcr.io/x/fail":  {err: errors.New("timeout")},
	}}, cl, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := map[string]model.Reason{
		"patch": model.Why(model.ReasonPatch, "1.0.0", "1.0.1"),
		"pin":   model.Why(model.ReasonDigestPinned),
		"gone":  model.Why(model.ReasonRemoved),
		"fail":  model.Why(model.ReasonLookupFailed),
	}
	for id, w := range want {
		got := st.rows[id]
		if !reflect.DeepEqual(got.RiskCode, w) || got.RiskReason != w.String() {
			t.Errorf("%s: code %+v, reason %q; want %+v", id, got.RiskCode, got.RiskReason, w)
		}
	}
}

func TestBreakingNotesCarryTheirReleaseAndSignal(t *testing.T) {
	st := &fakeStore{}
	cl := &fakeChangelog{entries: []model.ReleaseEntry{{Tag: "v3.0.0", Body: "Breaking change: rename the volume first."}}}
	e := New(fakeCollector{list: []model.Container{
		{ID: "brk", Name: "brk", Repo: "ghcr.io/x/brk", Tag: "latest", ImageVersion: "2.0.0", Digest: "sha256:b1"},
	}}, fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/brk": {tag: "latest", dig: "sha256:b2", verTag: "v3.0.0", verDig: "sha256:b2"},
	}}, cl, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := model.Why(model.ReasonBreakingIn, "v3.0.0", "breaking change")
	if got := st.rows["brk"]; !reflect.DeepEqual(got.RiskCode, want) || got.Risk != model.RiskCritical {
		t.Errorf("code %+v, risk %s; want %+v and critical", got.RiskCode, got.Risk, want)
	}
}
