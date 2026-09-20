package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/cafeed"
	"github.com/junkerderprovinz/shiplog/internal/model"
	"github.com/junkerderprovinz/shiplog/internal/resolver"
)

// fakeCAFeed answers every lookup with the same result.
type fakeCAFeed struct {
	result cafeed.Result
	ok     bool
}

func (f fakeCAFeed) Lookup(name, repo, templateURL string) (cafeed.Result, bool) {
	return f.result, f.ok
}

type fakeCollector struct{ list []model.Container }

func (f fakeCollector) List(context.Context) ([]model.Container, error) { return f.list, nil }

type errCollector struct{}

func (errCollector) List(context.Context) ([]model.Container, error) {
	return nil, errors.New("socket down")
}

type resolveResult struct {
	tag, dig       string // newest tag and same-tag digest
	verTag, verDig string // newest semver tag and its digest
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

// The fakes are called from concurrent workers, hence the mutexes.
type fakeChangelog struct {
	mu         sync.Mutex
	called     int
	raw        string
	entries    []model.ReleaseEntry
	deprecated bool
}

func (f *fakeChangelog) Get(_ context.Context, _ model.Container, from, to string) (*model.Changelog, bool) {
	f.mu.Lock()
	f.called++
	f.mu.Unlock()
	return &model.Changelog{FromTag: from, ToTag: to, Provider: "fake", Raw: f.raw, Entries: f.entries, Deprecated: f.deprecated}, true
}

type fakeNotifier struct {
	mu sync.Mutex
	n  int
}

func (f *fakeNotifier) Notify(context.Context, model.UpdateStatus) error {
	f.mu.Lock()
	f.n++
	f.mu.Unlock()
	return nil
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n
}

type fakeStore struct {
	mu         sync.Mutex
	rows       map[string]model.UpdateStatus
	overrides  map[string]string
	suppressed map[string]bool
}

func (f *fakeStore) SourceOverrides() (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.overrides, nil
}

func (f *fakeStore) SuppressedUnmaintained() (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.suppressed, nil
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

func (f *fakeStore) List() ([]model.UpdateStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]model.UpdateStatus, 0, len(f.rows))
	for _, s := range f.rows {
		out = append(out, s)
	}
	return out, nil
}

func TestSweepFlagsUnmaintained(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "gone", Name: "GoneApp", Repo: "ghcr.io/x/gone", Tag: "1.0.0", Digest: "sha256:g", Managed: true},
		{ID: "rm", Name: "RemovedApp", Repo: "ghcr.io/x/removed", Tag: "1.0.0", Digest: "sha256:r", Managed: true},
		{ID: "fine", Name: "FineApp", Repo: "ghcr.io/x/fine", Tag: "1.0.0", Digest: "sha256:f", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/gone":    {err: resolver.ErrRepoNotFound},
		"ghcr.io/x/removed": {tag: "1.0.0", dig: "sha256:r"},
		"ghcr.io/x/fine":    {tag: "1.0.0", dig: "sha256:f"},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"removedapp": "http://ca/removed.xml", "fineapp": "http://ca/fine.xml"}
	}
	e.checkURL = func(_ context.Context, url string) int {
		if strings.Contains(url, "removed") {
			return 404
		}
		return 200
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if g := st.rows["gone"]; !g.Unmaintained || g.UnmaintainedReason != "Image no longer in the registry" {
		t.Fatalf("gone: want unmaintained/image gone, got %v/%q", g.Unmaintained, g.UnmaintainedReason)
	}
	if r := st.rows["rm"]; !r.Unmaintained || r.UnmaintainedReason != "Removed from Community Applications" {
		t.Fatalf("removed: want unmaintained/removed-from-CA, got %v/%q", r.Unmaintained, r.UnmaintainedReason)
	}
	if f := st.rows["fine"]; f.Unmaintained {
		t.Fatalf("fine: must not be unmaintained, got reason %q", f.UnmaintainedReason)
	}
}

