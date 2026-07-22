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
	mu        sync.Mutex
	rows      map[string]model.UpdateStatus
	overrides map[string]string
}

func (f *fakeStore) SourceOverrides() (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.overrides, nil
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

func (f *fakeStore) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rows, id)
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

// The "ignore third-party containers" filter drops containers that lack Unraid's
// net.unraid.docker.managed label (Compose/Dockhand/docker run) from the sweep,
// and forgets any row they already had, while leaving Unraid-managed containers
// fully tracked. Off by default → every container is tracked.
func TestSweepIgnoreUnmanagedFiltersThirdParty(t *testing.T) {
	mk := func() fakeCollector {
		return fakeCollector{list: []model.Container{
			{ID: "u", Name: "krusader", Repo: "ghcr.io/x/krusader", Tag: "1.0.0", Digest: "sha256:k", Managed: true},
			{ID: "t", Name: "compose-app", Repo: "docker.io/library/nginx", Tag: "1.27.0", Digest: "sha256:n", Managed: false},
		}}
	}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/krusader":      {tag: "1.1.0", dig: "sha256:kn"},
		"docker.io/library/nginx": {tag: "1.28.0", dig: "sha256:nn"},
	}}

	// Default (off): both the managed and the third-party container are tracked.
	stOff := &fakeStore{}
	if err := New(mk(), res, &fakeChangelog{}, stOff, time.Hour).Sweep(context.Background()); err != nil {
		t.Fatalf("sweep (off) returned: %v", err)
	}
	if _, ok := stOff.rows["u"]; !ok {
		t.Error("off: managed container must be tracked")
	}
	if _, ok := stOff.rows["t"]; !ok {
		t.Error("off: third-party container must be tracked when the filter is off")
	}

	// On: the third-party container is skipped AND its pre-existing row is deleted.
	stOn := &fakeStore{rows: map[string]model.UpdateStatus{
		"t": {Container: model.Container{ID: "t", Name: "compose-app"}},
	}}
	e := New(mk(), res, &fakeChangelog{}, stOn, time.Hour).WithIgnoreUnmanaged(true)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep (on) returned: %v", err)
	}
	if _, ok := stOn.rows["u"]; !ok {
		t.Error("on: managed container must still be tracked")
	}
	if _, ok := stOn.rows["t"]; ok {
		t.Error("on: third-party container must be dropped and its prior row deleted")
	}
}

