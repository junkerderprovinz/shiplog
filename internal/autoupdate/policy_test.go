package autoupdate

import (
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func st(k model.Kind) model.UpdateStatus { return model.UpdateStatus{Kind: k} }

func TestEligible(t *testing.T) {
	cases := []struct {
		name string
		st   model.UpdateStatus
		p    Policy
		want bool
	}{
		{"patch under patch", st(model.KindPatch), Policy{LevelPatch, false}, true},
		{"minor under patch", st(model.KindMinor), Policy{LevelPatch, false}, false},
		{"minor under minor", st(model.KindMinor), Policy{LevelMinor, false}, true},
		{"major under minor", st(model.KindMajor), Policy{LevelMinor, false}, false},
		{"major under major", st(model.KindMajor), Policy{LevelMajor, false}, true},
		{"digest off", st(model.KindDigest), Policy{LevelMajor, false}, false},
		{"digest on", st(model.KindDigest), Policy{LevelOff, true}, true},
		{"unknown never", st(model.KindUnknown), Policy{LevelMajor, true}, false},
		{"none never", st(model.KindNone), Policy{LevelMajor, true}, false},
		{"off nothing", st(model.KindPatch), Policy{LevelOff, false}, false},
	}
	for _, c := range cases {
		if got := Eligible(c.st, c.p); got != c.want {
			t.Errorf("%s: Eligible = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	for s, want := range map[string]Level{
		"off": LevelOff, "patch": LevelPatch, "minor": LevelMinor, "major": LevelMajor,
		"bogus": LevelOff, "": LevelOff,
	} {
		if got := ParseLevel(s); got != want {
			t.Errorf("ParseLevel(%q) = %d, want %d", s, got, want)
		}
	}
}
