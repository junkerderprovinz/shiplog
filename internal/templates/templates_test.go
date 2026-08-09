package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectPages(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "my-Dozzle.xml",
		`<?xml version="1.0"?><Container version="2"><Name>Dozzle</Name><Project>https://github.com/amir20/dozzle</Project></Container>`)
	write(t, dir, "my-Plex.xml",
		`<?xml version="1.0"?><Container version="2"><Name>plex</Name><Project>https://www.plex.tv/</Project></Container>`)
	// Malformed, missing fields and non-XML entries must be skipped silently.
	write(t, dir, "my-Broken.xml", `<Container><Name>Broken`)
	write(t, dir, "my-NoProject.xml", `<Container version="2"><Name>NoProject</Name></Container>`)
	write(t, dir, "notes.txt", `not a template`)

	pages := ProjectPages(dir)
	if got := pages["dozzle"]; got != "https://github.com/amir20/dozzle" {
		t.Fatalf("dozzle = %q", got)
	}
	if got := pages["plex"]; got != "https://www.plex.tv/" {
		t.Fatalf("plex = %q (non-GitHub URLs are included; sources decides)", got)
	}
	if _, ok := pages["broken"]; ok {
		t.Fatal("malformed template must be skipped")
	}
	if _, ok := pages["noproject"]; ok {
		t.Fatal("template without <Project> must be skipped")
	}
}

func TestProjectPages_MissingDir(t *testing.T) {
	if pages := ProjectPages(filepath.Join(t.TempDir(), "nope")); pages != nil {
		t.Fatalf("missing dir must yield nil, got %v", pages)
	}
}

func TestTemplateURLs(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "my-OpenCloud.xml",
		`<?xml version="1.0"?><Container version="2"><Name>OpenCloud</Name><TemplateURL>https://raw.githubusercontent.com/junkerderprovinz/unraid-apps/main/opencloud/opencloud.xml</TemplateURL></Container>`)
	// A manually-created container carries no <TemplateURL> and must be skipped.
	write(t, dir, "my-Manual.xml", `<Container version="2"><Name>Manual</Name></Container>`)
	write(t, dir, "my-Broken.xml", `<Container><Name>Broken`)

	urls := TemplateURLs(dir)
	if got := urls["opencloud"]; got != "https://raw.githubusercontent.com/junkerderprovinz/unraid-apps/main/opencloud/opencloud.xml" {
		t.Fatalf("opencloud = %q", got)
	}
	if _, ok := urls["manual"]; ok {
		t.Fatal("template without <TemplateURL> must be skipped")
	}
	if _, ok := urls["broken"]; ok {
		t.Fatal("malformed template must be skipped")
	}
}

func TestTemplateURLs_MissingDir(t *testing.T) {
	if urls := TemplateURLs(filepath.Join(t.TempDir(), "nope")); urls != nil {
		t.Fatalf("missing dir must yield nil, got %v", urls)
	}
}
