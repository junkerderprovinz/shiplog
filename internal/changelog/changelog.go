// Package changelog resolves the "what changed" payload for a container update.
//
// Providers are tried in order via a Chain: the first provider that reports it
// handled the request wins. The GitHub provider mines upstream releases from the
// repo named by the container's OCI source label; the Fallback provider always
// handles, producing a bare version-delta changelog so the engine never returns
// nothing for an update.
//
// It is a dependency-free unit: standard library only. The GitHub base URL is
// injectable so tests can route the API at an httptest server.
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

// Get returns the first handled changelog in the chain, or (nil, false) if no
// provider handled the request.
func (ch Chain) Get(ctx context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	for _, p := range ch {
		if cl, ok := p.Get(ctx, c, fromTag, toTag); ok {
			return cl, true
		}
	}
	return nil, false
}

// semver is a parsed major.minor.patch triple. A private copy lives here so the
// changelog unit stays decoupled from internal/risk and internal/resolver.
type semver struct{ major, minor, patch int }

// parseSemver strips a leading 'v', drops any '-prerelease'/'+build' suffix,
// and reads up to three numeric components (missing default to 0). It reports
// false if there are zero numeric components or any present one is non-numeric.
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
