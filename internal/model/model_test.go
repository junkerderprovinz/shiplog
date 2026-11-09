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