// A rolling ":latest" whose resolved version actually moved (7.1.0 -> 7.2.0) must
// be classified by that version delta (minor/medium), not the flat tag comparison
// (latest vs latest -> digest/low) that cannot see the jump.
func TestSweepRollingTagUsesVersionDeltaForRisk(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "oc", Name: "opencloud", Repo: "ghcr.io/o/opencloud", Tag: "latest", Digest: "sha256:run", ImageVersion: "7.1.0"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		// rolling tag unchanged (latest==latest), but the resolved newest semver is 7.2.0
		"ghcr.io/o/opencloud": {tag: "latest", dig: "sha256:new", verTag: "7.2.0", verDig: "sha256:v72"},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	oc := st.rows["oc"]
	if oc.Kind != model.KindMinor || oc.Risk != model.RiskMedium {
		t.Fatalf("rolling tag with version jump: want minor/medium, got %s/%s", oc.Kind, oc.Risk)
	}
	if oc.RunningVersion != "7.1.0" {
		t.Fatalf("running version: want 7.1.0 (from image label), got %q", oc.RunningVersion)
	}
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
			name: "image label version shows immediately for an unproven latest",
			// First sight, digest doesn't match the newest version, no prior — but
			// the image declares its own version via the OCI label.
			c:               model.Container{Tag: "latest", Digest: dRun, ImageVersion: "2.7.2"},
			newestVerTag:    "2.8.0",
			newestVerDigest: dNew,
			want:            "2.7.2",
		},
		{
			name: "digest proof outranks the image label",
			c:    model.Container{Tag: "latest", Digest: dNew, ImageVersion: "1.0.0"},
			// running image IS the newest version by digest → trust the proof, not
			// a possibly-stale self-declared label.
			newestVerTag:    "1.8.0",
			newestVerDigest: dNew,
			want:            "1.8.0",
		},
		{
			name: "remembered version outranks the image label",
			c:    model.Container{Tag: "latest", Digest: dRun, ImageVersion: "9.9.9"},
			prior: model.UpdateStatus{
				Container: model.Container{Digest: dRun}, RunningVersion: "1.7.0",
			},
			hasPrior:        true,
			newestVerTag:    "1.9.0",
			newestVerDigest: dNew,
			want:            "1.7.0",
		},
		{
			name: "non-version image label is ignored",
			// A revision SHA (org.opencontainers.image.revision fallback) isn't
			// version-like, so it must not be shown as a version.
			c:               model.Container{Tag: "latest", Digest: dRun, ImageVersion: "6e5f64bd"},
			newestVerTag:    "2.0.0",
			newestVerDigest: dNew,
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

// When the registry lookup fails, a container with no remembered version should
// still show its image-declared version (or pinned tag) rather than blank.
func TestSweepResolveErrorFallsBackToImageVersion(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "bv", Name: "bombvault", Repo: "ghcr.io/x/bombvault", Tag: "latest", Digest: "sha256:d", ImageVersion: "3.0.1"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/bombvault": {err: errors.New("registry unreachable")},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := st.rows["bv"]
	if row.Error == "" {
		t.Fatal("resolver error must be captured")
	}
	if row.RunningVersion != "3.0.1" {
		t.Fatalf("running version on resolve error = %q, want 3.0.1 (from image label)", row.RunningVersion)
	}
}

// A transient resolve failure (e.g. a 429 burst) must NOT blank a container we
// already resolved: the prior verdict + changelog are carried forward unchanged
// and no error is surfaced. Only the CheckedAt timestamp advances.
func TestSweepTransientFailureKeepsPriorVerdict(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "im", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "latest", Digest: "sha256:d"},
	}}
	st := &fakeStore{}
	// Seed a good prior row (as a successful earlier sweep would have stored).
	prior := model.UpdateStatus{
		Container:      model.Container{ID: "im", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "latest", Digest: "sha256:d"},
		RunningVersion: "1.122.0",
		NewestTag:      "1.124.0",
		Kind:           model.KindMinor,
		Risk:           model.RiskMedium,
		RiskReason:     "2 minor versions",
		Changelog:      &model.Changelog{Provider: "github", Raw: "notes"},
	}
	_ = st.Upsert(prior)

	// This sweep's resolve fails transiently.
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/immich": {err: errors.New("tags/list: rate limited (429)")},
	}}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	row := st.rows["im"]
	if row.Error != "" {
		t.Errorf("transient failure surfaced an error %q; should stay silent with a prior row", row.Error)
	}
	if row.Kind != model.KindMinor || row.Risk != model.RiskMedium {
		t.Errorf("verdict not carried forward: kind=%s risk=%s", row.Kind, row.Risk)
	}
	if row.NewestTag != "1.124.0" || row.RunningVersion != "1.122.0" {
		t.Errorf("versions not carried forward: newest=%q running=%q", row.NewestTag, row.RunningVersion)
	}
	if row.Changelog == nil || row.Changelog.Raw != "notes" {
		t.Errorf("changelog not carried forward: %#v", row.Changelog)
	}
}

