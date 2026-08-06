package risk

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestScanBreaking(t *testing.T) {
	cases := []struct {
		name    string
		cl      *model.Changelog
		wantHit bool
		wantIn  string // substring the reason must contain when hit
	}{
		{"nil changelog", nil, false, ""},
		{"empty changelog", &model.Changelog{}, false, ""},
		{
			"benign notes stay quiet",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v1.2.0", Body: "Quality of life improvements and another round of bug fixes."},
			}},
			false, "",
		},
		{
			"breaking heading in the newest release",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v2.0.0", Body: "## Breaking change\nYou must migrate your data before starting."},
			}},
			true, "v2.0.0",
		},
		{
			"breaking note hidden in an intermediate release, not the newest",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v3.1.0", Body: "Bug fixes."},
				{Tag: "v3.0.0", Body: "We removed support for pgvecto.rs; migrate to VectorChord."},
				{Tag: "v2.9.0", Body: "Minor tweaks."},
			}},
			true, "v3.0.0",
		},
		{
			"case-insensitive match",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v2.0.0", Body: "ACTION REQUIRED: run the migration script."},
			}},
			true, "action required",
		},
		{
			"falls back to raw body when no entries",
			&model.Changelog{Raw: "This release is NOT backward compatible with older configs."},
			true, "not backward compatible",
		},
		{
			"plain feature words do not trip it",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v1.5.0", Body: "New dashboard, faster search, and a migration guide link in the docs."},
			}},
			false, "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, hit := ScanBreaking(c.cl)
			if hit != c.wantHit {
				t.Fatalf("ScanBreaking hit = %v, want %v (reason %q)", hit, c.wantHit, reason)
			}
			if hit {
				if reason == "" {
					t.Fatal("a hit must carry a non-empty reason")
				}
				if c.wantIn != "" && !strings.Contains(reason, c.wantIn) {
					t.Errorf("reason %q does not mention %q", reason, c.wantIn)
				}
			}
		})
	}
}
