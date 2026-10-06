package engine

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		c    model.Container
		// running is what decideRunningVersion believed; the rest is what the
		// resolver returned for the container's own tag and for the highest
		// version tag in the registry.
		running, newestTag, newestDigest, verTag, verDigest string

		wantKind   model.Kind
		wantRisk   model.RiskLevel
		wantNewer  string
		wantNewest string
		wantTo     string
	}{
		{
			name: "latest unchanged, a higher version tag from another channel",
			c:    model.Container{Tag: "latest", Digest: "sha256:f247"}, running: "4.0.20.3014-ls326",
			newestTag: "latest", newestDigest: "sha256:f247", verTag: "5.14", verDigest: "sha256:v5dev",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "4.0.20.3014-ls326",
		},
		{
			name: "latest unchanged, an old date tag sorts highest",
			c:    model.Container{Tag: "latest", Digest: "sha256:bfcd"}, running: "v2.18.2-ls246",
			newestTag: "latest", newestDigest: "sha256:bfcd", verTag: "2021.12.16", verDigest: "sha256:old2021",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "v2.18.2-ls246",
		},
		{
			name: "latest unchanged while a newer version tag points elsewhere",
			c:    model.Container{Tag: "latest", Digest: "sha256:cur"}, running: "1.8.0",
			newestTag: "latest", newestDigest: "sha256:cur", verTag: "1.9.0", verDigest: "sha256:next",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "1.8.0",
		},
		{
			name: "latest unchanged and equal to the newest version tag",
			c:    model.Container{Tag: "latest", Digest: "sha256:cur"}, running: "1.8.0",
			newestTag: "latest", newestDigest: "sha256:cur", verTag: "1.8.0", verDigest: "sha256:cur",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "1.8.0",
		},
		{
			name:    "mirror pull matching a secondary digest",
			c:       model.Container{Tag: "latest", Digest: "sha256:mirror", Digests: []string{"sha256:mirror", "sha256:up"}},
			running: "3.0.0", newestTag: "latest", newestDigest: "sha256:up", verTag: "9.9.9", verDigest: "sha256:junk",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "3.0.0",
		},
		{
			name:      "named channel tag unchanged",
			c:         model.Container{Tag: "nightly", Digest: "sha256:n1"},
			newestTag: "nightly", newestDigest: "sha256:n1", verTag: "3.1.0", verDigest: "sha256:rel",
			wantKind: model.KindNone, wantRisk: model.RiskNone,
		},
		{
			name: "pinned 0.6.1 unchanged while 0.7.0 exists",
			c:    model.Container{Tag: "0.6.1", Digest: "sha256:7200"}, running: "0.6.1",
			newestTag: "0.7.0", newestDigest: "sha256:7200", verTag: "0.7.0",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantNewer: "0.7.0",
		},
		{
			name: "pinned tag current through a mirror digest",
			c:    model.Container{Tag: "1.2.0", Digest: "sha256:mirror", Digests: []string{"sha256:mirror", "sha256:up"}}, running: "1.2.0",
			newestTag: "2.0.0", newestDigest: "sha256:up",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantNewer: "2.0.0",
		},
		{
			name: "pinned tag with an unknown running digest",
			c:    model.Container{Tag: "1.2.0"}, running: "1.2.0",
			newestTag: "1.3.0", newestDigest: "sha256:t",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantNewer: "1.3.0",
		},
		{
			name: "pinned tag that is the newest",
			c:    model.Container{Tag: "0.7.0", Digest: "sha256:7200"}, running: "0.7.0",
			newestTag: "0.7.0", newestDigest: "sha256:7200", verTag: "0.7.0",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "0.7.0",
		},
		{
			// A pull of :16 fetches the rebuilt 16, never 17.
			name: "pinned major tag rebuilt while a newer major exists",
			c:    model.Container{Tag: "16", Digest: "sha256:old"}, running: "16",
			newestTag: "17.2.0", newestDigest: "sha256:rebuilt",
			wantKind: model.KindDigest, wantRisk: model.RiskLow, wantNewest: "16", wantTo: "16",
		},
		{
			name: "latest moved, the newest version tag is another image",
			c:    model.Container{Tag: "latest", Digest: "sha256:a07a"}, running: "2026.9.30-a9d990033",
			newestTag: "latest", newestDigest: "sha256:d4c2", verTag: "2025.8.1", verDigest: "sha256:junk",
			wantKind: model.KindDigest, wantRisk: model.RiskLow,
		},
		{
			name:      "nightly moved, no version tags at all",
			c:         model.Container{Tag: "nightly", Digest: "sha256:5287"},
			newestTag: "nightly", newestDigest: "sha256:new",
			wantKind: model.KindDigest, wantRisk: model.RiskLow,
		},
		{
			name: "latest moved to the newest version tag, a minor jump",
			c:    model.Container{Tag: "latest", Digest: "sha256:run"}, running: "7.1.0",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "7.2.0", verDigest: "sha256:new",
			wantKind: model.KindMinor, wantRisk: model.RiskMedium, wantTo: "7.2.0",
		},
		{
			name: "latest moved to the newest version tag, a patch jump",
			c:    model.Container{Tag: "latest", Digest: "sha256:run"}, running: "2026.9.0",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "2026.9.1", verDigest: "sha256:new",
			wantKind: model.KindPatch, wantRisk: model.RiskLow, wantTo: "2026.9.1",
		},
		{
			name: "latest moved to the newest version tag, a major jump",
			c:    model.Container{Tag: "latest", Digest: "sha256:run"}, running: "1.9.4",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "2.0.0", verDigest: "sha256:new",
			wantKind: model.KindMajor, wantRisk: model.RiskHigh, wantTo: "2.0.0",
		},
		{
			name: "latest moved, an old date tag sorts highest",
			c:    model.Container{Tag: "latest", Digest: "sha256:run"}, running: "4.0.20",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "2021.12.16", verDigest: "sha256:old2021",
			wantKind: model.KindDigest, wantRisk: model.RiskLow,
		},
		{
			name: "latest rebuilt at the running version",
			c:    model.Container{Tag: "latest", Digest: "sha256:run"}, running: "7.2.0",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "7.2.0", verDigest: "sha256:new",
			wantKind: model.KindDigest, wantRisk: model.RiskLow, wantTo: "7.2.0",
		},
		{
			name:      "latest moved, running version unknown",
			c:         model.Container{Tag: "latest", Digest: "sha256:run"},
			newestTag: "latest", newestDigest: "sha256:new", verTag: "3.4.5", verDigest: "sha256:new",
			wantKind: model.KindDigest, wantRisk: model.RiskLow, wantTo: "3.4.5",
		},
		{
			name: "an empty version digest never vouches for the version tag",
			c:    model.Container{Tag: "latest", Digest: "sha256:run"}, running: "1.0.0",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "5.0.0",
			wantKind: model.KindDigest, wantRisk: model.RiskLow,
		},
		{
			name: "unknown running digest on a rolling tag",
			c:    model.Container{Tag: "latest"}, running: "1.0.0",
			newestTag: "latest", newestDigest: "sha256:new", verTag: "1.1.0", verDigest: "sha256:new",
			wantKind: model.KindNone, wantRisk: model.RiskNone, wantTo: "1.0.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := classify(tc.c, tc.running, tc.newestTag, tc.newestDigest, tc.verTag, tc.verDigest)
			if v.kind != tc.wantKind || v.risk != tc.wantRisk {
				t.Errorf("kind/risk = %s/%s, want %s/%s (reason %q)", v.kind, v.risk, tc.wantKind, tc.wantRisk, v.reason)
			}
			if v.newer != tc.wantNewer {
				t.Errorf("newer = %q, want %q", v.newer, tc.wantNewer)
			}
			if v.newest != tc.wantNewest {
				t.Errorf("newest = %q, want %q", v.newest, tc.wantNewest)
			}
			if v.to != tc.wantTo {
				t.Errorf("changelog end = %q, want %q", v.to, tc.wantTo)
			}
		})
	}
}