// A dead raw template path counts only when the GitHub repo is gone as well.
func TestSweepGithubRawURLRotIsNotFlaggedUnmaintained(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "moved", Name: "MovedApp", Repo: "ghcr.io/x/moved", Tag: "1.0.0", Digest: "sha256:m", Managed: true},
		{ID: "gone", Name: "TrulyGoneApp", Repo: "ghcr.io/x/gone2", Tag: "1.0.0", Digest: "sha256:g", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/moved": {tag: "1.0.0", dig: "sha256:m"},
		"ghcr.io/x/gone2": {tag: "1.0.0", dig: "sha256:g"},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{
			"movedapp":     "https://raw.githubusercontent.com/someowner/renamed-repo/main/app/app.xml",
			"trulygoneapp": "https://raw.githubusercontent.com/anotherowner/dead-repo/main/app/app.xml",
		}
	}
	e.checkURL = func(_ context.Context, url string) int {
		switch url {
		case "https://raw.githubusercontent.com/someowner/renamed-repo/main/app/app.xml":
			return 404
		case "https://github.com/someowner/renamed-repo":
			return 200
		case "https://raw.githubusercontent.com/anotherowner/dead-repo/main/app/app.xml":
			return 404
		case "https://github.com/anotherowner/dead-repo":
			return 404
		}
		t.Fatalf("unexpected checkURL call: %s", url)
		return 0
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if m := st.rows["moved"]; m.Unmaintained {
		t.Fatalf("moved: a dead raw path with a live repo must not be flagged unmaintained, got reason %q", m.UnmaintainedReason)
	}
	if g := st.rows["gone"]; !g.Unmaintained || g.UnmaintainedReason != "Removed from Community Applications" {
		t.Fatalf("gone: a dead raw path and a dead repo must be flagged, got %v/%q", g.Unmaintained, g.UnmaintainedReason)
	}
}

// Only a 404 or 410 on the repo page confirms the removal; a rate limit, a
// server error or a failed request proves nothing.
func TestSweepAmbiguousCorroborationIsNotFlaggedUnmaintained(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "limited", Name: "RateLimitedApp", Repo: "ghcr.io/x/limited", Tag: "1.0.0", Digest: "sha256:l", Managed: true},
		{ID: "erred", Name: "ErroredApp", Repo: "ghcr.io/x/erred", Tag: "1.0.0", Digest: "sha256:e", Managed: true},
		{ID: "failed", Name: "FailedProbeApp", Repo: "ghcr.io/x/failed", Tag: "1.0.0", Digest: "sha256:f", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/limited": {tag: "1.0.0", dig: "sha256:l"},
		"ghcr.io/x/erred":   {tag: "1.0.0", dig: "sha256:e"},
		"ghcr.io/x/failed":  {tag: "1.0.0", dig: "sha256:f"},
	}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{
			"ratelimitedapp": "https://raw.githubusercontent.com/owner/limited/main/app.xml",
			"erroredapp":     "https://raw.githubusercontent.com/owner/erred/main/app.xml",
			"failedprobeapp": "https://raw.githubusercontent.com/owner/failed/main/app.xml",
		}
	}
	e.checkURL = func(_ context.Context, url string) int {
		switch url {
		case "https://raw.githubusercontent.com/owner/limited/main/app.xml":
			return 404
		case "https://github.com/owner/limited":
			return 429
		case "https://raw.githubusercontent.com/owner/erred/main/app.xml":
			return 404
		case "https://github.com/owner/erred":
			return 503
		case "https://raw.githubusercontent.com/owner/failed/main/app.xml":
			return 404
		case "https://github.com/owner/failed":
			return 0
		}
		t.Fatalf("unexpected checkURL call: %s", url)
		return 0
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	for id, label := range map[string]string{"limited": "rate-limited", "erred": "server-error", "failed": "failed-request"} {
		if r := st.rows[id]; r.Unmaintained {
			t.Fatalf("%s (%s): an inconclusive corroboration probe must not flag unmaintained, got reason %q", id, label, r.UnmaintainedReason)
		}
	}
}

