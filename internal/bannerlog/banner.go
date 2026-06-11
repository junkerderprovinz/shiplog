// Package bannerlog prints the ShipLog init-log banner and the loud
// "<APP> IS READY" banner (house standard alongside the ASCII init banner).
package bannerlog

import (
	"fmt"
	"io"
)

const initArt = `
   _____ _     _       _
  / ____| |   (_)     | |
 | (___ | |__  _ _ __ | |     ___   __ _
  \___ \| '_ \| | '_ \| |    / _ \ / _' |
  ____) | | | | | |_) | |___| (_) | (_| |
 |_____/|_| |_|_| .__/|______\___/ \__, |
                | |                  __/ |
                |_|                 |___/
   anchor down  -  update intelligence for your docker fleet
`

// Init prints the ASCII init banner at startup.
func Init(w io.Writer) {
	fmt.Fprint(w, initArt)
}

// Ready prints a loud, unmistakable readiness banner once the HTTP listener is
// up. The exact phrase "SHIPLOG IS READY" is asserted by the CI boot smoke gate
// (grep it in `docker logs`), so do not reword it without updating build.yml.
func Ready(w io.Writer, addr string) {
	const bar = "==================================================================="
	fmt.Fprintln(w, bar)
	fmt.Fprintf(w, "   SHIPLOG IS READY  -  listening on http://%s\n", addr)
	fmt.Fprintln(w, bar)
}
