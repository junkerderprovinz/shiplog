// Package updater applies the newest image for a container by recreating it
// identically. On Unraid the host daemon uses the native update path (pull +
// recreate from the user template my-<name>.xml); everywhere else it is a no-op
// (the generic ghcr container stays a read-only advisor).
package updater

import (
	"context"
	"errors"
	"os"
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

// ErrSpikePending marks the not-yet-resolved on-box trigger. Task 1 of the plan
// (the on-box spike) replaces Unraid.Update's body with the real Unraid hook;
// until then it returns this so the wiring compiles and the executor is fully
// testable against the interface.
var ErrSpikePending = errors.New("updater: Unraid native update hook not yet wired (on-box spike)")

// unraidTemplateDir is where Unraid stores per-container user templates.
const unraidTemplateDir = "/boot/config/plugins/dockerMan/templates-user"

// Noop performs no updates. Used by the generic container and in tests.
type Noop struct{}

func (Noop) Update(context.Context, string) error { return ErrUnsupported }
func (Noop) Supported() bool                      { return false }

// Unraid applies updates via Unraid's native container-update path.
type Unraid struct{}

// Supported reports whether the Unraid user-template dir exists (i.e. this is the
// Unraid plugin, not the generic container).
func (Unraid) Supported() bool {
	fi, err := os.Stat(unraidTemplateDir)
	return err == nil && fi.IsDir()
}

// Update is a stub until the on-box spike (plan Task 1) wires the real hook.
func (Unraid) Update(context.Context, string) error { return ErrSpikePending }
