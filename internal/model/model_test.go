package model

import "testing"

func TestRiskLevelOrder(t *testing.T) {
	if !RiskHigh.MoreSevere(RiskMedium) || !RiskMedium.MoreSevere(RiskLow) || RiskLow.MoreSevere(RiskHigh) {
		t.Fatal("severity ordering wrong")
	}
	if RiskUnknown.MoreSevere(RiskLow) {
		t.Fatal("unknown must not outrank a real level")
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