// A suppression also silences the archived-repo and registry-gone signals,
// which have no other way out.
func TestSweepSuppressedUnmaintainedSilencesEveryTrigger(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "gone", Name: "GoneApp", Repo: "ghcr.io/x/gone-suppressed", Tag: "1.0.0", Digest: "sha256:g", Managed: true},
		{ID: "arch", Name: "ArchApp", Repo: "ghcr.io/x/arch-suppressed", Tag: "1.0.0", Digest: "sha256:a", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/gone-suppressed": {err: resolver.ErrRepoNotFound},
		"ghcr.io/x/arch-suppressed": {tag: "1.0.0", dig: "sha256:a"},
	}}
	st := &fakeStore{suppressed: map[string]bool{
		"ghcr.io/x/gone-suppressed": true,
		"ghcr.io/x/arch-suppressed": true,
	}}
	e := New(col, res, &fakeChangelog{deprecated: true}, st, time.Hour)
	e.templateURLs = func() map[string]string { return nil }
	e.checkURL = func(context.Context, string) int { return 200 }
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if g := st.rows["gone"]; g.Unmaintained {
		t.Fatalf("gone: suppressed registry-404 must not flag unmaintained, got reason %q", g.UnmaintainedReason)
	}
	if a := st.rows["arch"]; a.Unmaintained {
		t.Fatalf("arch: suppressed archived-repo must not flag unmaintained, got reason %q", a.UnmaintainedReason)
	}
}

// An archived source repo flags the container even while its image and
// template still exist.
func TestSweepFlagsArchivedRepo(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "arch", Name: "ArchApp", Repo: "ghcr.io/x/arch", Tag: "1.0.0", Digest: "sha256:a", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/arch": {tag: "1.0.0", dig: "sha256:a"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{deprecated: true}, st, time.Hour)
	e.templateURLs = func() map[string]string { return nil }
	e.checkURL = func(context.Context, string) int { return 200 }
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if a := st.rows["arch"]; !a.Unmaintained || a.UnmaintainedReason != "Source repository archived" {
		t.Fatalf("arch: want unmaintained/archived, got %v/%q", a.Unmaintained, a.UnmaintainedReason)
	}
}

// The feed catches an app pulled from CA whose template file still exists.
func TestSweepFlagsUnmaintainedViaCAFeedAbsent(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "gone", Name: "GoneApp", Repo: "ghcr.io/x/gone3", Tag: "1.0.0", Digest: "sha256:g", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/gone3": {tag: "1.0.0", dig: "sha256:g"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"goneapp": "https://raw.githubusercontent.com/x/gone/main/app.xml"}
	}
	e.checkURL = func(context.Context, string) int { return 200 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: false}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if g := st.rows["gone"]; !g.Unmaintained || g.UnmaintainedReason != "Removed from Community Applications" {
		t.Fatalf("want unmaintained/removed-from-CA via feed, got %v/%q", g.Unmaintained, g.UnmaintainedReason)
	}
}

// A feed that still lists the app wins over a dead template URL left behind by
// a moved repo.
func TestSweepCAFeedOverridesStaleRawURLProxy(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "moved2", Name: "OpenHands", Repo: "docker.openhands.dev/openhands/openhands", Tag: "1.7", Digest: "sha256:o", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"docker.openhands.dev/openhands/openhands": {tag: "1.7", dig: "sha256:o"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"openhands": "https://raw.githubusercontent.com/junkerderprovinz/openhands/main/templates/openhands.xml"}
	}
	rawURLChecked := false
	e.checkURL = func(_ context.Context, url string) int {
		rawURLChecked = true
		return 404
	}
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: true}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if m := st.rows["moved2"]; m.Unmaintained {
		t.Fatalf("a stale raw template URL must not override a feed confirming the app is still listed, got reason %q", m.UnmaintainedReason)
	}
	if rawURLChecked {
		t.Error("the raw-URL probe must not run once the feed gave a conclusive answer")
	}
}

// An app from a hand-authored template never came from CA, so its absence from
// the feed means nothing.
func TestSweepPersonalTemplateIsNotFlaggedUnmaintained(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "own", Name: "MyOwnApp", Repo: "ghcr.io/x/ownapp", Tag: "1.0.0", Digest: "sha256:p", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/ownapp": {tag: "1.0.0", dig: "sha256:p"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"myownapp": ""}
	}
	e.checkURL = func(context.Context, string) int { return 404 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: false}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if g := st.rows["own"]; g.Unmaintained {
		t.Fatalf("personal-template app wrongly flagged unmaintained: %q", g.UnmaintainedReason)
	}
}

