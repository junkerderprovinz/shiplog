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
		from    string // running version passed to ScanBreaking
		wantHit bool
		wantIn  string // substring the reason must contain when hit
	}{
		{"nil changelog", nil, "v1.0.0", false, ""},
		{"empty changelog", &model.Changelog{}, "v1.0.0", false, ""},
		{
			"benign notes stay quiet",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v1.2.0", Body: "Quality of life improvements and another round of bug fixes."},
			}},
			"v1.0.0", false, "",
		},
		{
			"breaking heading in the newest release",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v2.0.0", Body: "## Breaking change\nYou must migrate your data before starting."},
			}},
			"v1.0.0", true, "v2.0.0",
		},
		{
			"breaking note hidden in an intermediate release, not the newest",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v3.1.0", Body: "Bug fixes."},
				{Tag: "v3.0.0", Body: "We removed support for pgvecto.rs; migrate to VectorChord."},
				{Tag: "v2.9.0", Body: "Minor tweaks."},
			}},
			"v2.8.0", true, "v3.0.0",
		},
		{
			"case-insensitive match",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v2.0.0", Body: "ACTION REQUIRED: run the migration script."},
			}},
			"v1.0.0", true, "action required",
		},
		{
			"falls back to raw body when no entries and the span advances",
			&model.Changelog{FromTag: "v1.0.0", ToTag: "v2.0.0", Raw: "This release is NOT backward compatible with older configs."},
			"v1.0.0", true, "not backward compatible",
		},
		{
			"plain feature words do not trip it",
			&model.Changelog{Entries: []model.ReleaseEntry{
				{Tag: "v1.5.0", Body: "New dashboard, faster search, and a migration guide link in the docs."},
			}},
			"v1.0.0", false, "",
		},
		{
			// Regression: a same-version digest rebuild must NOT flag the running
			// release's own breaking note (Immich v3.1.0 iOS-14 case).
			"same-version rebuild does not flag the running release's own note",
			&model.Changelog{FromTag: "v3.1.0", ToTag: "v3.1.0", Entries: []model.ReleaseEntry{
				{Tag: "v3.1.0", Body: "## Breaking change\nDrop support for iOS 14."},
			}},
			"v3.1.0", false, "",
		},
		{
			// Regression: an already-installed older release must not flag (Redis
			// 8.6.5 "data loss" while running 8.10).
			"older already-installed release does not flag",
			&model.Changelog{FromTag: "8.10", ToTag: "8.10", Entries: []model.ReleaseEntry{
				{Tag: "8.6.5", Body: "Fix a bug that could cause data loss on restart."},
			}},
			"8.10", false, "",
		},
		{
			// Raw must not be scanned when the span does not advance the version.
			"raw is ignored on a same-version rebuild",
			&model.Changelog{FromTag: "v1.0.0", ToTag: "v1.0.0", Raw: "This build is not backward compatible."},
			"v1.0.0", false, "",
		},
		{
			// A genuinely newer release above the running version still flags.
			"release newer than running still flags",
			&model.Changelog{FromTag: "v3.1.0", ToTag: "v3.2.0", Entries: []model.ReleaseEntry{
				{Tag: "v3.2.0", Body: "## Breaking change\nConfig format changed."},
			}},
			"v3.1.0", true, "v3.2.0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, hit := ScanBreaking(c.cl, c.from)
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
