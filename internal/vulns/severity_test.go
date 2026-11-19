package vulns

import "testing"

func TestBaseScoreMatchesTheSpecificationExamples(t *testing.T) {
	cases := []struct {
		vector string
		want   float64
	}{
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:H", 9.1},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N", 6.1},
		{"CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:H/I:H/A:N", 5.6},
		{"CVSS:3.0/AV:N/AC:L/PR:L/UI:N/S:C/C:H/I:H/A:H", 9.9},
		{"CVSS:3.1/AV:P/AC:H/PR:H/UI:R/S:U/C:N/I:N/A:N", 0},
	}
	for _, c := range cases {
		got, ok := baseScore(c.vector)
		if !ok || got != c.want {
			t.Errorf("baseScore(%s) = %v, %v; want %v", c.vector, got, ok, c.want)
		}
	}
}

func TestBaseScoreRefusesWhatItCannotRead(t *testing.T) {
	for _, v := range []string{"", "AV:N/AC:L/Au:N/C:P/I:P/A:P", "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N", "CVSS:3.1/AV:N/AC:L"} {
		if _, ok := baseScore(v); ok {
			t.Errorf("baseScore(%q) scored a vector it cannot read", v)
		}
	}
}

func TestSeverityOfARecord(t *testing.T) {
	cases := []struct {
		name string
		rec  osvRecord
		want Severity
	}{
		{"cvss 3 vector", osvRecord{Severity: []osvSeverity{{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:H"}}}, Critical},
		{"high band", osvRecord{Severity: []osvSeverity{{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:H/I:H/A:N"}}}, High},
		{"advisory label without a vector", osvRecord{
			Severity:         []osvSeverity{{Type: "CVSS_V4", Score: "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N"}},
			DatabaseSpecific: map[string]any{"severity": "CRITICAL"},
		}, Critical},
		{"moderate label", osvRecord{DatabaseSpecific: map[string]any{"severity": "MODERATE"}}, Medium},
		{"distribution priority", osvRecord{Severity: []osvSeverity{{Type: "Ubuntu", Score: "high"}}}, High},
		{"nothing to go by", osvRecord{}, Unrated},
	}
	for _, c := range cases {
		if got := c.rec.severity(); got != c.want {
			t.Errorf("%s: severity = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestADistributionsOwnPriorityOutranksTheVector(t *testing.T) {
	rec := osvRecord{Severity: []osvSeverity{
		{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H"},
		{Type: "Ubuntu", Score: "negligible"},
	}}
	if got := rec.severity(); got != Low {
		t.Errorf("severity = %v, want the distribution's negligible as low", got)
	}
}

func TestAReleasesUrgencyOutranksTheVector(t *testing.T) {
	rec := osvRecord{Affected: []osvAffected{
		{Package: osvPackage{Ecosystem: "Debian:12"}, EcosystemSpecific: map[string]any{"urgency": "low"}},
		{Package: osvPackage{Ecosystem: "Debian:13"}, EcosystemSpecific: map[string]any{"urgency": "medium**"}},
		{Package: osvPackage{Ecosystem: "Debian:14"}, EcosystemSpecific: map[string]any{"urgency": "not yet assigned"}},
		{Package: osvPackage{Ecosystem: "Debian:11"}, EcosystemSpecific: map[string]any{"urgency": "unimportant"}},
	}}
	want := map[string]Severity{"Debian:12": Low, "Debian:13": Medium}
	got := rec.releaseSeverity()
	if len(got) != len(want) || got["Debian:12"] != Low || got["Debian:13"] != Medium {
		t.Errorf("releaseSeverity = %v, want %v", got, want)
	}
}

func TestAVectorOutranksALabel(t *testing.T) {
	rec := osvRecord{
		Severity:         []osvSeverity{{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"}},
		DatabaseSpecific: map[string]any{"severity": "LOW"},
	}
	if got := rec.severity(); got != Critical {
		t.Errorf("severity = %v, want critical from the vector", got)
	}
}

func TestARecordIsKeyedByItsCVE(t *testing.T) {
	cases := []struct {
		rec  osvRecord
		want string
	}{
		{osvRecord{ID: "DEBIAN-CVE-2024-5535", Upstream: []string{"CVE-2024-5535"}}, "CVE-2024-5535"},
		{osvRecord{ID: "GHSA-9wx4-h78v-vm56", Aliases: []string{"PYSEC-2026-1873", "CVE-2024-35195"}}, "CVE-2024-35195"},
		{osvRecord{ID: "CVE-2024-1"}, "CVE-2024-1"},
		{osvRecord{ID: "GHSA-xxxx"}, "GHSA-xxxx"},
	}
	for _, c := range cases {
		if got := c.rec.key(); got != c.want {
			t.Errorf("key(%s) = %s, want %s", c.rec.ID, got, c.want)
		}
	}
}

func TestADistributionCanCallARecordUnimportant(t *testing.T) {
	rec := osvRecord{Affected: []osvAffected{
		{Package: osvPackage{Ecosystem: "Debian:12"}, EcosystemSpecific: map[string]any{"urgency": "unimportant"}},
		{Package: osvPackage{Ecosystem: "Debian:13"}, EcosystemSpecific: map[string]any{"urgency": "low"}},
	}}
	got := rec.minor()
	if len(got) != 1 || got[0] != "Debian:12" {
		t.Errorf("minor = %v, want [Debian:12]", got)
	}
}
