package engine

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// fakeVulns answers with the report kept for the container and records the
// newest digest each check was given.
type fakeVulns struct {
	mu     sync.Mutex
	report map[string]model.Vulns
	newest map[string]string
}

func (f *fakeVulns) Check(_ context.Context, c model.Container, newestDigest string) *model.Vulns {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.newest == nil {
		f.newest = map[string]string{}
	}
	f.newest[c.ID] = newestDigest
	v := f.report[c.ID]
	return &v
}

func sweepWithVulns(t *testing.T, containers []model.Container, res fakeResolver, cl *fakeChangelog, v *fakeVulns) map[string]model.UpdateStatus {
	t.Helper()
	st := &fakeStore{}
	e := New(fakeCollector{list: containers}, res, cl, st, time.Hour)
	if v != nil {
		e.WithVulns(v)
	}
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st.rows
}

func TestEveryContainerCarriesItsKnownVulnerabilities(t *testing.T) {
	v := &fakeVulns{report: map[string]model.Vulns{
		"upd":    {Scope: model.VulnScopeImage, Critical: 1, High: 3},
		"cur":    {Scope: model.VulnScopeImage, High: 2},
		"pinned": {Scope: model.VulnScopeSystem, Critical: 4},
	}}
	rows := sweepWithVulns(t, []model.Container{
		{ID: "upd", Name: "app", Repo: "ghcr.io/x/app", Tag: "1.0.0", Digest: "sha256:a1"},
		{ID: "cur", Name: "cur", Repo: "ghcr.io/x/cur", Tag: "latest", Digest: "sha256:c1"},
		{ID: "pinned", Name: "pinned", Repo: "ghcr.io/x/pin", PinnedDigest: "sha256:p1", Digest: "sha256:p1"},
	}, fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/app": {tag: "1.0.0", dig: "sha256:a2"},
		"ghcr.io/x/cur": {tag: "latest", dig: "sha256:c1"},
	}}, &fakeChangelog{}, v)

	if got := rows["upd"].Vulns; got == nil || got.Critical != 1 || got.High != 3 {
		t.Errorf("upd: vulns = %+v", got)
	}
	if v.newest["upd"] != "sha256:a2" {
		t.Errorf("upd: checked against %q, want the digest an update pulls", v.newest["upd"])
	}
	if got := rows["cur"].Vulns; got == nil || got.High != 2 || v.newest["cur"] != "" {
		t.Errorf("cur: vulns = %+v against %q, want no newest image for an up to date container", got, v.newest["cur"])
	}
	if got := rows["pinned"].Vulns; got == nil || got.Critical != 4 || v.newest["pinned"] != "" {
		t.Errorf("pinned: vulns = %+v against %q", got, v.newest["pinned"])
	}
}

func TestASweepSaysHowManyImagesItChecked(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	v := &fakeVulns{report: map[string]model.Vulns{
		"a": {Scope: model.VulnScopeImage, Critical: 1},
		"b": {Scope: model.VulnScopeSystem, High: 2},
		"c": {Reason: model.VulnNoPackages},
	}}
	sweepWithVulns(t, []model.Container{
		{ID: "a", Name: "a", Repo: "ghcr.io/x/a", Tag: "1", Digest: "sha256:a"},
		{ID: "b", Name: "b", Repo: "ghcr.io/x/b", Tag: "1", Digest: "sha256:b"},
		{ID: "c", Name: "c", Repo: "ghcr.io/x/c", Tag: "1", Digest: "sha256:c"},
	}, fakeResolver{byRepo: map[string]resolveResult{
		"ghcr.io/x/a": {tag: "1", dig: "sha256:a"}, "ghcr.io/x/b": {tag: "1", dig: "sha256:b"}, "ghcr.io/x/c": {tag: "1", dig: "sha256:c"},
	}}, &fakeChangelog{}, v)
	want := "vulnerability check: 3 containers, 1 by the image's package list, 1 by the system packages, 1 without a package list, 0 failed"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("log = %q, want %q", buf.String(), want)
	}
}

