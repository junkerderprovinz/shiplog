// Package plugins reads where the installed Unraid plugins stand, the way
// Unraid's Plugins tab and its `plugin` script read it. It fetches nothing:
// Unraid's daily plugincheck (or a check from the Plugins tab) downloads each
// plugin's newest .plg into /tmp/plugins, and the scanner compares that file
// with the installed one.
package plugins

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The places Unraid keeps the plugins and its own version.
const (
	InstalledDir = "/var/log/plugins"
	FetchedDir   = "/tmp/plugins"
	VersionFile  = "/etc/unraid-version"
)

// State is how a plugin stands against its newest release.
type State string

const (
	// StateCurrent runs the newest release the last check found.
	StateCurrent State = "current"
	// StateAvailable has a newer release that this Unraid can install.
	StateAvailable State = "available"
	// StateIncompatible has a newer release that needs another Unraid
	// version, which `plugin update` refuses.
	StateIncompatible State = "incompatible"
	// StateUnchecked has not been checked since boot.
	StateUnchecked State = "unchecked"
)

// Plugin is one installed plugin.
type Plugin struct {
	// File is the .plg file name, which `plugin update` takes.
	File      string `json:"file"`
	Name      string `json:"name"`
	Author    string `json:"author,omitempty"`
	Installed string `json:"installed"`
	Latest    string `json:"latest,omitempty"`
	State     State  `json:"state"`
	// Min and Max bound the Unraid versions the newest release installs on.
	Min     string `json:"min,omitempty"`
	Max     string `json:"max,omitempty"`
	Support string `json:"support,omitempty"`
	URL     string `json:"pluginURL"`
	// Changes holds the release notes newer than the installed version, when
	// the newest .plg carries them.
	Changes   string    `json:"changes,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitzero"`
}

// HasUpdate reports whether a newer release is known, installable or not.
func (p Plugin) HasUpdate() bool { return p.State == StateAvailable || p.State == StateIncompatible }

// Scanner reads the plugins from the directories Unraid keeps them in.
type Scanner struct {
	Installed string // InstalledDir
	Fetched   string // FetchedDir
	// Unraid returns the running Unraid version, empty when unknown.
	Unraid func() string
}

// Default is the scanner for this host, or nil when it is not Unraid.
func Default() *Scanner {
	if fi, err := os.Stat(InstalledDir); err != nil || !fi.IsDir() {
		return nil
	}
	return &Scanner{Installed: InstalledDir, Fetched: FetchedDir, Unraid: func() string { return ReadUnraidVersion(VersionFile) }}
}

// builtin are the OS plugins `plugin checkall` leaves out, as Unraid updates
// them on its own.
var builtin = map[string]bool{"unRAIDServer": true, "unRAIDServer-": true}