func TestSweepUnchangedRollingTagsAreUpToDate(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "sonarr", Name: "sonarr", Repo: "lscr.io/linuxserver/sonarr", Tag: "latest", Digest: "sha256:f247", ImageVersion: "4.0.20.3014-ls326"},
		{ID: "tautulli", Name: "tautulli", Repo: "docker.io/linuxserver/tautulli", Tag: "latest", Digest: "sha256:bfcd", ImageVersion: "v2.18.2-ls246"},
		{ID: "nzbget", Name: "nzbget", Repo: "lscr.io/linuxserver/nzbget", Tag: "latest", Digest: "sha256:ac88", ImageVersion: "v26.3-ls265"},
		{ID: "own", Name: "ownapp", Repo: "git.example.net/x/ownapp", Tag: "latest", Digest: "sha256:9638", ImageVersion: "0.0.0-dev"},
		{ID: "searx", Name: "searxng", Repo: "docker.io/searxng/searxng", Tag: "latest", Digest: "sha256:a07a", ImageVersion: "2026.9.30-a9d990033"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"lscr.io/linuxserver/sonarr":     {tag: "latest", dig: "sha256:f247", verTag: "5.14", verDig: "sha256:v5dev"},
		"docker.io/linuxserver/tautulli": {tag: "latest", dig: "sha256:bfcd", verTag: "2021.12.16", verDig: "sha256:old"},
		"lscr.io/linuxserver/nzbget":     {tag: "latest", dig: "sha256:ac88", verTag: "2021.11.25", verDig: "sha256:old2"},
		"git.example.net/x/ownapp":       {tag: "latest", dig: "sha256:9638", verTag: "1.2.1", verDig: "sha256:other"},
		"docker.io/searxng/searxng":      {tag: "latest", dig: "sha256:d4c2", verTag: "2025.8.1", verDig: "sha256:junk"},
	}}
	st := &fakeStore{}
	if err := New(col, res, &fakeChangelog{}, st, time.Hour).Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"sonarr", "tautulli", "nzbget", "own"} {
		row := st.rows[id]
		if row.HasUpdate() || row.Risk != model.RiskNone || row.NewerVersion != "" {
			t.Errorf("%s: want plain up to date, got kind=%s risk=%s newer=%q (%s)", id, row.Kind, row.Risk, row.NewerVersion, row.RiskReason)
		}
		cl := row.Changelog
		if cl == nil {
			t.Fatalf("%s: an up-to-date container still gets the running version's notes", id)
		}
		if cl.FromTag != row.RunningVersion || cl.ToTag != row.RunningVersion {
			t.Errorf("%s: changelog span %q to %q, want the running version %q on both ends", id, cl.FromTag, cl.ToTag, row.RunningVersion)
		}
	}

	searx := st.rows["searx"]
	if searx.Kind != model.KindDigest || searx.Risk != model.RiskLow {
		t.Errorf("searxng: a real digest move stays an update, got kind=%s risk=%s", searx.Kind, searx.Risk)
	}
	if searx.Changelog == nil || searx.Changelog.ToTag != "latest" {
		t.Errorf("searxng: without a matching version tag the changelog asks for recent releases, got %+v", searx.Changelog)
	}
}

