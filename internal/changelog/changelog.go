// Package changelog resolves the "what changed" payload for a container update.
// A Chain tries its providers in order; GitHub reads the releases of the
// container's source repo, and Fallback always answers with a bare version
// delta.
package changelog

import (
	"context"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Provider resolves a changelog for the span (fromTag, toTag] of a container.
// The bool reports whether this provider handled the request; an unhandled
// provider lets the Chain fall through to the next one.
type Provider interface {
	Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool)
}

// Chain tries each Provider in order and returns the first handled result.
type Chain []Provider

// Get returns the first handled changelog in the chain.
func (ch Chain) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	for _, p := range ch {
		if cl, ok := p.Get(ctx, c, fromTag, toTag); ok {
			return cl, true
		}
	}
	return nil, false
}

// semver is a major.minor.patch triple, kept here so that changelog does not
// depend on risk or resolver.
type semver struct{ major, minor, patch int }

func (v semver) compare(o semver) int {
	switch {
	case v.major != o.major:
		return sign(v.major - o.major)
	case v.minor != o.minor:
		return sign(v.minor - o.minor)
	default:
		return sign(v.patch - o.patch)
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

// parseSemver reads up to three numeric components from a tag, ignoring a
// leading "v" and any pre-release or build suffix. Missing components are 0.
func parseSemver(tag string) (semver, bool) {
	core := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core = core[:i]
	}
	if i := strings.IndexByte(core, '-'); i >= 0 {
		core = core[:i]
	}
	if core == "" {
		return semver{}, false
	}
	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	var v semver
	dst := []*int{&v.major, &v.minor, &v.patch}
	for i, p := range parts {
		if p == "" {
			return semver{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		*dst[i] = n
	}
	return v, true
}