// Scan lists the installed plugins sorted by name. A plugin whose .plg does
// not parse is left out, as the `plugin` script leaves it out.
func (s *Scanner) Scan() ([]Plugin, error) {
	links, err := filepath.Glob(filepath.Join(s.Installed, "*.plg"))
	if err != nil {
		return nil, err
	}
	unraid := ""
	if s.Unraid != nil {
		unraid = s.Unraid()
	}
	out := []Plugin{}
	for _, link := range links {
		file := filepath.Base(link)
		if builtin[strings.TrimSuffix(file, ".plg")] {
			continue
		}
		// Only a symlink to an existing file is installed.
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(s.Installed, target)
		}
		inst, err := ReadFile(target)
		if err != nil || inst.URL == "" {
			continue
		}
		p := Plugin{File: file, Name: inst.Name, Author: inst.Author, Installed: inst.Version, Support: inst.Support, URL: inst.URL, State: StateUnchecked}
		if p.Name == "" {
			p.Name = strings.TrimSuffix(file, ".plg")
		}
		fetched := filepath.Join(s.Fetched, file)
		if fi, err := os.Stat(fetched); err == nil {
			p.CheckedAt = fi.ModTime()
			if latest, err := ReadFile(fetched); err == nil && latest.Version != "" {
				p.Latest, p.Min, p.Max = latest.Version, latest.Min, latest.Max
				p.State = state(inst.Version, latest, unraid)
				if p.HasUpdate() {
					p.Changes = Newer(latest.Changes, inst.Version)
				}
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// state compares as Unraid compares: a plain string comparison of the
// versions, so the scanner never offers an update the Plugins tab does not.
func state(installed string, latest Manifest, unraid string) State {
	if strings.Compare(latest.Version, installed) <= 0 {
		return StateCurrent
	}
	if unraid != "" {
		if latest.Min != "" && CompareVersions(unraid, latest.Min) < 0 {
			return StateIncompatible
		}
		if latest.Max != "" && CompareVersions(unraid, latest.Max) > 0 {
			return StateIncompatible
		}
	}
	return StateAvailable
}

// Manifest is what a .plg says about its plugin.
type Manifest struct {
	Name, Author, Version, URL, Support, Min, Max string
	Changes                                       string
}

var errNoPlugin = errors.New("plugins: no PLUGIN element")

// ReadFile reads the manifest of the .plg at path.
func ReadFile(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return Parse(b)
}

var entityRE = regexp.MustCompile(`<!ENTITY\s+([A-Za-z_][\w.-]*)\s+(?:"([^"]*)"|'([^']*)')\s*>`)
var refRE = regexp.MustCompile(`&([A-Za-z_][\w.-]*);`)

// Parse reads a .plg. Its DOCTYPE entities are expanded the way libxml
// expands them for Unraid, including entities that use other entities.
func Parse(b []byte) (Manifest, error) {
	ents := entities(b)
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = false // release notes often hold a bare & or <
	d.Entity = ents
	var m Manifest
	inChanges, found := false, false
	var changes strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "PLUGIN":
				found = true
				for _, a := range t.Attr {
					v := expand(a.Value, ents)
					switch a.Name.Local {
					case "name":
						m.Name = v
					case "author":
						m.Author = v
					case "version":
						m.Version = v
					case "pluginURL":
						m.URL = v
					case "support":
						m.Support = v
					case "min":
						m.Min = v
					case "max":
						m.Max = v
					case "Unraid":
						// The plugin script reads it as max when max is missing.
						if m.Max == "" {
							m.Max = v
						}
					}
				}
			case "CHANGES":
				inChanges = true
			}
		case xml.EndElement:
			if t.Name.Local == "CHANGES" {
				inChanges = false
			}
		case xml.CharData:
			if inChanges {
				changes.Write(t)
			}
		}
	}
	if !found {
		return Manifest{}, errNoPlugin
	}
	m.Changes = strings.TrimSpace(expand(changes.String(), ents))
	return m, nil
}

func entities(b []byte) map[string]string {
	raw := map[string]string{}
	head := b
	if i := bytes.Index(b, []byte("]>")); i >= 0 {
		head = b[:i]
	}
	for _, m := range entityRE.FindAllSubmatch(head, -1) {
		v := m[2]
		if v == nil {
			v = m[3]
		}
		raw[string(m[1])] = string(v)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = expand(v, raw)
	}
	return out
}

// expand replaces &name; with its entity, a few levels deep, and leaves an
// unknown reference as written.
func expand(s string, ents map[string]string) string {
	for range 8 {
		next := refRE.ReplaceAllStringFunc(s, func(ref string) string {
			if v, ok := ents[ref[1:len(ref)-1]]; ok {
				return v
			}
			return ref
		})
		if next == s {
			break
		}
		s = next
	}
	return s
}

var headingRE = regexp.MustCompile(`(?m)^\s*#{2,4}\s*v?([0-9][^\s#]*)\s*$`)

// Newer keeps the release-note sections above the first version heading
// (###2026.10.12) that does not sort after installed, as Unraid compares
// versions. Notes run newest first, and stopping there keeps an old heading
// with a typo in its year out. Notes without such headings come back whole.
func Newer(changes, installed string) string {
	idx := headingRE.FindAllStringSubmatchIndex(changes, -1)
	if len(idx) == 0 {
		return changes
	}
	var keep []string
	for i, m := range idx {
		end := len(changes)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		if strings.Compare(changes[m[2]:m[3]], installed) <= 0 {
			break
		}
		keep = append(keep, strings.TrimSpace(changes[m[0]:end]))
	}
	return strings.Join(keep, "\n\n")
}

var versionRE = regexp.MustCompile(`(?m)^version="?([^"\s]+)"?`)

// ReadUnraidVersion reads the version from /etc/unraid-version.
func ReadUnraidVersion(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if m := versionRE.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

// CompareVersions compares dotted versions the way PHP's version_compare
// does for the plain numeric versions Unraid uses (7.2.1, 6.12.10-rc.2).
// A pre-release part sorts before the release.
func CompareVersions(a, b string) int {
	pa, ra, _ := strings.Cut(a, "-")
	pb, rb, _ := strings.Cut(b, "-")
	as, bs := strings.Split(pa, "."), strings.Split(pb, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		x, y := num(as, i), num(bs, i)
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case ra == rb:
		return 0
	case ra == "":
		return 1
	case rb == "":
		return -1
	}
	return strings.Compare(ra, rb)
}

func num(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n := 0
	for _, c := range parts[i] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
