package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// digestNotifier records the digests it was handed next to single notices.
type digestNotifier struct {
	fakeNotifier
	mu      sync.Mutex
	digests [][]model.UpdateStatus
}

func (d *digestNotifier) NotifyDigest(_ context.Context, sts []model.UpdateStatus) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.digests = append(d.digests, sts)
	return nil
}

// threeMoved has three known containers whose rolling tag moved since the
// last sweep.
func threeMoved() (fakeCollector, fakeResolver, *fakeStore) {
	col := fakeCollector{}
	res := fakeResolver{byRepo: map[string]resolveResult{}}
	st := &fakeStore{rows: map[string]model.UpdateStatus{}}
	for _, id := range []string{"a", "b", "c"} {
		repo := "ghcr.io/x/" + id
		col.list = append(col.list, model.Container{ID: id, Name: "app-" + id, Repo: repo, Tag: "latest", Digest: "sha256:old"})
		res.byRepo[repo] = resolveResult{tag: "latest", dig: "sha256:new-" + id}
		st.rows[id] = model.UpdateStatus{Container: model.Container{ID: id, Name: "app-" + id, Digest: "sha256:old"}, NewestDigest: "sha256:old", Kind: model.KindNone}
	}
	return col, res, st
}

func TestADigestSweepSendsOneMessageForEveryNewUpdate(t *testing.T) {
	col, res, st := threeMoved()
	nf := &digestNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithDigest(true)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nf.count() != 0 {
		t.Errorf("got %d single notices, want none", nf.count())
	}
	if len(nf.digests) != 1 || len(nf.digests[0]) != 3 {
		t.Fatalf("digests = %v, want one with three updates", nf.digests)
	}
}

func TestADigestSweepWithNothingNewSendsNothing(t *testing.T) {
	col, res, st := threeMoved()
	nf := &digestNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithDigest(true)
	_ = e.Sweep(context.Background())
	nf.digests = nil
	// The second sweep sees the same images, which were announced already.
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(nf.digests) != 0 || nf.count() != 0 {
		t.Errorf("got %d notices and digests %v, want nothing", nf.count(), nf.digests)
	}
}

func TestWithoutTheDigestEveryUpdateIsItsOwnNotice(t *testing.T) {
	col, res, st := threeMoved()
	nf := &digestNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nf.count() != 3 || len(nf.digests) != 0 {
		t.Errorf("got %d notices and %d digests, want 3 and none", nf.count(), len(nf.digests))
	}
}

// A notifier that cannot send a digest keeps sending each update on its own.
func TestTheDigestNeedsANotifierThatCanSendOne(t *testing.T) {
	col, res, st := threeMoved()
	nf := &fakeNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithDigest(true)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nf.count() != 3 {
		t.Errorf("got %d notices, want 3", nf.count())
	}
}

func TestTheMinimumRiskHoldsBackALowerUpdate(t *testing.T) {
	for min, want := range map[model.RiskLevel]int{"": 1, model.RiskLow: 1, model.RiskMedium: 0, model.RiskCritical: 0} {
		col, res, st := threeMoved()
		col.list = col.list[:1]
		nf := &fakeNotifier{}
		e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithMinRisk(min)
		if err := e.Sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := st.rows["a"].Risk; got != model.RiskLow {
			t.Fatalf("a moved rolling tag should be low risk, got %s", got)
		}
		if nf.count() != want {
			t.Errorf("min %q: got %d notices for a low update, want %d", min, nf.count(), want)
		}
	}
}

func TestTheMinimumRiskLetsAnUnknownRiskThrough(t *testing.T) {
	nf := &fakeNotifier{}
	e := New(fakeCollector{}, fakeResolver{}, &fakeChangelog{}, &fakeStore{}, time.Hour).WithNotifier(nf).WithMinRisk(model.RiskHigh)
	prior := model.UpdateStatus{NewestDigest: "sha256:old", Kind: model.KindNone}
	for risk, want := range map[model.RiskLevel]int{model.RiskUnknown: 1, model.RiskMedium: 0, model.RiskHigh: 1, model.RiskCritical: 1} {
		nf.n = 0
		e.maybeNotify(context.Background(), model.UpdateStatus{NewestDigest: "sha256:new", Kind: model.KindDigest, Risk: risk}, prior, true)
		if nf.count() != want {
			t.Errorf("risk %s: got %d notices, want %d", risk, nf.count(), want)
		}
	}
}
