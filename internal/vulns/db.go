package vulns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// DefaultDB is the open vulnerability database the check asks.
const DefaultDB = "https://api.osv.dev"

const (
	batchSize    = 1000 // the most queries the database takes in one request
	maxPages     = 20
	fetchWorkers = 4
	maxRecord    = 8 << 20
)

// Finding is one known vulnerability of a package list.
type Finding struct {
	Severity Severity
	Package  string
}

// summary is what the check keeps of a record. Records can be large, so the
// cache holds only this.
type summary struct {
	Modified  string   `json:"m"`
	Key       string   `json:"k"`
	Severity  Severity `json:"s"`
	Minor     []string `json:"u,omitempty"`
	Withdrawn bool     `json:"w,omitempty"`
}

// DB looks packages up in the vulnerability database. What it learns of a
// record stays in a file under dir until the record changes.
type DB struct {
	base   string
	client *http.Client
	file   string

	mu     sync.Mutex
	loaded bool
	known  map[string]summary
}

// NewDB asks the database at base and caches under dir.
func NewDB(base, dir string) *DB {
	return &DB{
		base:   base,
		client: &http.Client{Timeout: 60 * time.Second},
		file:   filepath.Join(dir, "records.json"),
		known:  map[string]summary{},
	}
}

type hit struct {
	pkg      Package
	id       string
	modified string
}

// Find returns the vulnerabilities that affect pkgs, one per flaw, keyed by
// its CVE where it has one.
func (d *DB) Find(ctx context.Context, pkgs []Package) (map[string]Finding, error) {
	d.load()
	var hits []hit
	for start := 0; start < len(pkgs); start += batchSize {
		chunk := pkgs[start:min(start+batchSize, len(pkgs))]
		h, err := d.query(ctx, chunk)
		if err != nil {
			return nil, err
		}
		hits = append(hits, h...)
	}
	if err := d.fetchChanged(ctx, hits); err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]Finding{}
	for _, h := range hits {
		s := d.known[h.id]
		if s.Withdrawn || slices.Contains(s.Minor, h.pkg.Ecosystem) {
			continue
		}
		if have, ok := out[s.Key]; !ok || s.Severity > have.Severity {
			out[s.Key] = Finding{Severity: s.Severity, Package: h.pkg.Name}
		}
	}
	return out, nil
}

type query struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}

func newQuery(p Package, page string) query {
	var q query
	q.Package.Ecosystem, q.Package.Name, q.Version, q.PageToken = p.Ecosystem, p.Name, p.Version, page
	return q
}

// query asks for the records of up to batchSize packages. A package with more
// records than one answer holds is asked again for the next page.
func (d *DB) query(ctx context.Context, pkgs []Package) ([]hit, error) {
	queries := make([]query, len(pkgs))
	for i, p := range pkgs {
		queries[i] = newQuery(p, "")
	}
	var hits []hit
	for page := 0; len(queries) > 0; page++ {
		if page == maxPages {
			return nil, fmt.Errorf("vulnerability database: more than %d pages of records", maxPages)
		}
		var resp struct {
			Results []struct {
				Vulns []struct {
					ID       string `json:"id"`
					Modified string `json:"modified"`
				} `json:"vulns"`
				Next string `json:"next_page_token"`
			} `json:"results"`
		}
		if err := d.post(ctx, "/v1/querybatch", map[string]any{"queries": queries}, &resp); err != nil {
			return nil, err
		}
		if len(resp.Results) != len(queries) {
			return nil, fmt.Errorf("vulnerability database: %d answers for %d queries", len(resp.Results), len(queries))
		}
		var next []query
		for i, res := range resp.Results {
			p := Package{queries[i].Package.Ecosystem, queries[i].Package.Name, queries[i].Version}
			for _, v := range res.Vulns {
				hits = append(hits, hit{pkg: p, id: v.ID, modified: v.Modified})
			}
			if res.Next != "" {
				next = append(next, newQuery(p, res.Next))
			}
		}
		queries = next
	}
	return hits, nil
}

// fetchChanged reads every record the cache does not hold in its current form.
func (d *DB) fetchChanged(ctx context.Context, hits []hit) error {
	d.mu.Lock()
	want := map[string]bool{}
	for _, h := range hits {
		if s, ok := d.known[h.id]; !ok || s.Modified != h.modified {
			want[h.id] = true
		}
	}
	d.mu.Unlock()
	if len(want) == 0 {
		return nil
	}

	ids := make(chan string)
	errs := make(chan error, len(want))
	var wg sync.WaitGroup
	for range min(fetchWorkers, len(want)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ids {
				var r osvRecord
				if err := d.get(ctx, "/v1/vulns/"+url.PathEscape(id), &r); err != nil {
					errs <- err
					continue
				}
				d.mu.Lock()
				d.known[id] = summary{Modified: r.Modified, Key: r.key(), Severity: r.severity(), Minor: r.minor(), Withdrawn: r.Withdrawn != ""}
				d.mu.Unlock()
			}
		}()
	}
	for id := range want {
		ids <- id
	}
	close(ids)
	wg.Wait()
	close(errs)
	// Whatever arrived is kept, so a retry fetches only what failed.
	d.save()
	return <-errs
}

func (d *DB) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return d.do(req, out)
}

func (d *DB) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.base+path, nil)
	if err != nil {
		return err
	}
	return d.do(req, out)
}

func (d *DB) do(req *http.Request, out any) error {
	req.Header.Set("User-Agent", "shiplog")
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("vulnerability database: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vulnerability database: %s answered %d", req.URL.Path, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRecord)).Decode(out); err != nil {
		return fmt.Errorf("vulnerability database: %s: %w", req.URL.Path, err)
	}
	return nil
}

// load reads the cache once. A missing or broken file only costs fetches.
func (d *DB) load() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.loaded {
		return
	}
	d.loaded = true
	if b, err := os.ReadFile(d.file); err == nil {
		_ = json.Unmarshal(b, &d.known)
		if d.known == nil {
			d.known = map[string]summary{}
		}
	}
}

func (d *DB) save() {
	d.mu.Lock()
	b, err := json.Marshal(d.known)
	d.mu.Unlock()
	if err != nil {
		return
	}
	_ = writeFile(d.file, b)
}

// writeFile replaces path in one step, so a crash leaves the old file.
func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