// A source override marks the app as the user's own, whose template URL may
// point into a private repo that 404s publicly.
func TestSweepSourceOverrideExemptsFromUnmaintained(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "mine", Name: "Stashify", Repo: "ghcr.io/x/stashify", Tag: "1.0.0", Digest: "sha256:s", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/stashify": {tag: "1.0.0", dig: "sha256:s"}}}
	st := &fakeStore{overrides: map[string]string{"ghcr.io/x/stashify": "https://github.com/x/stashify"}}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"stashify": "https://raw.githubusercontent.com/x/stashify/main/unraid/stashify.xml"}
	}
	e.checkURL = func(context.Context, string) int { return 404 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: false}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if g := st.rows["mine"]; g.Unmaintained {
		t.Fatalf("override-sourced app wrongly flagged unmaintained: %q", g.UnmaintainedReason)
	}
}

func TestSweepFlagsUnmaintainedViaCAFeedBlacklisted(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "bl", Name: "BadApp", Repo: "x/bad", Tag: "1.0.0", Digest: "sha256:b", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"x/bad": {tag: "1.0.0", dig: "sha256:b"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"badapp": "https://raw.githubusercontent.com/x/bad/main/badapp.xml"}
	}
	e.checkURL = func(context.Context, string) int { return 200 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: false, Note: "Repository no longer exists on dockerHub"}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	want := "Removed from Community Applications: Repository no longer exists on dockerHub"
	if b := st.rows["bl"]; !b.Unmaintained || b.UnmaintainedReason != want {
		t.Fatalf("want unmaintained/%q, got %v/%q", want, b.Unmaintained, b.UnmaintainedReason)
	}
}