func TestSweepClearsAStaleVersionVerdict(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "sonarr", Name: "sonarr", Repo: "lscr.io/linuxserver/sonarr", Tag: "latest", Digest: "sha256:f247", ImageVersion: "4.0.20.3014-ls326"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"lscr.io/linuxserver/sonarr": {tag: "latest", dig: "sha256:f247", verTag: "5.14", verDig: "sha256:v5dev"},
	}}
	st := &fakeStore{rows: map[string]model.UpdateStatus{
		"sonarr": {
			Container:    model.Container{ID: "sonarr", Name: "sonarr", Digest: "sha256:f247"},
			NewestDigest: "sha256:f247", NewestTag: "latest",
			Kind: model.KindMajor, Risk: model.RiskHigh, RunningVersion: "4.0.20.3014-ls326",
		},
	}}
	nf := &fakeNotifier{}
	if err := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := st.rows["sonarr"]; row.HasUpdate() {
		t.Fatalf("a stale major must clear to up to date, got kind=%s", row.Kind)
	}
	if nf.count() != 0 {
		t.Fatalf("an up-to-date verdict must not notify, got %d", nf.count())
	}
}

func TestSweepPinnedTagNewerVersionIsAdvisory(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "wyo", Name: "wyoming-openai", Repo: "ghcr.io/roryeckel/wyoming_openai", Tag: "0.6.1", Digest: "sha256:7200"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/roryeckel/wyoming_openai": {tag: "0.7.0", dig: "sha256:7200", verTag: "0.7.0"},
	}}
	nf := &fakeNotifier{}
	st := &fakeStore{rows: map[string]model.UpdateStatus{
		"wyo": {Container: model.Container{ID: "wyo", Name: "wyoming-openai"}, Kind: model.KindNone, Risk: model.RiskNone},
	}}
	if err := New(col, res, &fakeChangelog{}, st, time.Hour).WithNotifier(nf).Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := st.rows["wyo"]
	if row.HasUpdate() || row.Risk != model.RiskNone {
		t.Fatalf("a pinned tag is not an update, got kind=%s risk=%s", row.Kind, row.Risk)
	}
	if row.NewerVersion != "0.7.0" {
		t.Fatalf("want an advisory for 0.7.0, got newer=%q", row.NewerVersion)
	}
	if row.RunningVersion != "0.6.1" {
		t.Errorf("running version = %q, want the pinned 0.6.1", row.RunningVersion)
	}
	if row.Changelog == nil || row.Changelog.FromTag != "0.6.1" || row.Changelog.ToTag != "0.7.0" {
		t.Errorf("the changelog covers what a tag change would bring (0.6.1 to 0.7.0), got %+v", row.Changelog)
	}
	if nf.count() != 0 {
		t.Errorf("an advisory must not notify, got %d", nf.count())
	}
}

