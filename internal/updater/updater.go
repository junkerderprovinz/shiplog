// Package updater applies the newest image for a container through Unraid's
// own update path, the one behind the "apply update" button. Elsewhere it does
// nothing, and the container image stays a read-only advisor.
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
	"unicode/utf8"
)

// Updater applies the newest image for one container, recreating it identically.
type Updater interface {
	// Update pulls the newest image and recreates the container from its Unraid
	// user template, or creates it from there when no such container exists. It
	// returns what the update printed, since success alone does not prove the
	// container survived.
	Update(ctx context.Context, name string) (string, error)
	// Supported reports whether this host can apply updates.
	Supported() bool
}

// ErrUnsupported is returned by a Noop updater's Update.
var ErrUnsupported = errors.New("auto-update requires the Unraid plugin (no template dir)")

const (
	unraidTemplateDir = "/boot/config/plugins/dockerMan/templates-user"
	// updateScript recreates a container from my-<Name>.xml with the newest image
	// and keeps its settings and net.unraid.docker.* labels. It has been stable
	// since Unraid 6.10.
	updateScript = "/usr/local/emhttp/plugins/dynamix.docker.manager/scripts/update_container"
	// outputLimit keeps a log line readable. The end of the output is kept, since
	// the script ends with what went wrong.
	outputLimit = 4 << 10
)

// Noop performs no updates. Used by the generic container and in tests.
type Noop struct{}

func (Noop) Update(context.Context, string) (string, error) { return "", ErrUnsupported }
func (Noop) Supported() bool                                { return false }

// Unraid applies updates via Unraid's native container-update script.
type Unraid struct{}

// Supported reports whether the Unraid user-template dir exists.
func (Unraid) Supported() bool {
	fi, err := os.Stat(unraidTemplateDir)
	return err == nil && fi.IsDir()
}

// Update runs Unraid's update script for one container. The script pulls and
// recreates unconditionally, so callers decide eligibility first. The container
// is not stopped beforehand, because the script keeps its running state.
func (Unraid) Update(ctx context.Context, name string) (string, error) {
	tmpl := filepath.Join(unraidTemplateDir, "my-"+name+".xml")
	if _, err := os.Stat(tmpl); err != nil {
		return "", fmt.Errorf("updater: no Unraid template for %q (%s): %w", name, tmpl, err)
	}
	if _, err := os.Stat(updateScript); err != nil {
		return "", fmt.Errorf("updater: Unraid update script not found (%s): %w", updateScript, err)
	}
	// The script rawurldecodes its argument and then splits it on '*'.
	out, err := run(ctx, updateScript, url.QueryEscape(name))
	if err != nil {
		if out != "" {
			return out, fmt.Errorf("updater: update %q failed: %w (%s)", name, err, tail(out, 300))
		}
		return out, fmt.Errorf("updater: update %q failed: %w", name, err)
	}
	return out, nil
}

// run executes a command and returns the end of its stdout and stderr,
// interleaved as they were written.
func run(ctx context.Context, name string, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return tail(strings.TrimSpace(out.String()), outputLimit), err
}

// tail keeps the last n bytes of s without splitting a character.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	for s != "" && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return "…" + s
}