// A demoted app is still listed and updated, so it is not unmaintained.
func TestSweepFlagsCADeprecatedWithoutUnmaintained(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "dep", Name: "HandBrake", Repo: "coppit/handbrake", Tag: "1.0.0", Digest: "sha256:h", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"coppit/handbrake": {tag: "1.0.0", dig: "sha256:h"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string {
		return map[string]string{"handbrake": "https://raw.githubusercontent.com/coppit/handbrake/main/handbrake.xml"}
	}
	e.checkURL = func(context.Context, string) int { return 200 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: true, Deprecated: true, Note: "A better supported and more up to date app is available from DJoss"}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	d := st.rows["dep"]
	if d.Unmaintained {
		t.Fatalf("a deprecated but listed app must not be Unmaintained, got reason %q", d.UnmaintainedReason)
	}
	if !d.CADeprecated || d.CADeprecatedNote == "" {
		t.Fatalf("want CADeprecated with a note, got %v/%q", d.CADeprecated, d.CADeprecatedNote)
	}
}

func TestSweepCAFeedInconclusiveLeavesContainerAlone(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "inc", Name: "MaybeApp", Repo: "x/maybe", Tag: "1.0.0", Digest: "sha256:m", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"x/maybe": {tag: "1.0.0", dig: "sha256:m"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string { return nil }
	e.checkURL = func(context.Context, string) int { return 200 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) { return fakeCAFeed{ok: false}, nil }
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if m := st.rows["inc"]; m.Unmaintained || m.CADeprecated {
		t.Fatalf("inconclusive feed lookup must not flag anything, got unmaintained=%v ca_deprecated=%v", m.Unmaintained, m.CADeprecated)
	}
}

// An archived repo wins over a demotion in the feed.
func TestSweepCAFeedNotConsultedWhenAlreadyFlagged(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "arch2", Name: "ArchApp2", Repo: "ghcr.io/x/arch2", Tag: "1.0.0", Digest: "sha256:a", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/arch2": {tag: "1.0.0", dig: "sha256:a"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{deprecated: true}, st, time.Hour)
	e.templateURLs = func() map[string]string { return nil }
	e.checkURL = func(context.Context, string) int { return 200 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) {
		return fakeCAFeed{ok: true, result: cafeed.Result{Listed: true, Deprecated: true}}, nil
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if a := st.rows["arch2"]; !a.Unmaintained || a.UnmaintainedReason != "Source repository archived" {
		t.Fatalf("archived signal should win, got %v/%q", a.Unmaintained, a.UnmaintainedReason)
	}
	if st.rows["arch2"].CADeprecated {
		t.Fatal("CADeprecated must not be set once Unmaintained is already true via another signal")
	}
}

func TestSweepCAFeedLoadErrorDoesNotPanic(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "ok", Name: "FineApp", Repo: "ghcr.io/x/fine2", Tag: "1.0.0", Digest: "sha256:f", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/fine2": {tag: "1.0.0", dig: "sha256:f"}}}
	st := &fakeStore{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	e.templateURLs = func() map[string]string { return nil }
	e.checkURL = func(context.Context, string) int { return 200 }
	e.caFeed = func(context.Context) (caFeedLookuper, error) { return nil, errors.New("network down") }
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if f := st.rows["ok"]; f.Unmaintained || f.CADeprecated {
		t.Fatalf("a caFeed load error must leave the container unflagged, got unmaintained=%v ca_deprecated=%v", f.Unmaintained, f.CADeprecated)
	}
}

func TestUnmaintainedNotifiesOnce(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "rm", Name: "RemovedApp", Repo: "ghcr.io/x/removed", Tag: "1.0.0", Digest: "sha256:r", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/removed": {tag: "1.0.0", dig: "sha256:r"}}}
	st := &fakeStore{rows: map[string]model.UpdateStatus{
		"rm": {Container: model.Container{ID: "rm", Name: "RemovedApp", Digest: "sha256:r"}, Kind: model.KindNone, RunningVersion: "1.0.0"},
	}}
	nf := &fakeNotifier{}
	e := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf)
	e.templateURLs = func() map[string]string { return map[string]string{"removedapp": "http://ca/removed.xml"} }
	e.checkURL = func(context.Context, string) int { return 404 }

	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep 1: %v", err)
	}
	if nf.count() != 1 {
		t.Fatalf("first flip must notify once, got %d", nf.count())
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep 2: %v", err)
	}
	if nf.count() != 1 {
		t.Fatalf("already-unmaintained must not re-notify, got %d", nf.count())
	}
}

func TestSweepClassifiesAndCapturesPerContainerErrors(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "a", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.2.0", Digest: "sha256:o", Source: "https://github.com/x/immich"},
		{ID: "b", Name: "redis", Repo: "docker.io/library/redis", Tag: "7.2.0", Digest: "sha256:r"},
		{ID: "c", Name: "caddy", Repo: "docker.io/library/caddy", Tag: "2.7.0", Digest: "sha256:c", Source: "https://github.com/caddyserver/caddy"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/immich":        {tag: "1.4.0", dig: "sha256:n"},
		"docker.io/library/redis": {err: errors.New("registry timeout")},
		"docker.io/library/caddy": {tag: "2.7.0", dig: "sha256:c"},
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
	// redis fails to resolve and never reaches the changelog step.
	if cl.called != 2 {
		t.Fatalf("changelog should be fetched for each resolved container, got %d", cl.called)
	}

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
}

// Breaking release notes raise even a low-risk digest move to critical.
func TestSweepEscalatesBreakingChangelogToCritical(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "im", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "latest", Digest: "sha256:old", Source: "https://github.com/x/immich"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/immich": {tag: "latest", dig: "sha256:new"},
	}}
	cl := &fakeChangelog{entries: []model.ReleaseEntry{
		{Tag: "v2.0.0", Body: "## Breaking change\nWe removed support for pgvecto.rs. You must migrate to VectorChord before updating."},
	}}
	st := &fakeStore{}
	e := New(col, res, cl, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := st.rows["im"]
	if row.Risk != model.RiskCritical {
		t.Fatalf("breaking changelog must escalate to critical, got risk=%s reason=%q", row.Risk, row.RiskReason)
	}
	if !strings.Contains(row.RiskReason, "breaking change") {
		t.Errorf("reason should explain the escalation, got %q", row.RiskReason)
	}
	if row.Kind != model.KindDigest {
		t.Errorf("kind = %s, want digest (escalation must not rewrite the kind)", row.Kind)
	}
}

func TestSweepBenignChangelogKeepsVersionRisk(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "a", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.2.0", Digest: "sha256:o", Source: "https://github.com/x/immich"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/immich": {tag: "1.3.0", dig: "sha256:n"},
	}}
	cl := &fakeChangelog{entries: []model.ReleaseEntry{
		{Tag: "v1.3.0", Body: "Quality of life improvements and another round of bug fixes."},
	}}
	st := &fakeStore{}
	e := New(col, res, cl, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := st.rows["a"]; row.Risk != model.RiskMedium {
		t.Fatalf("benign minor bump must stay medium, got %s (%q)", row.Risk, row.RiskReason)
	}
}

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

func TestSweepRollingTagUsesVersionDeltaForRisk(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "oc", Name: "opencloud", Repo: "ghcr.io/o/opencloud", Tag: "latest", Digest: "sha256:run", ImageVersion: "7.1.0"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
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
		dRun = "sha256:running"
		dNew = "sha256:newver"
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
			newestVerDigest: dNew,
			want:            "1.8.0",
		},
		{
			name:            "latest unchanged carries the remembered version forward",
			c:               model.Container{Tag: "latest", Digest: dRun},
			newestVerTag:    "1.9.0",
			newestVerDigest: dNew,
			prior:           model.UpdateStatus{Container: model.Container{Digest: dRun}, RunningVersion: "1.7.0"},
			hasPrior:        true,
			want:            "1.7.0",
		},
		{
			name:            "latest lagging a newer published tag is not mislabeled",
			c:               model.Container{Tag: "latest", Digest: dRun},
			newestVerTag:    "2.0.0",
			newestVerDigest: dNew,
			want:            "",
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
			newestVerTag:    "",
			newestVerDigest: "",
			want:            "",
		},
		{
			name:            "image label version shows immediately for an unproven latest",
			c:               model.Container{Tag: "latest", Digest: dRun, ImageVersion: "2.7.2"},
			newestVerTag:    "2.8.0",
			newestVerDigest: dNew,
			want:            "2.7.2",
		},
		{
			name:            "digest proof outranks the image label",
			c:               model.Container{Tag: "latest", Digest: dNew, ImageVersion: "1.0.0"},
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
			name:            "revision label is not shown as a version",
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

func TestSweepRemembersLatestVersion(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "oh", Name: "openhands", Repo: "ghcr.io/x/openhands", Tag: "latest", Digest: "sha256:v18"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
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

func TestSweepTransientFailureKeepsPriorVerdict(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "im", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "latest", Digest: "sha256:d"},
	}}
	st := &fakeStore{}
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
		// A pin that also names a tag still runs the digest.
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
		"l":  "no registry digest (built or loaded locally), not checked against a registry",
		"p":  "pinned by digest, updates only when the pin changes",
		"tp": "pinned by digest, updates only when the pin changes",
		"i":  "referenced by image ID, no tag to compare",
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
	for _, id := range []string{"l", "p", "tp", "i"} {
		if st.rows[id].Changelog == nil {
			t.Errorf("%s: expected a changelog resolved without an upstream, got none", id)
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

func TestSweepPrunesOrphanedRows(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "live", Name: "live", Repo: "docker.io/library/app", Tag: "1.0", Managed: true},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"docker.io/library/app": {tag: "1.0", dig: "sha256:a", verTag: "1.0", verDig: "sha256:a"},
	}}
	st := &fakeStore{rows: map[string]model.UpdateStatus{
		"live":   {Container: model.Container{ID: "live", Name: "live"}, Kind: model.KindNone},
		"orphan": {Container: model.Container{ID: "orphan", Name: "gone"}, Kind: model.KindNone},
	}}
	e := New(col, res, &fakeChangelog{}, st, time.Hour)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if _, ok := st.rows["orphan"]; ok {
		t.Error("orphaned row (container no longer present) was not pruned")
	}
	if _, ok := st.rows["live"]; !ok {
		t.Error("live container row was wrongly pruned")
	}
}
