package plugin

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type plgFile struct {
	Name   string `xml:"Name,attr"`
	Run    string `xml:"Run,attr"`
	Method string `xml:"Method,attr"`
	Inline string `xml:"INLINE"`
}

// plgEntities reads the DOCTYPE entities of shiplog.plg with every reference
// inside them resolved.
func plgEntities(t *testing.T, plg string) map[string]string {
	t.Helper()
	ents := map[string]string{}
	for _, m := range regexp.MustCompile(`<!ENTITY\s+(\w+)\s+"([^"]*)">`).FindAllStringSubmatch(plg, -1) {
		ents[m[1]] = m[2]
	}
	ref := regexp.MustCompile(`&(\w+);`)
	for range ents {
		for k, v := range ents {
			ents[k] = ref.ReplaceAllStringFunc(v, func(s string) string {
				if r, ok := ents[s[1:len(s)-1]]; ok {
					return r
				}
				return s
			})
		}
	}
	return ents
}

func plgFiles(t *testing.T, plg string, ents map[string]string) []plgFile {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(plg))
	d.Entity = ents
	var files []plgFile
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "FILE" {
			var f plgFile
			if err := d.DecodeElement(&f, &se); err != nil {
				t.Fatal(err)
			}
			files = append(files, f)
		}
	}
	return files
}

// TestAnInstallKeepsThePackageItIsAboutToInstall runs the steps before the
// package on a flash folder holding this version's package and an older one.
// Unraid installs every .plg again at boot, often before the network is up,
// so the package on the flash is what ShipLog starts from.
func TestAnInstallKeepsThePackageItIsAboutToInstall(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("needs bash")
	}
	raw, err := os.ReadFile("shiplog.plg")
	if err != nil {
		t.Fatal(err)
	}
	plg := string(raw)
	ents := plgEntities(t, plg)
	flash := filepath.ToSlash(t.TempDir())
	emhttp := filepath.ToSlash(t.TempDir())
	pkg := ents["plgPath"] + "/" + ents["txz"]

	var before []string
	found := false
	for _, f := range plgFiles(t, plg, ents) {
		if f.Name == pkg {
			found = true
			break
		}
		if f.Run == "/bin/bash" && f.Method == "" {
			before = append(before, f.Inline)
		}
	}
	if !found {
		t.Fatalf("shiplog.plg has no FILE for %s", pkg)
	}

	current := ents["txz"]
	older := "shiplog-0.0.1-x86_64-1.txz"
	for _, name := range []string{current, older} {
		if err := os.WriteFile(filepath.Join(flash, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, script := range before {
		script = strings.ReplaceAll(script, ents["plgPath"], flash)
		script = strings.ReplaceAll(script, ents["emhttp"], emhttp)
		if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
			t.Fatalf("pre-install step fails: %v\n%s", err, out)
		}
	}

	if _, err := os.Stat(filepath.Join(flash, current)); err != nil {
		t.Errorf("the pre-install steps delete %s, so a boot without network has nothing to install", current)
	}
	if _, err := os.Stat(filepath.Join(flash, older)); err == nil {
		t.Errorf("the pre-install steps keep the older %s on the flash", older)
	}
}