func TestSweepRebuiltPinnedTagIsADigestUpdate(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "p", Name: "pinned", Repo: "ghcr.io/x/pinned", Tag: "1.2.0", Digest: "sha256:old"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/pinned": {tag: "1.4.0", dig: "sha256:rebuilt", verTag: "1.4.0"},
	}}
	st := &fakeStore{}
	if err := New(col, res, &fakeChangelog{}, st, time.Hour).Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := st.rows["p"]
	if row.Kind != model.KindDigest || row.NewestTag != "1.2.0" || row.NewerVersion != "" {
		t.Fatalf("a pull of the pinned tag fetches the rebuilt 1.2.0, got kind=%s newest=%q newer=%q", row.Kind, row.NewestTag, row.NewerVersion)
	}
}

func TestSweepMoveToNewestVersionUsesItForChangelog(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "oc", Name: "opencloud", Repo: "ghcr.io/o/opencloud", Tag: "latest", Digest: "sha256:run", ImageVersion: "7.1.0"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/o/opencloud": {tag: "latest", dig: "sha256:new", verTag: "7.2.0", verDig: "sha256:new"},
	}}
	st := &fakeStore{}
	if err := New(col, res, &fakeChangelog{}, st, time.Hour).Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := st.rows["oc"]
	if row.Kind != model.KindMinor {
		t.Fatalf("kind = %s, want minor", row.Kind)
	}
	if cl := row.Changelog; cl == nil || cl.FromTag != "7.1.0" || cl.ToTag != "7.2.0" {
		t.Fatalf("changelog span = %+v, want 7.1.0 to 7.2.0", cl)
	}
}

