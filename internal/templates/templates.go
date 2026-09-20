// Package templates reads the Unraid dockerMan user templates, one XML file per
// container, for their project page and template URL. An unreadable or
// malformed template is skipped, so it cannot break a sweep.
package templates

import (
	"encoding/xml"
	"os"
	"path/filepath"
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