// A transient resolve failure with NO usable prior must fall back to the bare
// "unknown" verdict and surface the honest error.
func TestSweepTransientFailureNoPriorSurfacesError(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "new", Name: "fresh", Repo: "ghcr.io/x/fresh", Tag: "latest", Digest: "sha256:d", ImageVersion: "2.0.0"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/fresh": {err: errors.New("tags/list: rate limited (429)")},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := st.rows["new"]
	if row.Error == "" {
		t.Error("first-sight transient failure must surface an error")
	}
	if row.Risk != model.RiskUnknown {
		t.Errorf("risk = %s, want unknown", row.Risk)
	}
	if row.RunningVersion != "2.0.0" {
		t.Errorf("running version = %q, want the image-label 2.0.0", row.RunningVersion)
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

// countingResolver records how often each (repo, tag) is resolved.
type countingResolver struct {
	mu    sync.Mutex
	calls map[string]int
	res   resolveResult
}

func (c *countingResolver) Resolve(_ context.Context, repo, tag, _ string) (string, string, string, string, error) {
	c.mu.Lock()
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[repo+"|"+tag]++
	c.mu.Unlock()
	return c.res.tag, c.res.dig, c.res.verTag, c.res.verDig, c.res.err
}

func TestSweepSkipsContainersWithoutUpstream(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "l", Name: "local", Repo: "docker.io/library/myapp", Tag: "latest", IsLocal: true},
		{ID: "p", Name: "pinned", Repo: "ghcr.io/x/y", Tag: "", PinnedDigest: "sha256:aaa"},
		// The compose-style pin KEEPS a tag for readability; Docker still pulls
		// the digest, so an "update available" verdict would be un-actionable.
		{ID: "tp", Name: "tagged-pin", Repo: "ghcr.io/x/y", Tag: "1.2.3", PinnedDigest: "sha256:aaa"},
		{ID: "i", Name: "byid", Repo: "", Tag: ""},
	}}
	res := &countingResolver{}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)

	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep returned: %v", err)
	}
	if len(res.calls) != 0 {
		t.Errorf("resolver was called for upstream-less containers: %v", res.calls)
	}
	for id, wantReason := range map[string]string{
		"l":  "no registry digest (built or loaded locally) — not checked against a registry",
		"p":  "pinned by digest — updates only when the pin changes",
		"tp": "pinned by digest — updates only when the pin changes",
		"i":  "referenced by image ID — no tag to compare",
	} {
		row := st.rows[id]
		if row.Kind != model.KindNone || row.Risk != model.RiskNone {
			t.Errorf("%s: kind/risk = %s/%s, want none/none", id, row.Kind, row.Risk)
		}
		if row.RiskReason != wantReason {
			t.Errorf("%s: reason = %q, want %q", id, row.RiskReason, wantReason)
		}
		if row.Error != "" {
			t.Errorf("%s: unexpected error %q", id, row.Error)
		}
	}
}

func TestSweepResolvesEachRepoTagOnce(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "a", Name: "db1", Repo: "docker.io/library/postgres", Tag: "16", Digest: "sha256:p"},
		{ID: "b", Name: "db2", Repo: "docker.io/library/postgres", Tag: "16", Digest: "sha256:p"},
		{ID: "c", Name: "db3-stopped", Repo: "docker.io/library/postgres", Tag: "16", Digest: "sha256:p", State: "exited"},
		{ID: "d", Name: "other", Repo: "docker.io/library/redis", Tag: "7", Digest: "sha256:r"},
	}}
	res := &countingResolver{res: resolveResult{tag: "16", dig: "sha256:p"}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)

	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep returned: %v", err)
	}
	if got := res.calls["docker.io/library/postgres|16"]; got != 1 {
		t.Errorf("postgres:16 resolved %d times, want exactly 1 for the whole sweep", got)
	}
	if got := res.calls["docker.io/library/redis|7"]; got != 1 {
		t.Errorf("redis:7 resolved %d times, want 1", got)
	}
	if len(st.rows) != 4 {
		t.Errorf("stored %d rows, want 4 (every container gets its own row)", len(st.rows))
	}
}

func TestCheckAcceptsMirrorDigestAsCurrent(t *testing.T) {
	// The running image was pulled through a mirror: its primary digest differs
	// from the upstream one, but RepoDigests carries the upstream digest too.
	col := fakeCollector{list: []model.Container{
		{
			ID: "m", Name: "mirrored", Repo: "docker.io/library/caddy", Tag: "2.7.0",
			Digest:  "sha256:mirror",
			Digests: []string{"sha256:mirror", "sha256:upstream"},
		},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"docker.io/library/caddy": {tag: "2.7.0", dig: "sha256:upstream"},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)

	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep returned: %v", err)
	}
	row := st.rows["m"]
	if row.Kind != model.KindNone {
		t.Errorf("kind = %s, want none (mirror digest matches upstream, no phantom drift)", row.Kind)
	}
}
