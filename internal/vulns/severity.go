package vulns

import (
	"math"
	"regexp"
	"strings"
)

// Severity ranks a vulnerability by the CVSS bands.
type Severity int

const (
	Unrated Severity = iota
	Low
	Medium
	High
	Critical
)

func (s Severity) String() string {
	return [...]string{"unrated", "low", "medium", "high", "critical"}[s]
}

func bandOf(score float64) Severity {
	switch {
	case score >= 9:
		return Critical
	case score >= 7:
		return High
	case score >= 4:
		return Medium
	case score > 0:
		return Low
	}
	return Unrated
}

func labelSeverity(label string) Severity {
	switch strings.ToLower(strings.TrimRight(strings.TrimSpace(label), "*")) {
	case "critical":
		return Critical
	case "high", "important":
		return High
	case "moderate", "medium":
		return Medium
	case "low", "negligible":
		return Low
	}
	return Unrated
}

// osvRecord is the part of a database record the count needs.
type osvRecord struct {
	ID               string         `json:"id"`
	Modified         string         `json:"modified"`
	Aliases          []string       `json:"aliases"`
	Upstream         []string       `json:"upstream"`
	Withdrawn        string         `json:"withdrawn"`
	Severity         []osvSeverity  `json:"severity"`
	DatabaseSpecific map[string]any `json:"database_specific"`
	Affected         []osvAffected  `json:"affected"`
}

type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type osvAffected struct {
	Package           osvPackage     `json:"package"`
	Ranges            []osvRange     `json:"ranges"`
	EcosystemSpecific map[string]any `json:"ecosystem_specific"`
}

type osvRange struct {
	Type   string              `json:"type"`
	Events []map[string]string `json:"events"`
}

type osvPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

// severity prefers a distribution's own priority, which weighs the flaw for
// the package it ships, then a CVSS 3 vector, which every source scores the
// same way, over the label a single source gave.
func (r osvRecord) severity() Severity {
	for _, s := range r.Severity {
		if s.Type == "Ubuntu" {
			if sev := labelSeverity(s.Score); sev != Unrated {
				return sev
			}
		}
	}
	for _, s := range r.Severity {
		if s.Type == "CVSS_V3" {
			if score, ok := baseScore(s.Score); ok {
				return bandOf(score)
			}
		}
	}
	if label, ok := r.DatabaseSpecific["severity"].(string); ok {
		if sev := labelSeverity(label); sev != Unrated {
			return sev
		}
	}
	return Unrated
}

// releaseSeverity holds the urgency a distribution gave the flaw per release,
// which outranks the record's severity for the packages of that release.
func (r osvRecord) releaseSeverity() map[string]Severity {
	var out map[string]Severity
	for _, a := range r.Affected {
		u, _ := a.EcosystemSpecific["urgency"].(string)
		if sev := labelSeverity(u); sev != Unrated {
			if out == nil {
				out = map[string]Severity{}
			}
			out[a.Package.Ecosystem] = sev
		}
	}
	return out
}

// key is the CVE behind a record, so a flaw that a distribution and an
// advisory both list counts once.
func (r osvRecord) key() string {
	if strings.HasPrefix(r.ID, "CVE-") {
		return r.ID
	}
	for _, ids := range [][]string{r.Upstream, r.Aliases} {
		for _, id := range ids {
			if strings.HasPrefix(id, "CVE-") {
				return id
			}
		}
	}
	return r.ID
}

// minor lists the releases whose distribution calls the flaw unimportant for
// the package it ships.
func (r osvRecord) minor() []string {
	var out []string
	for _, a := range r.Affected {
		if u, _ := a.EcosystemSpecific["urgency"].(string); u == "unimportant" {
			out = append(out, a.Package.Ecosystem)
		}
	}
	return out
}

// fixes lists the packages a fixed version exists for, as fixKey spells them.
// A range that ends only in last_affected has none.
func (r osvRecord) fixes() []string {
	var out []string
	for _, a := range r.Affected {
		if a.fixed() {
			out = append(out, fixKey(a.Package.Ecosystem, a.Package.Name))
		}
	}
	return out
}

func (a osvAffected) fixed() bool {
	for _, rg := range a.Ranges {
		switch rg.Type {
		case "ECOSYSTEM", "SEMVER", "GIT":
		default:
			continue
		}
		for _, e := range rg.Events {
			if e["fixed"] != "" {
				return true
			}
		}
	}
	return false
}

var pypiSeparators = regexp.MustCompile(`[-_.]+`)

// fixKey names a package the same way for a query and a record, which may
// spell it in another case, and for PyPI with other separators.
func fixKey(ecosystem, name string) string {
	name = strings.ToLower(name)
	if ecosystem == "PyPI" {
		name = pypiSeparators.ReplaceAllString(name, "-")
	}
	return ecosystem + " " + name
}

// cvssWeights holds the CVSS 3.x base metric values. PR has a second set for
// a changed scope.
var cvssWeights = map[string]map[string]float64{
	"AV": {"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2},
	"AC": {"L": 0.77, "H": 0.44},
	"PR": {"N": 0.85, "L": 0.62, "H": 0.27},
	"UI": {"N": 0.85, "R": 0.62},
	"C":  {"H": 0.56, "L": 0.22, "N": 0},
	"I":  {"H": 0.56, "L": 0.22, "N": 0},
	"A":  {"H": 0.56, "L": 0.22, "N": 0},
}

var changedPR = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}

// baseScore computes the CVSS 3.0 or 3.1 base score of a vector.
func baseScore(vector string) (float64, bool) {
	parts := strings.Split(vector, "/")
	if len(parts) < 9 || !strings.HasPrefix(parts[0], "CVSS:3.") {
		return 0, false
	}
	m := map[string]string{}
	for _, p := range parts[1:] {
		k, v, ok := strings.Cut(p, ":")
		if !ok {
			return 0, false
		}
		m[k] = v
	}
	changed := false
	switch m["S"] {
	case "C":
		changed = true
	case "U":
	default:
		return 0, false
	}
	w := map[string]float64{}
	for k, vals := range cvssWeights {
		v, ok := vals[m[k]]
		if !ok {
			return 0, false
		}
		w[k] = v
	}
	if changed {
		w["PR"] = changedPR[m["PR"]]
	}

	iss := 1 - (1-w["C"])*(1-w["I"])*(1-w["A"])
	impact := 6.42 * iss
	if changed {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	}
	if impact <= 0 {
		return 0, true
	}
	exploit := 8.22 * w["AV"] * w["AC"] * w["PR"] * w["UI"]
	sum := impact + exploit
	if changed {
		sum *= 1.08
	}
	return roundUp(math.Min(sum, 10)), true
}

// roundUp is the specification's Roundup, which works on integers so that
// floating point noise cannot lift a score by a tenth.
func roundUp(x float64) float64 {
	n := int(math.Round(x * 100000))
	if n%10000 == 0 {
		return float64(n) / 100000
	}
	return float64(n/10000+1) / 10
}
