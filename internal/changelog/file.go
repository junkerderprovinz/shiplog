package changelog

import (
	"context"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// maxFileSize caps how much of a changelog file is read. The newest entries sit
// at the top, so a cut-off tail loses only old history.
const maxFileSize = 1 << 20

// headLines is how much of a file without version headings is shown.
const headLines = 40

// File resolves a changelog from a plain text or Markdown file, for projects
// that keep a CHANGELOG.md or History.txt instead of publishing releases.
type File struct {
	httpClient *http.Client
	ttl        time.Duration

	mu    sync.Mutex
	cache map[string]*fileCache
}

type fileCache struct {
	etag, lastModified string
	text               string
	fetchedAt          time.Time
}

// NewFile returns a File provider.
func NewFile() *File {
	return &File{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		ttl:        10 * time.Minute,
		cache:      make(map[string]*fileCache),
	}
}

// Get shows the file's sections between the running and the newest version
// when both are versions and the file has matching headings, else its newest
// section, else its first lines.
func (f *File) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	if c.ChangelogFile == "" {
		return nil, false
	}
	text, ok := f.fetch(ctx, c.ChangelogFile)
	if !ok {
		return nil, false
	}
	cl := &model.Changelog{
		FromTag:  fromTag,
		ToTag:    toTag,
		Source:   "changelog file (manual source)",
		Provider: "file",
		URL:      c.ChangelogFile,
	}
	secs := splitSections(text)
	if len(secs) == 0 {
		cl.Raw = firstLines(text, headLines)
		return cl, cl.Raw != ""
	}

	var picked []fileSection
	fromV, fromOK := parseSemver(fromTag)
	toV, toOK := parseSemver(toTag)
	if fromOK && toOK && toV.compare(fromV) > 0 {
		for _, s := range secs {
			if s.version.compare(fromV) > 0 && s.version.compare(toV) <= 0 {
				picked = append(picked, s)
			}
		}
	}
	if picked == nil {
		picked = secs[:1]
	}
	for _, s := range picked {
		cl.Entries = append(cl.Entries, model.ReleaseEntry{Tag: s.title, Body: s.body, URL: c.ChangelogFile, PublishedAt: s.date})
	}
	if len(picked) > 1 {
		cl.Recent = true
		cl.Raw = picked[0].body
	} else {
		cl.Raw = "## " + picked[0].title + "\n" + picked[0].body
	}
	return cl, true
}

// fetch serves a cached copy younger than ttl without a request and revalidates
// an older one. A failed request falls back to the stale copy.
func (f *File) fetch(ctx context.Context, url string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	cached := f.cache[url]
	if cached != nil && time.Since(cached.fetchedAt) < f.ttl {
		return cached.text, true
	}
	stale := func() (string, bool) {
		if cached == nil {
			return "", false
		}
		return cached.text, true
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return stale()
	}
	if cached != nil {
		if cached.etag != "" {
			req.Header.Set("If-None-Match", cached.etag)
		}
		if cached.lastModified != "" {
			req.Header.Set("If-Modified-Since", cached.lastModified)
		}
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return stale()
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotModified && cached != nil:
		cached.fetchedAt = time.Now()
		return cached.text, true
	case resp.StatusCode != http.StatusOK || IsWebPage(resp.Header.Get("Content-Type")):
		return stale()
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxFileSize))
	if err != nil {
		return stale()
	}
	fc := &fileCache{
		etag:         resp.Header.Get("ETag"),
		lastModified: resp.Header.Get("Last-Modified"),
		text:         strings.ReplaceAll(string(b), "\r\n", "\n"),
		fetchedAt:    time.Now(),
	}
	f.cache[url] = fc
	return fc.text, true
}

// IsWebPage reports an HTML content type: a URL that answers with a page, such
// as a repo's file view, is not the file itself.
func IsWebPage(contentType string) bool {
	mt, _, _ := mime.ParseMediaType(contentType)
	return mt == "text/html" || mt == "application/xhtml+xml"
}

type fileSection struct {
	title   string
	version semver
	date    time.Time
	body    string
}

var (
	atxHeading = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	setextRule = regexp.MustCompile(`^(=+|-+)\s*$`)
	// A plain-text heading starts in the first column, such as "Version 2026.4"
	// or "1.4.2 (2024-05-01)".
	plainHeading = regexp.MustCompile(`(?i)^(?:version|release|v)?\s*\[?\d+(?:\.\d+)+\b`)
	versionIn    = regexp.MustCompile(`\d+(?:\.\d+){1,2}`)
	dateIn       = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)
)

type heading struct {
	line, level int
	title       string
}

// splitSections cuts the file at its version headings, newest first as
// changelogs are written. A heading without a version, such as "Unreleased" or
// "### Fixed", either ends a section or belongs to it, depending on its level.
func splitSections(text string) []fileSection {
	lines := strings.Split(text, "\n")
	var heads []heading
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case atxHeading.MatchString(l):
			m := atxHeading.FindStringSubmatch(l)
			heads = append(heads, heading{i, len(m[1]), cleanTitle(m[2])})
		case strings.TrimSpace(l) != "" && !isListItem(l) && i+1 < len(lines) && setextRule.MatchString(lines[i+1]):
			level := 2
			if lines[i+1][0] == '=' {
				level = 1
			}
			heads = append(heads, heading{i, level, cleanTitle(l)})
			i++
		case len(l) <= 80 && plainHeading.MatchString(l):
			heads = append(heads, heading{i, 2, cleanTitle(l)})
		}
	}

	level := 0
	for _, h := range heads {
		if versionIn.MatchString(h.title) {
			level = h.level
			break
		}
	}
	if level == 0 {
		return nil
	}

	var bounds []heading
	for _, h := range heads {
		if h.level <= level {
			bounds = append(bounds, h)
		}
	}
	var secs []fileSection
	for i, h := range bounds {
		v, ok := parseSemver(versionIn.FindString(h.title))
		if !ok {
			continue
		}
		end := len(lines)
		if i+1 < len(bounds) {
			end = bounds[i+1].line
		}
		start := h.line + 1
		// A setext heading's underline is not part of the body.
		if start < len(lines) && setextRule.MatchString(lines[start]) {
			start++
		}
		s := fileSection{
			title:   h.title,
			version: v,
			body:    strings.Trim(strings.Join(lines[start:end], "\n"), "\n"),
		}
		if d := dateIn.FindString(h.title); d != "" {
			s.date, _ = time.Parse("2006-01-02", d)
		}
		secs = append(secs, s)
	}
	return secs
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// cleanTitle turns "[1.2.3](https://…/compare) - 2024-05-01" into
// "1.2.3 - 2024-05-01".
func cleanTitle(t string) string {
	t = mdLink.ReplaceAllString(t, "$1")
	return strings.TrimSpace(strings.NewReplacer("[", "", "]", "").Replace(t))
}

func isListItem(l string) bool {
	t := strings.TrimSpace(l)
	return strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") || strings.HasPrefix(t, "+ ")
}

func firstLines(text string, n int) string {
	lines := strings.Split(strings.TrimLeft(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n ")
}
