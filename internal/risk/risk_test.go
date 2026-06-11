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
		// --- prompt-supplied cases ---
		{"1.2.3", "1.2.3", "sha256:a", "sha256:a", model.KindNone, model.RiskNone},
		{"latest", "latest", "sha256:a", "sha256:b", model.KindDigest, model.RiskLow},
		{"1.2.3", "1.2.4", "", "", model.KindPatch, model.RiskLow},
		{"1.2.3", "1.3.0", "", "", model.KindMinor, model.RiskMedium},
		{"1.2.3", "2.0.0", "", "", model.KindMajor, model.RiskHigh},
		{"v1.2.3", "v1.4.0", "", "", model.KindMinor, model.RiskMedium},
		{"stable", "stable", "sha256:a", "sha256:a", model.KindNone, model.RiskNone},
		{"weird-tag", "other-tag", "", "", model.KindUnknown, model.RiskUnknown},

		// --- additional hardening cases ---
		// two-component semver: 1.2 == 1.2.0, bumping to 1.2.1 is a patch
		{"1.2", "1.2.1", "", "", model.KindPatch, model.RiskLow},
		// two-component semver minor bump: 1.2 (==1.2.0) -> 1.3 (==1.3.0)
		{"1.2", "1.3", "", "", model.KindMinor, model.RiskMedium},
		// one-component semver major bump: 1 (==1.0.0) -> 2 (==2.0.0)
		{"1", "2", "", "", model.KindMajor, model.RiskHigh},
		// pre-release suffix stripped: 1.2.3-rc1 == 1.2.3 -> 1.2.4 is a patch
		{"1.2.3-rc1", "1.2.4", "", "", model.KindPatch, model.RiskLow},
		// build-metadata suffix stripped: 1.2.3+build5 == 1.2.3
		{"1.2.3+build5", "1.3.0", "", "", model.KindMinor, model.RiskMedium},
		// equal semver tags, no digests -> nothing to report
		{"1.2.3", "1.2.3", "", "", model.KindNone, model.RiskNone},
		// equal semver tags via different forms (v-prefix + prerelease) -> KindNone
		{"v1.2.3", "1.2.3-rc1", "", "", model.KindNone, model.RiskNone},
		// newest is LOWER than current -> treated as up to date
		{"1.2.4", "1.2.3", "", "", model.KindNone, model.RiskNone},
		// downgrade across major -> still KindNone
		{"2.0.0", "1.9.9", "", "", model.KindNone, model.RiskNone},
		// one side semver, other side not -> unknown
		{"1.2.3", "nightly", "", "", model.KindUnknown, model.RiskUnknown},
		{"nightly", "1.2.3", "", "", model.KindUnknown, model.RiskUnknown},
		// same non-semver tag, no digests -> nothing changed
		{"latest", "latest", "", "", model.KindNone, model.RiskNone},
		// same non-semver tag, one digest empty -> can't prove a move
		{"latest", "latest", "sha256:a", "", model.KindNone, model.RiskNone},
		// same semver tag but digest moved -> digest update
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
