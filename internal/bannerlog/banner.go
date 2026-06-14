// Package bannerlog prints the ShipLog init-log banner and the loud
// "<APP> IS READY" banner (house standard alongside the ASCII init banner).
package bannerlog

import (
	_ "embed"
	"fmt"
	"io"
)

// initArt is the house init banner: the ShipLog anchor in ASCII + the shared
// "Junker der Provinz" wordmark (same signature the other own-image repos print).
//
//go:embed banner.txt
var initArt string

// Init prints the ASCII init banner + the app line at startup.
func Init(w io.Writer) {
	_, _ = fmt.Fprint(w, initArt)
	_, _ = fmt.Fprintln(w, "   ShipLog  -  read-only update intelligence for your Docker fleet")
	_, _ = fmt.Fprintln(w)
}

// Ready prints a loud, unmistakable readiness banner once the HTTP listener is
// up. The exact phrase "SHIPLOG IS READY" is asserted by the CI boot smoke gate
// (grep it in `docker logs`), so do not reword it without updating build.yml.
func Ready(w io.Writer, addr string) {
	const bar = "==================================================================="
	_, _ = fmt.Fprintln(w, bar)
	_, _ = fmt.Fprintf(w, "   SHIPLOG IS READY  -  listening on http://%s\n", addr)
	_, _ = fmt.Fprintln(w, bar)
}