// With the production policy (level major, digest moves on) only images that
// really changed under their own tag are eligible.
func TestFleetEligibility(t *testing.T) {
	col := fakeCollector{list: []model.Container{
		{ID: "sonarr", Name: "sonarr", Repo: "r/sonarr", Tag: "latest", Digest: "sha256:s1", ImageVersion: "4.0.20.3014-ls326"},
		{ID: "tautulli", Name: "tautulli", Repo: "r/tautulli", Tag: "latest", Digest: "sha256:t1", ImageVersion: "v2.18.2-ls246"},
		{ID: "own", Name: "ownapp", Repo: "r/ownapp", Tag: "latest", Digest: "sha256:c1", ImageVersion: "0.0.0-dev"},
		{ID: "wyo", Name: "wyoming-openai", Repo: "r/wyoming", Tag: "0.6.1", Digest: "sha256:w1"},
		{ID: "redis", Name: "redis", Repo: "r/redis", Tag: "latest", Digest: "sha256:r1"},
		{ID: "searx", Name: "searxng", Repo: "r/searxng", Tag: "latest", Digest: "sha256:x-old", ImageVersion: "2026.9.30-a9d990033"},
		{ID: "mealie", Name: "mealie", Repo: "r/mealie", Tag: "nightly", Digest: "sha256:m-old"},
		{ID: "oc", Name: "opencloud", Repo: "r/opencloud", Tag: "latest", Digest: "sha256:o-old", ImageVersion: "7.1.0"},
		{ID: "gitea", Name: "gitea", Repo: "r/gitea", Tag: "latest", Digest: "sha256:g-old", ImageVersion: "1.24.0"},
	}}
	res := fakeResolver{byRepo: map[string]resolveResult{
		"r/sonarr":    {tag: "latest", dig: "sha256:s1", verTag: "5.14", verDig: "sha256:v5"},
		"r/tautulli":  {tag: "latest", dig: "sha256:t1", verTag: "2021.12.16", verDig: "sha256:d1"},
		"r/ownapp":    {tag: "latest", dig: "sha256:c1", verTag: "1.2.1", verDig: "sha256:o1"},
		"r/wyoming":   {tag: "0.7.0", dig: "sha256:w1", verTag: "0.7.0"},
		"r/redis":     {tag: "latest", dig: "sha256:r1"},
		"r/searxng":   {tag: "latest", dig: "sha256:x-new", verTag: "2025.8.1", verDig: "sha256:junk"},
		"r/mealie":    {tag: "nightly", dig: "sha256:m-new"},
		"r/opencloud": {tag: "latest", dig: "sha256:o-new", verTag: "7.2.0", verDig: "sha256:o-new"},
		"r/gitea":     {tag: "latest", dig: "sha256:g-new"},
	}}
	st := &fakeStore{}
	if err := New(col, res, &fakeChangelog{}, st, time.Hour).Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, _ := st.List()

	policy := autoupdate.Policy{Level: autoupdate.LevelMajor, Digest: true, ExcludeContainers: []string{"gitea"}}
	var eligible, skipped []string
	for _, row := range rows {
		if !autoupdate.Eligible(row, policy) {
			continue
		}
		if autoupdate.ContainerExcluded(row.Container.Name, policy.ExcludeContainers) {
			skipped = append(skipped, row.Container.Name)
			continue
		}
		eligible = append(eligible, row.Container.Name)
	}
	sort.Strings(eligible)
	if want := []string{"mealie", "opencloud", "searxng"}; !reflect.DeepEqual(eligible, want) {
		t.Errorf("eligible = %v, want %v", eligible, want)
	}
	if want := []string{"gitea"}; !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped = %v, want %v", skipped, want)
	}
	if row := st.rows["wyo"]; row.HasUpdate() || row.NewerVersion != "0.7.0" {
		t.Errorf("wyoming-openai should be an advisory for 0.7.0, got %+v", row)
	}
}
