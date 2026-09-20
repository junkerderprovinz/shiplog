package risk

import (
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		cur, newest    string
		curDig, newDig string
		wantKind       model.Kind
		wantRisk       model.RiskLevel
	}{
		{"1.2.3", "1.2.3", "sha256:a", "sha256:a", model.KindNone, model.RiskNone},
		{"latest", "latest", "sha256:a", "sha256:b", model.KindDigest, model.RiskLow},
		{"1.2.3", "1.2.4", "", "", model.KindPatch, model.RiskLow},
		{"1.2.3", "1.3.0", "", "", model.KindMinor, model.RiskMedium},
		{"1.2.3", "2.0.0", "", "", model.KindMajor, model.RiskHigh},
		{"v1.2.3", "v1.4.0", "", "", model.KindMinor, model.RiskMedium},
		{"stable", "stable", "sha256:a", "sha256:a", model.KindNone, model.RiskNone},
		{"weird-tag", "other-tag", "", "", model.KindUnknown, model.RiskUnknown},
		// Missing components count as 0.
		{"1.2", "1.2.1", "", "", model.KindPatch, model.RiskLow},
		{"1.2", "1.3", "", "", model.KindMinor, model.RiskMedium},
		{"1", "2", "", "", model.KindMajor, model.RiskHigh},
		// Pre-release and build suffixes are ignored.
		{"1.2.3-rc1", "1.2.4", "", "", model.KindPatch, model.RiskLow},
		{"1.2.3+build5", "1.3.0", "", "", model.KindMinor, model.RiskMedium},
		{"1.2.3", "1.2.3", "", "", model.KindNone, model.RiskNone},
		{"v1.2.3", "1.2.3-rc1", "", "", model.KindNone, model.RiskNone},
		// A lower newest version counts as up to date.
		{"1.2.4", "1.2.3", "", "", model.KindNone, model.RiskNone},
		{"2.0.0", "1.9.9", "", "", model.KindNone, model.RiskNone},
		{"1.2.3", "nightly", "", "", model.KindUnknown, model.RiskUnknown},
		{"nightly", "1.2.3", "", "", model.KindUnknown, model.RiskUnknown},
		{"latest", "latest", "", "", model.KindNone, model.RiskNone},
		// One empty digest cannot prove a move.
		{"latest", "latest", "sha256:a", "", model.KindNone, model.RiskNone},
		{"1.2.3", "1.2.3", "sha256:a", "sha256:b", model.KindDigest, model.RiskLow},
	}
	for _, c := range cases {
		k, r, reason := Classify(c.cur, c.newest, c.curDig, c.newDig)
		if k != c.wantKind || r != c.wantRisk {
			t.Errorf("Classify(%q,%q)=%s/%s want %s/%s", c.cur, c.newest, k, r, c.wantKind, c.wantRisk)
		}
		if reason == "" {
			t.Errorf("Classify(%q,%q) empty reason", c.cur, c.newest)
		}
	}
}
