package model

import "testing"

func TestRiskLevelOrder(t *testing.T) {
	if !RiskHigh.MoreSevere(RiskMedium) || !RiskMedium.MoreSevere(RiskLow) || RiskLow.MoreSevere(RiskHigh) {
		t.Fatal("severity ordering wrong")
	}
	if RiskUnknown.MoreSevere(RiskLow) {
		t.Fatal("unknown must not outrank a real level")
	}
	// A changelog escalation to critical must not lose to the version-delta verdict.
	if !RiskCritical.MoreSevere(RiskHigh) || !RiskCritical.MoreSevere(RiskLow) || RiskHigh.MoreSevere(RiskCritical) {
		t.Fatal("critical must outrank every other level")
	}
}

func TestUpdateStatusHasUpdate(t *testing.T) {
	s := UpdateStatus{Kind: KindNone}
	if s.HasUpdate() {
		t.Fatal("KindNone must not be an update")
	}
	s.Kind = KindMinor
	if !s.HasUpdate() {
		t.Fatal("KindMinor must be an update")
	}
}

func TestSameVersionIgnoresPrefixAndBuildMetadata(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"10.3.5", "v10.3.5", true},
		{"V10.3.5", "10.3.5", true},
		{"release-10.3.5", "v10.3.5", true},
		{"10.3.5+build.7", "10.3.5", true},
		{" 10.3.5", "10.3.5", true},
		{"latest", "latest", true},
		{"10.3.5", "10.3.6", false},
		{"10.3.5-rc1", "10.3.5", false},
		{"10.3.5-ls12", "10.3.5-ls13", false},
		{"version", "ersion", false},
		{"latest", "v", false},
	}
	for _, c := range cases {
		if got := SameVersion(c.a, c.b); got != c.want {
			t.Errorf("SameVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestUpdateStatusLink(t *testing.T) {
	tests := []struct {
		name string
		st   UpdateStatus
		want string
	}{
		{"changelog file links the file itself",
			UpdateStatus{Changelog: &Changelog{Provider: "file", URL: "https://raw.githubusercontent.com/o/r/main/CHANGELOG.md"}},
			"https://raw.githubusercontent.com/o/r/main/CHANGELOG.md"},
		{"releases link to the repo root",
			UpdateStatus{Changelog: &Changelog{Provider: "github", URL: "https://github.com/o/r/releases"}},
			"https://github.com/o/r"},
		{"no changelog falls back to the source label",
			UpdateStatus{Container: Container{Source: "https://github.com/o/r.git"}},
			"https://github.com/o/r"},
	}
	for _, tt := range tests {
		if got := tt.st.Link(); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestUpdateStatusTarget(t *testing.T) {
	tests := []struct {
		name string
		st   UpdateStatus
		want string
	}{
		{"a pinned tag moves to the newest tag",
			UpdateStatus{Container: Container{Tag: "1.0"}, NewestTag: "1.2"}, "1.2"},
		{"a rolling move without a known version names none",
			UpdateStatus{Container: Container{Tag: "latest"}, NewestTag: "latest",
				Changelog: &Changelog{FromTag: "latest", ToTag: "latest", Recent: true, Entries: []ReleaseEntry{{Tag: "v6.1.0"}}}}, ""},
		{"a rolling move names the vouched version as the release spells it",
			UpdateStatus{Container: Container{Tag: "latest"}, NewestTag: "latest",
				Changelog: &Changelog{FromTag: "2.2.1", ToTag: "2.3.0", Entries: []ReleaseEntry{{Tag: "v2.3.0"}}}}, "v2.3.0"},
		{"a version without a release keeps its own spelling",
			UpdateStatus{Container: Container{Tag: "latest"}, NewestTag: "latest",
				Changelog: &Changelog{FromTag: "2.2.1", ToTag: "2.3.0", Recent: true, Entries: []ReleaseEntry{{Tag: "v2.2.9"}}}}, "2.3.0"},
		{"a rebuilt pinned tag names none",
			UpdateStatus{Container: Container{Tag: "6.1.0"}, NewestTag: "6.1.0",
				Changelog: &Changelog{FromTag: "6.1.0", ToTag: "6.1.0", Entries: []ReleaseEntry{{Tag: "v6.1.0"}}}}, ""},
	}
	for _, tt := range tests {
		if got := tt.st.Target(); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAVulnerabilityNoteSaysWhatAnUpdateCloses(t *testing.T) {
	cases := []struct {
		name   string
		v      *Vulns
		update bool
		want   string
	}{
		{"off", nil, true, ""},
		{"nothing known", &Vulns{Scope: VulnScopeImage}, true, ""},
		{"no update", &Vulns{Scope: VulnScopeImage, Critical: 2, High: 5}, false, "2 critical and 5 high known vulnerabilities"},
		{"closes some", &Vulns{Scope: VulnScopeImage, Critical: 2, High: 5, NewestKnown: true, FixedCritical: 1, FixedHigh: 3}, true,
			"2 critical and 5 high known vulnerabilities, the update closes 1 critical and 3 high"},
		{"closes only high", &Vulns{Scope: VulnScopeImage, Critical: 2, High: 5, NewestKnown: true, FixedHigh: 3}, true,
			"2 critical and 5 high known vulnerabilities, the update closes 3 high"},
		{"closes none", &Vulns{Scope: VulnScopeImage, Critical: 1, NewestKnown: true}, true,
			"1 critical known vulnerability, the update does not close it"},
		{"closes all", &Vulns{Scope: VulnScopeImage, High: 2, NewestKnown: true, FixedHigh: 2}, true,
			"2 high known vulnerabilities, the update closes all of them"},
		{"newest unknown", &Vulns{Scope: VulnScopeImage, High: 10}, true,
			"10 high known vulnerabilities, unknown what the update closes"},
	}
	for _, c := range cases {
		if got := c.v.Note(c.update); got != c.want {
			t.Errorf("%s: Note = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTheStatusLineSaysWhyNothingWasChecked(t *testing.T) {
	cases := map[string]struct {
		st   UpdateStatus
		want string
	}{
		"off":         {UpdateStatus{}, ""},
		"clean":       {UpdateStatus{Vulns: &Vulns{Scope: VulnScopeImage}}, "no known critical or high vulnerabilities"},
		"no packages": {UpdateStatus{Vulns: &Vulns{Reason: VulnNoPackages}}, "vulnerabilities unknown, no readable package list"},
		"failed":      {UpdateStatus{Vulns: &Vulns{Reason: VulnLookupFailed}}, "vulnerability lookup failed, ShipLog tries again at the next check"},
		"update": {UpdateStatus{Kind: KindPatch, Vulns: &Vulns{Scope: VulnScopeSystem, Critical: 1, NewestKnown: true, FixedCritical: 1}},
			"1 critical known vulnerability, the update closes it (system packages only)"},
	}
	for name, c := range cases {
		if got := c.st.VulnNote(); got != c.want {
			t.Errorf("%s: VulnNote = %q, want %q", name, got, c.want)
		}
	}
}

func TestAReasonReadsAsBeforeInEnglish(t *testing.T) {
	cases := []struct {
		r    Reason
		want string
	}{
		{Reason{Code: ReasonDigest}, "same tag, new image digest"},
		{Reason{Code: ReasonCurrent}, "up to date"},
		{Reason{Code: ReasonNonSemver}, "non-semver tags, cannot compare automatically"},
		{Reason{Code: ReasonMajor, Args: []string{"1", "2"}}, "major version bump 1.x → 2.x (review breaking changes)"},
		{Reason{Code: ReasonMinor, Args: []string{"1.2", "1.3"}}, "minor version bump 1.2 → 1.3 (new features, review the changelog)"},
		{Reason{Code: ReasonPatch, Args: []string{"1.2.3", "1.2.4"}}, "patch version bump 1.2.3 → 1.2.4 (fixes only, low risk)"},
		{Reason{Code: ReasonPinnedNewer, Args: []string{"1.2", "1.4"}}, "pinned to 1.2, newer version 1.4 available, change the tag to update"},
		{Reason{Code: ReasonBreakingIn, Args: []string{"v2.0.0", "breaking change"}}, `breaking change flagged in v2.0.0 release notes ("breaking change"), review before updating`},
		{Reason{Code: ReasonBreaking, Args: []string{"migration required"}}, `breaking change flagged in the release notes ("migration required"), review before updating`},
		{Reason{Code: ReasonDigestPinned}, "pinned by digest, updates only when the pin changes"},
		{Reason{Code: ReasonLocal}, "no registry digest (built or loaded locally), not checked against a registry"},
		{Reason{Code: ReasonImageID}, "referenced by image ID, no tag to compare"},
		{Reason{Code: ReasonRemoved}, "image removed from the registry"},
		{Reason{Code: ReasonLookupFailed}, "upstream lookup failed"},
	}
	if len(cases) != len(ReasonCodes) {
		t.Fatalf("%d cases for %d codes", len(cases), len(ReasonCodes))
	}
	for _, c := range cases {
		if got := c.r.String(); got != c.want {
			t.Errorf("%s = %q, want %q", c.r.Code, got, c.want)
		}
	}
}
