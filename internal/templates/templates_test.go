package templates

import (
	"os"
	"path/filepath"
	"strings"
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

const nextcloudTemplate = `<?xml version="1.0"?>
<Container version="2">
  <Name>Nextcloud</Name>
  <Repository>lscr.io/linuxserver/nextcloud:latest</Repository>
  <Registry>https://hub.docker.com/r/linuxserver/nextcloud/</Registry>
  <Config Name="Repository mirror" Target="/x" Default="" Mode="rw" Type="Path">/mnt/user/x</Config>
</Container>
`

func TestRepositoryReadsTheTemplatesImage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "my-Nextcloud.xml", nextcloudTemplate)
	got, err := Folder(dir).Repository("Nextcloud")
	if err != nil || got != "lscr.io/linuxserver/nextcloud:latest" {
		t.Fatalf("Repository = %q, %v", got, err)
	}
	if _, err := Folder(dir).Repository("Missing"); err == nil {
		t.Fatal("a container without a template must be an error")
	}
}

func TestSetRepositoryChangesOnlyTheImage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "my-Nextcloud.xml", nextcloudTemplate)
	const ref = "lscr.io/linuxserver/nextcloud:latest@sha256:aaaa"
	if err := Folder(dir).SetRepository("Nextcloud", ref); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "my-Nextcloud.xml"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(nextcloudTemplate, "nextcloud:latest</Repository>", "nextcloud:latest@sha256:aaaa</Repository>", 1)
	if string(got) != want {
		t.Errorf("template =\n%s\nwant\n%s", got, want)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("the directory holds %d files, want the template alone", len(entries))
	}
}

func TestSetRepositoryEscapesTheReference(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "my-Web.xml", `<Container><Name>Web</Name><Repository>nginx</Repository></Container>`)
	if err := Folder(dir).SetRepository("Web", "a&b<c"); err != nil {
		t.Fatal(err)
	}
	if got, err := Folder(dir).Repository("Web"); err != nil || got != "a&b<c" {
		t.Fatalf("Repository after setting = %q, %v", got, err)
	}
}

func TestSetRepositoryRefusesATemplateWithoutAnImage(t *testing.T) {
	dir := t.TempDir()
	const plain = `<Container><Name>Web</Name></Container>`
	write(t, dir, "my-Web.xml", plain)
	if err := Folder(dir).SetRepository("Web", "nginx"); err == nil {
		t.Fatal("want an error")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "my-Web.xml")); string(got) != plain {
		t.Errorf("template changed to %s", got)
	}
}

func TestTemplateURLs_MissingDir(t *testing.T) {
	if urls := TemplateURLs(filepath.Join(t.TempDir(), "nope")); urls != nil {
		t.Fatalf("missing dir must yield nil, got %v", urls)
	}
}
