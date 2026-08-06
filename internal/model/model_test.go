package model

import "testing"

func TestRiskLevelOrder(t *testing.T) {
	if !RiskHigh.MoreSevere(RiskMedium) || !RiskMedium.MoreSevere(RiskLow) || RiskLow.MoreSevere(RiskHigh) {
		t.Fatal("severity ordering wrong")
	}
	if RiskUnknown.MoreSevere(RiskLow) {
		t.Fatal("unknown must not outrank a real level")
	}
	// Critical is the top of the ladder: it outranks every other level, so a
	// changelog-driven escalation never gets undercut by the version-delta verdict.
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
