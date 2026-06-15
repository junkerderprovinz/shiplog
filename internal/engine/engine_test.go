package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// --- fakes ---

type fakeCollector struct{ list []model.Container }

func (f fakeCollector) List(context.Context) ([]model.Container, error) { return f.list, nil }

type errCollector struct{}

func (errCollector) List(context.Context) ([]model.Container, error) {
	return nil, errors.New("socket down")
}

type resolveResult struct {
	tag, dig string
	err      error
}
type fakeResolver struct{ byRepo map[string]resolveResult }

func (f fakeResolver) Resolve(_ context.Context, repo, _, _ string) (string, string, error) {
	r, ok := f.byRepo[repo]
	if !ok {
		return "", "", errors.New("no such repo")
	}
	return r.tag, r.dig, r.err
}

// The fakes are written from the engine's concurrent workers, so they guard
// their state with a mutex (the real store/changelog are concurrency-safe).
type fakeChangelog struct {
	mu     sync.Mutex
	called int
}

func (f *fakeChangelog) Get(_ context.Context, _ model.Container, from, to string) (*model.Changelog, bool) {
	f.mu.Lock()
	f.called++
	f.mu.Unlock()
	return &model.Changelog{FromTag: from, ToTag: to, Provider: "fake"}, true
}

type fakeStore struct {
	mu   sync.Mutex
	rows map[string]model.UpdateStatus
}

func (f *fakeStore) Upsert(s model.UpdateStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]model.UpdateStatus{}
	}
	f.rows[s.Container.ID] = s
	return nil
}

func (f *fakeStore) Get(id string) (model.UpdateStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.rows[id]
	if !ok {
		return model.UpdateStatus{}, errors.New("not found")
	}
	return s, nil
}

// --- tests ---

func TestSweepClassifiesAndCapturesPerContainerErrors(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "a", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.2.0", Digest: "sha256:o", Source: "https://github.com/x/immich"},
		{ID: "b", Name: "redis", Repo: "docker.io/library/redis", Tag: "7.2.0", Digest: "sha256:r"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/immich":        {tag: "1.4.0", dig: "sha256:n"},
		"docker.io/library/redis": {err: errors.New("registry timeout")},
	}}
	cl := &fakeChangelog{}
	st := &fakeStore{}
	e := New(col, res, cl, st, time.Hour)

	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep returned: %v", err)
	}

	immich := st.rows["a"]
	if immich.Kind != model.KindMinor || immich.Risk != model.RiskMedium {
		t.Fatalf("immich: want minor/medium, got %s/%s", immich.Kind, immich.Risk)
	}
	if immich.NewestTag != "1.4.0" || immich.Changelog == nil {
		t.Fatalf("immich: expected newest tag + changelog, got %q / %v", immich.NewestTag, immich.Changelog)
	}
	if cl.called != 1 {
		t.Fatalf("changelog should be fetched once (only for the update), got %d", cl.called)
	}

	redis := st.rows["b"]
	if redis.Error == "" {
		t.Fatal("redis: resolver error must be captured in the row")
	}
	if redis.Risk != model.RiskUnknown {
		t.Fatalf("redis: failed lookup should be unknown risk, got %s", redis.Risk)
	}
	// The whole sweep still succeeded despite one container failing.
}

func TestSweepReturnsCollectorError(t *testing.T) {
	e := New(errCollector{}, fakeResolver{}, &fakeChangelog{}, &fakeStore{}, time.Hour)
	if err := e.Sweep(context.Background()); err == nil {
		t.Fatal("a collector failure must surface as a sweep error")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	e := New(fakeCollector{}, fakeResolver{}, &fakeChangelog{}, &fakeStore{}, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
