// Package updater applies the newest image for a container by recreating it
// identically. On Unraid the host daemon calls Unraid's OWN native update path
// (the exact code the "apply update" button runs); everywhere else it is a no-op
// (the generic ghcr container stays a read-only advisor).
package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Updater applies the newest image for one container, recreating it identically.
type Updater interface {
	// Update pulls the container's newest image and recreates it from its Unraid
	// user template. Returns nil on a successful recreate; an error leaves the
	// container running on its old image (never half-updated).
	Update(ctx context.Context, name string) error
	// Supported reports whether this host can perform updates (Unraid template dir
	// present). The generic container returns false so the executor no-ops.
	Supported() bool
}

// ErrUnsupported is returned by a Noop updater's Update.
var ErrUnsupported = errors.New("auto-update requires the Unraid plugin (no template dir)")

const (
	unraidTemplateDir = "/boot/config/plugins/dockerMan/templates-user"
	// updateScript is Unraid's own container-update entrypoint — the exact code the
	// native "apply update" button runs: it reads the user template my-<Name>.xml,
	// pulls the newest image, and recreates the container identically (preserving
	// env/ports/paths/network and the load-bearing net.unraid.docker.* labels).
	// Stable since Unraid 6.10; ShipLog is on 6.12+/7.x.
	updateScript = "/usr/local/emhttp/plugins/dynamix.docker.manager/scripts/update_container"
)

// Noop performs no updates. Used by the generic container and in tests.
type Noop struct{}

func (Noop) Update(context.Context, string) error { return ErrUnsupported }
func (Noop) Supported() bool                      { return false }

// Unraid applies updates via Unraid's native container-update script.
type Unraid struct{}

// Supported reports whether the Unraid user-template dir exists (i.e. this is the
// Unraid plugin, not the generic container).
func (Unraid) Supported() bool {
	fi, err := os.Stat(unraidTemplateDir)
	return err == nil && fi.IsDir()
}

// Update invokes Unraid's native container-update path for one container, headless
// and as root — identical to clicking "apply update". The caller only invokes this
// for a container it has already determined has an eligible update (the script
// itself pulls+recreates unconditionally). Success is the script's exit code; a
// non-zero exit (or missing template/script) is returned as an error and the
// container is left on its old image. It must NOT pre-stop the container — the
// script auto-detects and preserves the running state.
func (Unraid) Update(ctx context.Context, name string) error {
	// A clean error instead of a silent no-op when the user template is gone.
	tmpl := filepath.Join(unraidTemplateDir, "my-"+name+".xml")
	if _, err := os.Stat(tmpl); err != nil {
		return fmt.Errorf("updater: no Unraid template for %q (%s): %w", name, tmpl, err)
	}
	if _, err := os.Stat(updateScript); err != nil {
		return fmt.Errorf("updater: Unraid update script not found (%s): %w", updateScript, err)
	}
	// name is argv[1], url-encoded because the script rawurldecodes then splits on
	// '*' (its multi-container delimiter).
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, updateScript, url.QueryEscape(name))
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if msg := firstNonEmpty(strings.TrimSpace(errb.String()), strings.TrimSpace(out.String())); msg != "" {
			return fmt.Errorf("updater: update %q failed: %w (%s)", name, err, truncate(msg, 300))
		}
		return fmt.Errorf("updater: update %q failed: %w", name, err)
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
