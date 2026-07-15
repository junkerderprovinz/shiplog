// Package templates reads the Unraid dockerMan user templates so the engine
// can use a container's <Project> page as a changelog-source candidate
// (sources.KindProject — community suggestion by btTeddy: most templates carry
// a proper GitHub link there, which covers images whose OCI source label is
// missing or inherited from a base image).
//
// The templates live as one XML file per container under
// /boot/config/plugins/dockerMan/templates-user/. Reading them is strictly
// best-effort: any unreadable file or malformed XML is skipped — a broken
// template must never break a sweep.
package templates

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// Dir is the standard dockerMan user-template directory on Unraid.
const Dir = "/boot/config/plugins/dockerMan/templates-user"

// tmpl mirrors the two template fields we consume.
type tmpl struct {
	Name    string `xml:"Name"`
	Project string `xml:"Project"`
}

// ProjectPages maps container name (lower-cased) → the template's <Project>
// URL, for every readable template in dir that has both fields. Non-GitHub
// URLs are included as-is — the sources package decides usability.
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
