package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// batchNotifier records the batches it was handed next to single notices.
type batchNotifier struct {
	fakeNotifier
	mu      sync.Mutex
	batches [][]model.UpdateStatus
}

func (d *batchNotifier) NotifyBatch(_ context.Context, sts []model.UpdateStatus) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.batches = append(d.batches, sts)
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

func TestABatchSweepSendsOneMessageForEveryNewUpdate(t *testing.T) {
	col, res, st := threeMoved()
	nf := &batchNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithBatch(true)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nf.count() != 0 {
		t.Errorf("got %d single notices, want none", nf.count())
	}
	if len(nf.batches) != 1 || len(nf.batches[0]) != 3 {
		t.Fatalf("batches = %v, want one with three updates", nf.batches)
	}
}

func TestABatchSweepWithNothingNewSendsNothing(t *testing.T) {
	col, res, st := threeMoved()
	nf := &batchNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithBatch(true)
	_ = e.Sweep(context.Background())
	nf.batches = nil
	// The second sweep sees the same images, which were announced already.
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(nf.batches) != 0 || nf.count() != 0 {
		t.Errorf("got %d notices and batches %v, want nothing", nf.count(), nf.batches)
	}
}

func TestWithoutTheBatchEveryUpdateIsItsOwnNotice(t *testing.T) {
	col, res, st := threeMoved()
	nf := &batchNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if nf.count() != 3 || len(nf.batches) != 0 {
		t.Errorf("got %d notices and %d batches, want 3 and none", nf.count(), len(nf.batches))
	}
}

// A notifier that cannot send a batch keeps sending each update on its own.
func TestTheBatchNeedsANotifierThatCanSendOne(t *testing.T) {
	col, res, st := threeMoved()
	nf := &fakeNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithBatch(true)
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

// cancelStore cancels the sweep's context once it has stored every row, as a
// settings save does when it restarts the engine.
type cancelStore struct {
	*fakeStore
	mu     sync.Mutex
	left   int
	cancel context.CancelFunc
}

func (c *cancelStore) Upsert(s model.UpdateStatus) error {
	err := c.fakeStore.Upsert(s)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.left--; c.left == 0 {
		c.cancel()
	}
	return err
}

// ctxBatchNotifier records whether the context of each batch was still live
// and whether it carried a deadline.
type ctxBatchNotifier struct {
	batchNotifier
	live    []bool
	bounded []bool
}

func (d *ctxBatchNotifier) NotifyBatch(ctx context.Context, sts []model.UpdateStatus) error {
	_, bounded := ctx.Deadline()
	d.mu.Lock()
	d.live = append(d.live, ctx.Err() == nil)
	d.bounded = append(d.bounded, bounded)
	d.mu.Unlock()
	return d.batchNotifier.NotifyBatch(ctx, sts)
}

// A slow hub can take a while to turn the updates down, and the channels
// still need their turn after it.
func TestAnUncancelledSweepSendsItsBatchWithoutTheShortBound(t *testing.T) {
	col, res, st := threeMoved()
	nf := &ctxBatchNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).WithBatch(true)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(nf.bounded) != 1 || nf.bounded[0] {
		t.Errorf("bounded = %v, want one batch on the sweep's own context", nf.bounded)
	}
}

func TestACancelledSweepStillSendsItsBatch(t *testing.T) {
	col, res, st := threeMoved()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	nf := &ctxBatchNotifier{}
	e := New(col, res, &fakeChangelog{}, &cancelStore{fakeStore: st, left: 3, cancel: cancel}, time.Hour).WithNotifier(nf).WithBatch(true)
	_ = e.Sweep(ctx)
	if ctx.Err() == nil {
		t.Fatal("the sweep should have been cancelled")
	}
	if len(nf.batches) != 1 || len(nf.batches[0]) != 3 || !nf.live[0] || !nf.bounded[0] {
		t.Errorf("batches = %v, live = %v, bounded = %v, want the three stored updates sent on a live, bounded context", nf.batches, nf.live, nf.bounded)
	}
}

func TestAHeldBackUpdateGoesOutOnceItReachesTheMinimum(t *testing.T) {
	nf := &fakeNotifier{}
	e := New(fakeCollector{}, fakeResolver{}, &fakeChangelog{}, &fakeStore{}, time.Hour).WithNotifier(nf).WithMinRisk(model.RiskHigh)
	for _, tc := range []struct {
		name      string
		was, now  model.RiskLevel
		wantSends int
	}{
		{"rose to the minimum", model.RiskMedium, model.RiskHigh, 1},
		{"rose past it", model.RiskLow, model.RiskCritical, 1},
		{"still below", model.RiskLow, model.RiskMedium, 0},
		{"announced already", model.RiskHigh, model.RiskCritical, 0},
	} {
		nf.n = 0
		prior := model.UpdateStatus{NewestDigest: "sha256:new", Kind: model.KindDigest, Risk: tc.was}
		e.maybeNotify(context.Background(), model.UpdateStatus{NewestDigest: "sha256:new", Kind: model.KindDigest, Risk: tc.now}, prior, true)
		if nf.count() != tc.wantSends {
			t.Errorf("%s: got %d notices, want %d", tc.name, nf.count(), tc.wantSends)
		}
	}
}
