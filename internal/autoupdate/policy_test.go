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
		{"patch under patch", st(model.KindPatch), Policy{Level: LevelPatch}, true},
		{"minor under patch", st(model.KindMinor), Policy{Level: LevelPatch}, false},
		{"minor under minor", st(model.KindMinor), Policy{Level: LevelMinor}, true},
		{"major under minor", st(model.KindMajor), Policy{Level: LevelMinor}, false},
		{"major under major", st(model.KindMajor), Policy{Level: LevelMajor}, true},
		{"digest off", st(model.KindDigest), Policy{Level: LevelMajor}, false},
		{"digest on", st(model.KindDigest), Policy{Level: LevelOff, Digest: true}, true},
		{"unknown never", st(model.KindUnknown), Policy{Level: LevelMajor, Digest: true}, false},
		{"none never", st(model.KindNone), Policy{Level: LevelMajor, Digest: true}, false},
		{"off nothing", st(model.KindPatch), Policy{Level: LevelOff}, false},
	}
	for _, c := range cases {
		if got := Eligible(c.st, c.p); got != c.want {
			t.Errorf("%s: Eligible = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseExcludeWords(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"breaking", []string{"breaking"}},
		{"breaking, migration required", []string{"breaking", "migration required"}},
		{" breaking ,, BREAKING CHANGE , ", []string{"breaking", "BREAKING CHANGE"}},
	}
	for _, c := range cases {
		got := ParseExcludeWords(c.in)
		if len(got) != len(c.want) {
			t.Errorf("ParseExcludeWords(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseExcludeWords(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestMatchedExcludeWord(t *testing.T) {
	words := []string{"breaking", "Migration Required"}
	cases := []struct {
		name string
		cl   *model.Changelog
		want string
	}{
		{"nil changelog", nil, ""},
		{"empty raw", &model.Changelog{Raw: ""}, ""},
		{"no match", &model.Changelog{Raw: "Bug fixes and performance improvements."}, ""},
		{"exact word", &model.Changelog{Raw: "This release includes a BREAKING change to the API."}, "breaking"},
		{"case-insensitive against mixed-case configured word", &model.Changelog{Raw: "please read the migration required section"}, "Migration Required"},
		{"substring within a larger word still counts", &model.Changelog{Raw: "a groundbreaking new feature"}, "breaking"},
		{"first configured word wins when both match", &model.Changelog{Raw: "breaking change, migration required"}, "breaking"},
	}
	for _, c := range cases {
		if got := MatchedExcludeWord(c.cl, words); got != c.want {
			t.Errorf("%s: MatchedExcludeWord = %q, want %q", c.name, got, c.want)
		}
	}
	if got := MatchedExcludeWord(&model.Changelog{Raw: "breaking"}, nil); got != "" {
		t.Errorf("no configured words must never match, got %q", got)
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