func TestARegistryHiccupStillChecksTheRunningImage(t *testing.T) {
	st := &fakeStore{rows: map[string]model.UpdateStatus{"a": {
		Container: model.Container{ID: "a", Name: "app", Repo: "ghcr.io/x/app", Tag: "1.0.0", Digest: "sha256:a1"},
		Kind:      model.KindDigest, Risk: model.RiskLow, NewestDigest: "sha256:a2", Vulns: &model.Vulns{Scope: model.VulnScopeImage, High: 1},
	}}}
	v := &fakeVulns{report: map[string]model.Vulns{"a": {Scope: model.VulnScopeImage, Critical: 3}}}
	e := New(fakeCollector{list: []model.Container{{ID: "a", Name: "app", Repo: "ghcr.io/x/app", Tag: "1.0.0", Digest: "sha256:a1"}}},
		fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/app": {err: errors.New("429")}}}, &fakeChangelog{}, st, time.Hour).WithVulns(v)
	if err := e.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := st.rows["a"]; got.Vulns == nil || got.Vulns.Critical != 3 || v.newest["a"] != "sha256:a2" || got.Kind != model.KindDigest {
		t.Errorf("row = %+v against %q, want the kept verdict with a fresh check", got, v.newest["a"])
	}
}

func TestWithoutTheCheckNoContainerCarriesVulnerabilities(t *testing.T) {
	rows := sweepWithVulns(t, []model.Container{
		{ID: "upd", Name: "app", Repo: "ghcr.io/x/app", Tag: "1.0.0", Digest: "sha256:a1"},
	}, fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/app": {tag: "1.0.0", dig: "sha256:a2"}}}, &fakeChangelog{}, nil)
	if rows["upd"].Vulns != nil {
		t.Errorf("vulns = %+v, want none while the check is off", rows["upd"].Vulns)
	}
}

func TestAnUpdateThatClosesACriticalVulnerabilityRanksOneStepHigher(t *testing.T) {
	// A rolling tag that serves a newer version shows the real jump.
	cases := []struct {
		name    string
		tag     string
		running string
		res     resolveResult
		want    model.RiskLevel
	}{
		{"patch", "latest", "1.0.0", resolveResult{tag: "latest", dig: "sha256:a2", verTag: "1.0.1", verDig: "sha256:a2"}, model.RiskMedium},
		{"minor", "latest", "1.0.0", resolveResult{tag: "latest", dig: "sha256:a2", verTag: "1.1.0", verDig: "sha256:a2"}, model.RiskHigh},
		{"major", "latest", "1.0.0", resolveResult{tag: "latest", dig: "sha256:a2", verTag: "2.0.0", verDig: "sha256:a2"}, model.RiskHigh},
		{"rebuild", "1.0.0", "", resolveResult{tag: "1.0.0", dig: "sha256:a2"}, model.RiskMedium},
	}
	for _, c := range cases {
		v := &fakeVulns{report: map[string]model.Vulns{"a": {Scope: model.VulnScopeImage, Critical: 2, NewestKnown: true, FixedCritical: 1}}}
		rows := sweepWithVulns(t, []model.Container{
			{ID: "a", Name: "app", Repo: "ghcr.io/x/app", Tag: c.tag, ImageVersion: c.running, Digest: "sha256:a1"},
		}, fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/app": c.res}}, &fakeChangelog{}, v)
		row := rows["a"]
		if !row.HasUpdate() {
			t.Fatalf("%s: no update in %+v", c.name, row)
		}
		if row.Risk != c.want {
			t.Errorf("%s: risk = %s, want %s", c.name, row.Risk, c.want)
		}
		// The window says why in the viewer's language, so the English reason
		// stays as the version delta gave it.
		if row.VulnRaised != (c.name != "major") || strings.Contains(row.RiskReason, "vulnerab") {
			t.Errorf("%s: raised = %v, reason = %q", c.name, row.VulnRaised, row.RiskReason)
		}
	}
}

func TestAFixThatIsNotKnownLeavesTheRiskAlone(t *testing.T) {
	for name, report := range map[string]model.Vulns{
		"newest unknown":   {Scope: model.VulnScopeImage, Critical: 2},
		"only high closed": {Scope: model.VulnScopeImage, Critical: 2, High: 1, NewestKnown: true, FixedHigh: 1},
	} {
		v := &fakeVulns{report: map[string]model.Vulns{"a": report}}
		rows := sweepWithVulns(t, []model.Container{
			{ID: "a", Name: "app", Repo: "ghcr.io/x/app", Tag: "1.0.0", Digest: "sha256:a1"},
		}, fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/app": {tag: "1.0.1", dig: "sha256:a2"}}}, &fakeChangelog{}, v)
		if row := rows["a"]; row.Risk != model.RiskLow || row.VulnRaised {
			t.Errorf("%s: risk = %s (%q), want low", name, row.Risk, row.RiskReason)
		}
	}
}

func TestBreakingNotesStillOutrankAClosedVulnerability(t *testing.T) {
	v := &fakeVulns{report: map[string]model.Vulns{"a": {Scope: model.VulnScopeImage, Critical: 1, NewestKnown: true, FixedCritical: 1}}}
	cl := &fakeChangelog{entries: []model.ReleaseEntry{{Tag: "1.0.1", Body: "Breaking change: you must migrate the database first."}}}
	rows := sweepWithVulns(t, []model.Container{
		{ID: "a", Name: "app", Repo: "ghcr.io/x/app", Tag: "1.0.0", Digest: "sha256:a1"},
	}, fakeResolver{byRepo: map[string]resolveResult{"ghcr.io/x/app": {tag: "1.0.1", dig: "sha256:a2"}}}, cl, v)
	if row := rows["a"]; row.Risk != model.RiskCritical {
		t.Errorf("risk = %s, want critical from the notes", row.Risk)
	}
}
