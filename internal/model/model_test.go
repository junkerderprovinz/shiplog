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
