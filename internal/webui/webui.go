// Package webui carries the page that signs in to the container and holds its
// sign-in settings. The Dockerfile builds it from ui/ into dist before the
// engine, so a build without that step serves an empty folder.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// Files is the built page.
func Files() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
