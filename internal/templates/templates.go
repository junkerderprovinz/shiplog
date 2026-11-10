// Package templates reads the Unraid dockerMan user templates, one XML file per
// container, for their project page and template URL, and sets the image a
// template runs. An unreadable or malformed template is skipped in a sweep, so
// it cannot break one.
package templates

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Dir is the standard dockerMan user-template directory on Unraid.
const Dir = "/boot/config/plugins/dockerMan/templates-user"

type tmpl struct {
	Name        string `xml:"Name"`
	Project     string `xml:"Project"`
	TemplateURL string `xml:"TemplateURL"`
}

// ProjectPages maps the lower-cased container name to the template's <Project>
// URL. Whether the URL can serve as a changelog source is up to package sources.
func ProjectPages(dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	pages := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".xml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var t tmpl
		if xml.Unmarshal(data, &t) != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(t.Name))
		project := strings.TrimSpace(t.Project)
		if name == "" || project == "" {
			continue
		}
		pages[name] = project
	}
	return pages
}

// Folder is a directory of user templates, such as Folder(Dir).
type Folder string

func (f Folder) path(name string) string { return filepath.Join(string(f), "my-"+name+".xml") }

// Repository returns the image the named container's template runs.
func (f Folder) Repository(name string) (string, error) {
	data, err := os.ReadFile(f.path(name))
	if err != nil {
		return "", err
	}
	var t struct {
		Repository string `xml:"Repository"`
	}
	if err := xml.Unmarshal(data, &t); err != nil {
		return "", fmt.Errorf("template of %s: %w", name, err)
	}
	return strings.TrimSpace(t.Repository), nil
}

var repositoryElement = regexp.MustCompile(`(?s)(<Repository>)(.*?)(</Repository>)`)

// SetRepository has the named container's template run ref and leaves the rest
// of the file byte for byte as Unraid wrote it. The file is replaced in one
// rename, so a crash leaves the old template or the new one.
func (f Folder) SetRepository(name, ref string) error {
	p := f.path(name)
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	loc := repositoryElement.FindSubmatchIndex(data)
	if loc == nil {
		return fmt.Errorf("template of %s names no repository", name)
	}
	var esc bytes.Buffer
	_ = xml.EscapeText(&esc, []byte(ref))
	out := append(append(append([]byte{}, data[:loc[4]]...), esc.Bytes()...), data[loc[5]:]...)

	tmp, err := os.CreateTemp(string(f), ".shiplog-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if fi, err := os.Stat(p); err == nil {
		_ = os.Chmod(tmp.Name(), fi.Mode().Perm())
	}
	return os.Rename(tmp.Name(), p)
}

// TemplateURLs maps the lower-cased container name to the template's
// <TemplateURL>, where Community Applications fetched it from. A 404 there means
// the app was pulled from CA or its source deleted.
func TemplateURLs(dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	urls := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".xml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var t tmpl
		if xml.Unmarshal(data, &t) != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(t.Name))
		u := strings.TrimSpace(t.TemplateURL)
		if name == "" || u == "" {
			continue
		}
		urls[name] = u
	}
	return urls
}
