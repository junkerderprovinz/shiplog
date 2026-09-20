// Package bannerlog prints the ShipLog startup banner and the readiness line.
package bannerlog

import (
	_ "embed"
	"fmt"
	"io"
)

// initArt is the ShipLog anchor in ASCII above the shared "Junker der Provinz"
// wordmark.
//
//go:embed banner.txt
var initArt string

// Init prints the ASCII banner and the app line at startup.
func Init(w io.Writer) {
	_, _ = fmt.Fprint(w, initArt)
	_, _ = fmt.Fprintln(w, "  ShipLog - read-only update intelligence for your Docker fleet")
	_, _ = fmt.Fprintln(w)
}

// Ready prints the readiness line once the HTTP listener is up. The smoke test
// in build.yml greps `docker logs` for "SHIPLOG IS READY"; changing the phrase
// means changing it there too.
func Ready(w io.Writer, addr string) {
	_, _ = fmt.Fprintf(w, "  \033[0;32m✓ SHIPLOG IS READY\033[0m - listening on http://%s\n", addr)
	_, _ = fmt.Fprintln(w)
}
