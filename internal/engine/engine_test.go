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
	tag, dig       string // newestTag (as-is) + same-tag digest
	verTag, verDig string // newest semver tag + its digest
	err            error
}
type fakeResolver struct{ byRepo map[string]resolveResult }

func (f fakeResolver) Resolve(_ context.Context, repo, _, _ string) (string, string, string, string, error) {
	r, ok := f.byRepo[repo]
	if !ok {
		return "", "", "", "", errors.New("no such repo")
	}
	return r.tag, r.dig, r.verTag, r.verDig, r.err
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
		{ID: "c", Name: "caddy", Repo: "docker.io/library/caddy", Tag: "2.7.0", Digest: "sha256:c", Source: "https://github.com/caddyserver/caddy"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/immich":        {tag: "1.4.0", dig: "sha256:n"},
		"docker.io/library/redis": {err: errors.New("registry timeout")},
		"docker.io/library/caddy": {tag: "2.7.0", dig: "sha256:c"}, // up to date
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
	// Changelog is fetched for every container that resolves (immich + caddy);
	// redis fails at resolve, so it never reaches the changelog step.
	if cl.called != 2 {
		t.Fatalf("changelog should be fetched for each resolved container, got %d", cl.called)
	}

	// caddy is up to date but still gets a changelog (the running version's notes).
	caddy := st.rows["c"]
	if caddy.HasUpdate() {
		t.Errorf("caddy: expected up-to-date (no update), got kind %s", caddy.Kind)
	}
	if caddy.Changelog == nil {
		t.Error("caddy: expected a changelog even when up to date")
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

func TestDecideRunningVersion(t *testing.T) {
	const (
		dRun = "sha256:running" // digest of the running image
		dNew = "sha256:newver"  // digest of the newest semver version
	)
	cases := []struct {
		name            string
		c               model.Container
		newestVerTag    string
		newestVerDigest string
		prior           model.UpdateStatus
		hasPrior        bool
		want            string
	}{
		{
			name: "pinned version tag is the running version",
			c:    model.Container{Tag: "1.7.0", Digest: dRun},
			want: "1.7.0",
		},
		{
			name:            "pinned tag wins even with a prior memory",
			c:               model.Container{Tag: "2.1.0", Digest: dRun},
			prior:           model.UpdateStatus{RunningVersion: "9.9.9"},
			hasPrior:        true,
			newestVerTag:    "2.2.0",
			newestVerDigest: dNew,
			want:            "2.1.0",
		},
		{
			name:            "latest proven to be newest by matching digest",
			c:               model.Container{Tag: "latest", Digest: dNew},
			newestVerTag:    "1.8.0",
			newestVerDigest: dNew, // running image == the newest version's image
			want:            "1.8.0",
		},
		{
			name:            "latest unchanged carries the remembered version forward",
			c:               model.Container{Tag: "latest", Digest: dRun},
			newestVerTag:    "1.9.0",
			newestVerDigest: dNew, // running != newest → not proven this sweep
			prior:           model.UpdateStatus{Container: model.Container{Digest: dRun}, RunningVersion: "1.7.0"},
			hasPrior:        true,
			want:            "1.7.0",
		},
		{
			name:            "latest lagging a newer published tag is NOT mislabeled",
			c:               model.Container{Tag: "latest", Digest: dRun}, // running an older image
			newestVerTag:    "2.0.0",
			newestVerDigest: dNew, // 2.0.0's digest differs from what's running
			want:            "",   // must NOT claim the running image is 2.0.0
		},
		{
			name:            "first sight of an out-of-date latest is unknown",
			c:               model.Container{Tag: "latest", Digest: dRun},
			newestVerTag:    "1.8.0",
			newestVerDigest: dNew,
			want:            "",
		},
		{
			name:            "repo without semver tags stays unknown",
			c:               model.Container{Tag: "latest", Digest: dRun},
			newestVerTag:    "", // no semver tags
			newestVerDigest: "",
			want:            "",
		},
		{
			name:         "empty running digest never carries forward",
			c:            model.Container{Tag: "latest", Digest: ""},
			prior:        model.UpdateStatus{Container: model.Container{Digest: ""}, RunningVersion: "1.7.0"},
			hasPrior:     true,
			newestVerTag: "1.8.0",
			want:         "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideRunningVersion(tc.c, tc.newestVerTag, tc.newestVerDigest, tc.prior, tc.hasPrior)
			if got != tc.want {
				t.Errorf("decideRunningVersion = %q, want %q", got, tc.want)
			}
		})
	}
}

// After a :latest container updates under ShipLog's watch, the stored status
// must carry the version we resolved so the next sweep can show "prev -> new".
func TestSweepRemembersLatestVersion(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "oh", Name: "openhands", Repo: "ghcr.io/x/openhands", Tag: "latest", Digest: "sha256:v18"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		// :latest echoed as newestTag; running digest == newest VERSION's digest
		// → proven to be running 1.8.0.
		"ghcr.io/x/openhands": {tag: "latest", dig: "sha256:v18", verTag: "1.8.0", verDig: "sha256:v18"},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := st.rows["oh"].RunningVersion; got != "1.8.0" {
		t.Fatalf("running version not remembered for up-to-date :latest: got %q, want 1.8.0", got)
	}
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
