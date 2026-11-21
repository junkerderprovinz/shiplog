package vulns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeDB answers querybatch from affects (package name -> record ids) and
// serves records by id. It counts the record fetches and keeps every query it
// was sent.
type fakeDB struct {
	affects map[string][]string
	records map[string]osvRecord
	fetches atomic.Int32
	mu      sync.Mutex
	queries []map[string]any
	pages   map[string]int    // package name -> extra pages of ids
	listed  map[string]string // record id -> modified as querybatch spells it
}

func (f *fakeDB) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/querybatch":
			var body struct {
				Queries []map[string]any `json:"queries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			f.mu.Lock()
			f.queries = append(f.queries, body.Queries...)
			f.mu.Unlock()
			type hit struct {
				ID       string `json:"id"`
				Modified string `json:"modified"`
			}
			type result struct {
				Vulns []hit  `json:"vulns,omitempty"`
				Next  string `json:"next_page_token,omitempty"`
			}
			out := struct {
				Results []result `json:"results"`
			}{}
			for _, q := range body.Queries {
				name := q["package"].(map[string]any)["name"].(string)
				ids, ok := f.affects[name+" "+q["version"].(string)]
				if !ok {
					ids = f.affects[name]
				}
				var res result
				if extra := f.pages[name]; extra > 0 {
					page, _ := q["page_token"].(string)
					if page == "" {
						res.Next = "p1"
						ids = ids[:1]
					} else {
						ids = ids[1:]
					}
				}
				for _, id := range ids {
					modified := f.records[id].Modified
					if m, ok := f.listed[id]; ok {
						modified = m
					}
					res.Vulns = append(res.Vulns, hit{ID: id, Modified: modified})
				}
				out.Results = append(out.Results, res)
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/vulns/"):
			f.fetches.Add(1)
			rec, ok := f.records[strings.TrimPrefix(r.URL.Path, "/v1/vulns/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(rec)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const (
	critVector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
	highVector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:H/I:H/A:N"
	lowVector  = "CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:L/I:N/A:N"
)

func rec(id, modified, vector string, upstream ...string) osvRecord {
	return osvRecord{ID: id, Modified: modified, Upstream: upstream, Severity: []osvSeverity{{Type: "CVSS_V3", Score: vector}}}
}

func TestFindCountsEachFlawOnceWithItsSeverity(t *testing.T) {
	f := &fakeDB{
		affects: map[string][]string{
			"openssl": {"DEBIAN-CVE-2024-1", "DEBIAN-CVE-2024-2"},
			"glibc":   {"DEBIAN-CVE-2024-3"},
			"pyyaml":  {"GHSA-aaaa"},
		},
		records: map[string]osvRecord{
			"DEBIAN-CVE-2024-1": rec("DEBIAN-CVE-2024-1", "m1", critVector, "CVE-2024-1"),
			"DEBIAN-CVE-2024-2": rec("DEBIAN-CVE-2024-2", "m1", lowVector, "CVE-2024-2"),
			"DEBIAN-CVE-2024-3": rec("DEBIAN-CVE-2024-3", "m1", highVector, "CVE-2024-3"),
			"GHSA-aaaa":         {ID: "GHSA-aaaa", Modified: "m1", Aliases: []string{"CVE-2024-1"}, DatabaseSpecific: map[string]any{"severity": "HIGH"}},
		},
	}
	db := NewDB(f.server(t).URL, t.TempDir())
	got, err := db.Find(context.Background(), []Package{
		{"Debian:12", "openssl", "3.0.11-1~deb12u2"},
		{"Debian:12", "glibc", "2.36-9"},
		{"PyPI", "pyyaml", "5.3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Finding{
		"CVE-2024-1": {Severity: Critical, Package: "openssl"},
		"CVE-2024-2": {Severity: Low, Package: "openssl"},
		"CVE-2024-3": {Severity: High, Package: "glibc"},
	}
	if len(got) != len(want) {
		t.Fatalf("Find = %+v, want %+v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
}

func TestOnlyNamesAndVersionsLeaveTheHost(t *testing.T) {
	f := &fakeDB{affects: map[string][]string{}, records: map[string]osvRecord{}}
	db := NewDB(f.server(t).URL, t.TempDir())
	if _, err := db.Find(context.Background(), []Package{{"Debian:12", "openssl", "3.0.11"}}); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(f.queries)
	if want := `[{"package":{"ecosystem":"Debian:12","name":"openssl"},"version":"3.0.11"}]`; string(b) != want {
		t.Errorf("sent %s, want %s", b, want)
	}
}

func TestARecordIsFetchedAgainOnlyWhenItChanged(t *testing.T) {
	f := &fakeDB{
		affects: map[string][]string{"openssl": {"DEBIAN-CVE-2024-1"}},
		records: map[string]osvRecord{"DEBIAN-CVE-2024-1": rec("DEBIAN-CVE-2024-1", "m1", critVector, "CVE-2024-1")},
	}
	srv := f.server(t)
	dir := t.TempDir()
	pkgs := []Package{{"Debian:12", "openssl", "3.0.11"}}
	if _, err := NewDB(srv.URL, dir).Find(context.Background(), pkgs); err != nil {
		t.Fatal(err)
	}
	// A restart reads the records from the cache.
	if _, err := NewDB(srv.URL, dir).Find(context.Background(), pkgs); err != nil {
		t.Fatal(err)
	}
	if n := f.fetches.Load(); n != 1 {
		t.Fatalf("fetched %d times, want 1", n)
	}
	f.records["DEBIAN-CVE-2024-1"] = rec("DEBIAN-CVE-2024-1", "m2", highVector, "CVE-2024-1")
	got, err := NewDB(srv.URL, dir).Find(context.Background(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if n := f.fetches.Load(); n != 2 {
		t.Errorf("fetched %d times after the change, want 2", n)
	}
	if got["CVE-2024-1"].Severity != High {
		t.Errorf("severity = %v, want the changed record's high", got["CVE-2024-1"].Severity)
	}
}

func TestARecordWhoseTimeIsSpelledFinerIsNotFetchedAgain(t *testing.T) {
	f := &fakeDB{
		affects: map[string][]string{"openssl": {"DEBIAN-CVE-2024-1"}},
		records: map[string]osvRecord{"DEBIAN-CVE-2024-1": rec("DEBIAN-CVE-2024-1", "2026-10-26T18:02:56.359328632Z", critVector, "CVE-2024-1")},
		listed:  map[string]string{"DEBIAN-CVE-2024-1": "2026-10-26T18:02:56.359328Z"},
	}
	db := NewDB(f.server(t).URL, t.TempDir())
	pkgs := []Package{{"Debian:12", "openssl", "3.0.11"}}
	for range 2 {
		if _, err := db.Find(context.Background(), pkgs); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fetches.Load(); n != 1 {
		t.Errorf("fetched %d times, want 1", n)
	}
}

func TestAReleasesUrgencyDecidesForItsPackages(t *testing.T) {
	r := rec("DEBIAN-CVE-2024-1", "m1", critVector, "CVE-2024-1")
	r.Affected = []osvAffected{{Package: osvPackage{Ecosystem: "Debian:12", Name: "openssl"}, EcosystemSpecific: map[string]any{"urgency": "low"}}}
	f := &fakeDB{affects: map[string][]string{"openssl": {"DEBIAN-CVE-2024-1"}}, records: map[string]osvRecord{"DEBIAN-CVE-2024-1": r}}
	db := NewDB(f.server(t).URL, t.TempDir())
	got, err := db.Find(context.Background(), []Package{{"Debian:12", "openssl", "3.0.11"}, {"Debian:13", "openssl", "3.0.11"}})
	if err != nil {
		t.Fatal(err)
	}
	if got["CVE-2024-1"].Severity != Critical {
		t.Errorf("severity = %v, want critical from the release without an urgency", got["CVE-2024-1"].Severity)
	}
	got, _ = db.Find(context.Background(), []Package{{"Debian:12", "openssl", "3.0.11"}})
	if got["CVE-2024-1"].Severity != Low {
		t.Errorf("severity = %v, want Debian 12's low", got["CVE-2024-1"].Severity)
	}
}

func TestUnimportantAndWithdrawnRecordsDoNotCount(t *testing.T) {
	minor := rec("DEBIAN-CVE-2024-1", "m1", critVector, "CVE-2024-1")
	minor.Affected = []osvAffected{{Package: osvPackage{Ecosystem: "Debian:12", Name: "openssl"}, EcosystemSpecific: map[string]any{"urgency": "unimportant"}}}
	gone := rec("GHSA-gone", "m1", critVector)
	gone.Withdrawn = "2024-01-01T00:00:00Z"
	f := &fakeDB{
		affects: map[string][]string{"openssl": {"DEBIAN-CVE-2024-1"}, "requests": {"GHSA-gone"}},
		records: map[string]osvRecord{"DEBIAN-CVE-2024-1": minor, "GHSA-gone": gone},
	}
	got, err := NewDB(f.server(t).URL, t.TempDir()).Find(context.Background(), []Package{
		{"Debian:12", "openssl", "3.0.11"},
		{"PyPI", "requests", "2.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("Find = %+v, want nothing", got)
	}
}

func TestAPackageWithManyRecordsIsReadToTheLastPage(t *testing.T) {
	f := &fakeDB{
		affects: map[string][]string{"linux": {"DEBIAN-CVE-1", "DEBIAN-CVE-2"}},
		records: map[string]osvRecord{
			"DEBIAN-CVE-1": rec("DEBIAN-CVE-1", "m", critVector, "CVE-1"),
			"DEBIAN-CVE-2": rec("DEBIAN-CVE-2", "m", critVector, "CVE-2"),
		},
		pages: map[string]int{"linux": 1},
	}
	got, err := NewDB(f.server(t).URL, t.TempDir()).Find(context.Background(), []Package{{"Debian:12", "linux", "6.1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("Find = %+v, want both records", got)
	}
}

func TestAFailedLookupIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := NewDB(srv.URL, t.TempDir()).Find(context.Background(), []Package{{"Debian:12", "openssl", "3"}}); err == nil {
		t.Error("a failed lookup must not read as no vulnerabilities")
	}
}

// affected adds an affected entry for name in eco whose range ends in events.
func affected(r osvRecord, eco, name string, events ...map[string]string) osvRecord {
	r.Affected = append(r.Affected, osvAffected{
		Package: osvPackage{Ecosystem: eco, Name: name},
		Ranges:  []osvRange{{Type: "ECOSYSTEM", Events: append([]map[string]string{{"introduced": "0"}}, events...)}},
	})
	return r
}

func TestFindTellsAFlawWithAFixFromOneWithout(t *testing.T) {
	fixed := affected(rec("DEBIAN-CVE-1", "m", critVector, "CVE-1"), "Debian:13", "curl", map[string]string{"fixed": "8.14.1-2+deb13u8"})
	open := affected(rec("DEBIAN-CVE-2", "m", critVector, "CVE-2"), "Debian:13", "curl")
	open = affected(open, "Debian:12", "curl", map[string]string{"fixed": "7.88.1-10+deb12u9"})
	last := affected(rec("GHSA-last", "m", highVector, "CVE-3"), "PyPI", "PyYAML", map[string]string{"last_affected": "5.3"})
	py := affected(rec("GHSA-py", "m", highVector, "CVE-4"), "PyPI", "Typing_Extensions", map[string]string{"fixed": "4.1"})
	f := &fakeDB{
		affects: map[string][]string{"curl": {"DEBIAN-CVE-1", "DEBIAN-CVE-2"}, "pyyaml": {"GHSA-last"}, "typing-extensions": {"GHSA-py"}},
		records: map[string]osvRecord{"DEBIAN-CVE-1": fixed, "DEBIAN-CVE-2": open, "GHSA-last": last, "GHSA-py": py},
	}
	got, err := NewDB(f.server(t).URL, t.TempDir()).Find(context.Background(), []Package{
		{"Debian:13", "curl", "8.14.1-2+deb13u7"},
		{"PyPI", "pyyaml", "5.3"},
		{"PyPI", "typing-extensions", "4.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"CVE-1": true, "CVE-2": false, "CVE-3": false, "CVE-4": true}
	for id, fixable := range want {
		if got[id].Fixable != fixable {
			t.Errorf("%s: fixable = %v, want %v", id, got[id].Fixable, fixable)
		}
	}
}

func TestAFlawStaysOpenWhileOnePackageHasNoFix(t *testing.T) {
	r := affected(rec("CVE-1", "m", critVector), "Debian:13", "openssl", map[string]string{"fixed": "3.5.1"})
	r = affected(r, "Go", "golang.org/x/crypto")
	f := &fakeDB{
		affects: map[string][]string{"openssl": {"CVE-1"}, "golang.org/x/crypto": {"CVE-1"}},
		records: map[string]osvRecord{"CVE-1": r},
	}
	got, err := NewDB(f.server(t).URL, t.TempDir()).Find(context.Background(), []Package{
		{"Debian:13", "openssl", "3.5.0"},
		{"Go", "golang.org/x/crypto", "0.1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["CVE-1"].Fixable {
		t.Error("a flaw with no fix for one of its packages reads as fixable")
	}
}

func TestACacheWithoutFixesIsReadAgain(t *testing.T) {
	r := affected(rec("DEBIAN-CVE-1", "m1", critVector, "CVE-1"), "Debian:13", "curl", map[string]string{"fixed": "8.14.1-2+deb13u8"})
	f := &fakeDB{affects: map[string][]string{"curl": {"DEBIAN-CVE-1"}}, records: map[string]osvRecord{"DEBIAN-CVE-1": r}}
	dir := t.TempDir()
	old := `{"DEBIAN-CVE-1":{"m":"m1","k":"CVE-1","s":4}}`
	if err := os.WriteFile(filepath.Join(dir, "records-2.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewDB(f.server(t).URL, dir).Find(context.Background(), []Package{{"Debian:13", "curl", "8.14.1-2+deb13u7"}})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.fetches.Load(); n != 1 || !got["CVE-1"].Fixable {
		t.Errorf("fetched %d times, fixable = %v, want the record read again with its fix", n, got["CVE-1"].Fixable)
	}
}
