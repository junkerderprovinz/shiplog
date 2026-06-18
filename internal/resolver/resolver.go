// Package resolver queries an OCI registry (Distribution v2 API) to find, for a
// given image reference, the newest available tag (by semver) and the current
// digest behind a fixed tag (to detect a moved :latest).
//
// It is a thin, dependency-free unit: only the standard library. The base URL
// per registry host is injectable so tests can point the whole flow at an
// httptest server.
package resolver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Accept lists the manifest media types we ask for on a HEAD, covering both
// multi-arch indexes/lists and single-arch manifests, OCI and Docker flavors.
const acceptManifests = "application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json"

// dockerHubRegistry is the canonical Docker Hub registry host (where docker.io
// images resolve) — the only host we ever attach Docker Hub credentials to.
const dockerHubRegistry = "registry-1.docker.io"

// Resolver talks to OCI registries to resolve newest tag + same-tag digest.
type Resolver struct {
	httpClient *http.Client
	// baseURL maps a registry host to the scheme+host base used for requests,
	// e.g. "registry-1.docker.io" -> "https://registry-1.docker.io". Injectable
	// so tests can route every host to a fake registry.
	baseURL func(host string) string

	// Optional Docker Hub credentials. When set, the token request for a
	// docker.io image carries HTTP Basic auth so the issued bearer token has the
	// authenticated (higher) rate limit. Never sent to any other registry.
	dhUser  string
	dhToken string
}

// New returns a Resolver with a sane default HTTP client and an HTTPS baseURL.
func New() *Resolver {
	return &Resolver{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    func(host string) string { return "https://" + host },
	}
}

// WithDockerHubAuth sets optional Docker Hub credentials (returns r for chaining).
// Empty values leave anonymous access unchanged.
func (r *Resolver) WithDockerHubAuth(user, token string) *Resolver {
	r.dhUser, r.dhToken = user, token
	return r
}

// Resolve inspects an image's upstream registry and reports:
//
//   - newestTag: the highest semver tag, or the requested tag echoed back when
//     that tag isn't semver (e.g. "latest") so the caller still gets digest-drift
//     detection for it;
//   - sameTagDigest: the digest currently behind the requested tag;
//   - newestVerTag: the highest semver tag in the repo regardless of the
//     requested tag ("" when the repo has no semver tags);
//   - newestVerDigest: the digest of newestVerTag, used to PROVE that a rolling
//     (":latest") container is actually running the newest version ("" when not
//     resolved). It's only fetched for non-semver requested tags, since a pinned
//     semver tag is already its own version.
//
// curDigest is accepted for API symmetry with the engine but isn't needed here.
func (r *Resolver) Resolve(ctx context.Context, repo, tag, curDigest string) (newestTag, sameTagDigest, newestVerTag, newestVerDigest string, err error) {
	host, pathRepo := splitRepo(repo)
	base := r.baseURL(host)

	tags, err := r.fetchTags(ctx, base, host, pathRepo)
	if err != nil {
		return "", "", "", "", err
	}

	newestVerTag = newestSemver(tags)
	newestTag = newestVerTag
	if !isSemver(tag) || newestTag == "" {
		// Non-semver requested tag: we can't rank it; report it as-is so the
		// caller still gets digest-drift detection for it.
		newestTag = tag
	}

	sameTagDigest, err = r.fetchDigest(ctx, base, host, pathRepo, tag)
	if err != nil {
		return "", "", "", "", err
	}

	// For a rolling tag, resolve the newest version's own digest so the engine
	// can confirm by digest which version the running image is. Best-effort: an
	// error just leaves it empty, which disables the proof (no wrong label).
	if !isSemver(tag) && newestVerTag != "" {
		if d, derr := r.fetchDigest(ctx, base, host, pathRepo, newestVerTag); derr == nil {
			newestVerDigest = d
		}
	}
	return newestTag, sameTagDigest, newestVerTag, newestVerDigest, nil
}

// splitRepo splits a normalized repo into the registry host and the path-repo.
//
//	"docker.io/library/redis" -> "registry-1.docker.io", "library/redis"
//	"ghcr.io/x/y"             -> "ghcr.io",              "x/y"
//	"host/ns/name"            -> "host",                 "ns/name"
func splitRepo(repo string) (host, pathRepo string) {
	first := strings.IndexByte(repo, '/')
	if first < 0 {
		// No host segment; treat the whole thing as a Docker Hub library image.
		return "registry-1.docker.io", "library/" + repo
	}
	host = repo[:first]
	pathRepo = repo[first+1:]
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}
	return host, pathRepo
}

// fetchTags GETs /v2/<repo>/tags/list, handling the anonymous bearer flow and
// FOLLOWING pagination (the `Link: …; rel="next"` header). Registries with many
// tags page the list; reading only the first page picked a stale "newest" (e.g.
// OpenHands has hundreds of tags and the 1.x ones were on a later page).
func (r *Resolver) fetchTags(ctx context.Context, base, host, pathRepo string) ([]string, error) {
	var all []string
	url := base + "/v2/" + pathRepo + "/tags/list?n=500"
	for page := 0; page < 100; page++ { // hard cap against a misbehaving registry
		resp, err := r.doAuthed(ctx, http.MethodGet, url, host, pathRepo)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("tags/list: unexpected status %d for %s", resp.StatusCode, pathRepo)
		}
		var body struct {
			Tags []string `json:"tags"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		link := resp.Header.Get("Link")
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("tags/list: decode: %w", err)
		}
		all = append(all, body.Tags...)

		next := nextLink(link, base)
		if next == "" {
			break
		}
		url = next
	}
	return all, nil
}

// nextLink extracts the rel="next" target from a Link header, resolved against
// base (registries usually return a root-relative path). Returns "" if none.
func nextLink(linkHeader, base string) string {
	for _, part := range strings.Split(linkHeader, ",") {
		if !strings.Contains(part, `rel="next"`) && !strings.Contains(part, "rel=next") {
			continue
		}
		lt := strings.IndexByte(part, '<')
		gt := strings.IndexByte(part, '>')
		if lt < 0 || gt <= lt {
			continue
		}
		u := strings.TrimSpace(part[lt+1 : gt])
		switch {
		case strings.HasPrefix(u, "http"):
			return u
		case strings.HasPrefix(u, "/"):
			return base + u
		default:
			return base + "/" + u
		}
	}
	return ""
}

// fetchDigest HEADs the manifest for tag and returns its Docker-Content-Digest.
func (r *Resolver) fetchDigest(ctx context.Context, base, host, pathRepo, tag string) (string, error) {
	url := base + "/v2/" + pathRepo + "/manifests/" + tag
	resp, err := r.doAuthed(ctx, http.MethodHead, url, host, pathRepo,
		header{"Accept", acceptManifests})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("manifests/%s: unexpected status %d", tag, resp.StatusCode)
	}
	return resp.Header.Get("Docker-Content-Digest"), nil
}

type header struct{ key, value string }

// doAuthed performs the request and, on a 401 carrying a Bearer challenge,
// fetches an anonymous token and retries once with Authorization set.
func (r *Resolver) doAuthed(ctx context.Context, method, url, host, pathRepo string, extra ...header) (*http.Response, error) {
	resp, err := r.do(ctx, method, url, "", extra...)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	challenge := resp.Header.Get("WWW-Authenticate")
	_ = resp.Body.Close()
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return nil, fmt.Errorf("401 without bearer challenge for %s", url)
	}

	token, err := r.fetchToken(ctx, challenge, pathRepo, host)
	if err != nil {
		return nil, err
	}
	return r.do(ctx, method, url, token, extra...)
}

// do issues a single request, optionally with a bearer token.
func (r *Resolver) do(ctx context.Context, method, url, token string, extra ...header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	for _, h := range extra {
		req.Header.Set(h.key, h.value)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return r.httpClient.Do(req)
}

// fetchToken parses a Bearer WWW-Authenticate challenge, GETs the realm with
// the advertised service and an enforced repository:<repo>:pull scope, and
// returns the token. For Docker Hub (host == registry-1.docker.io) it attaches
// HTTP Basic auth from the configured credentials, so the issued token carries
// the authenticated (higher) rate limit; credentials are never sent elsewhere.
func (r *Resolver) fetchToken(ctx context.Context, challenge, pathRepo, host string) (string, error) {
	params := parseChallenge(challenge)
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("bearer challenge missing realm: %q", challenge)
	}

	sep := "?"
	if strings.Contains(realm, "?") {
		sep = "&"
	}
	url := realm + sep + "service=" + queryEscape(params["service"]) +
		"&scope=" + queryEscape("repository:"+pathRepo+":pull")

	var extra []header
	if host == dockerHubRegistry && r.dhUser != "" && r.dhToken != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(r.dhUser + ":" + r.dhToken))
		extra = append(extra, header{"Authorization", "Basic " + cred})
	}

	resp, err := r.do(ctx, http.MethodGet, url, "", extra...)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token: unexpected status %d from realm %s", resp.StatusCode, realm)
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("token: decode: %w", err)
	}
	if body.Token != "" {
		return body.Token, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, nil
	}
	return "", fmt.Errorf("token: empty token from realm %s", realm)
}

// parseChallenge parses the key="value" pairs out of a Bearer challenge value,
// e.g. `Bearer realm="https://auth",service="reg",scope="repository:x:pull"`.
func parseChallenge(challenge string) map[string]string {
	out := map[string]string{}
	rest := strings.TrimSpace(challenge)
	if i := strings.IndexByte(rest, ' '); i >= 0 {
		rest = rest[i+1:] // drop the "Bearer" scheme word
	}
	for _, part := range splitChallengeParts(rest) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		out[key] = val
	}
	return out
}

// splitChallengeParts splits on commas that are not inside double quotes.
func splitChallengeParts(s string) []string {
	var parts []string
	var b strings.Builder
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
			b.WriteByte(c)
		case c == ',' && !inQuotes:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}

// queryEscape percent-encodes a query parameter value (stdlib net/url avoided
// here is unnecessary; we keep it minimal but correct for the chars at play).
func queryEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == ':', c == '/':
			b.WriteByte(c)
		default:
			b.WriteString("%")
			b.WriteString(strings.ToUpper(strconv.FormatInt(int64(c), 16)))
		}
	}
	return b.String()
}

// newestSemver returns the highest semver tag (by numeric major.minor.patch),
// ignoring tags that aren't semver. Returns "" if none qualify.
func newestSemver(tags []string) string {
	var best string
	var bestV [3]int
	for _, t := range tags {
		v, ok := parseSemver(t)
		if !ok {
			continue
		}
		if best == "" || less(bestV, v) {
			best, bestV = t, v
		}
	}
	return best
}

// less reports whether a < b in major.minor.patch order.
func less(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// isSemver reports whether t parses as numeric major.minor.patch (v-prefix ok).
func isSemver(t string) bool {
	_, ok := parseSemver(t)
	return ok
}

// parseSemver strips a leading 'v' and parses a numeric version. It accepts
// MAJOR.MINOR and MAJOR.MINOR.PATCH (a missing patch is 0): some projects publish
// two-part tags only (e.g. OpenHands "0.18"), and requiring exactly three parts
// hid the newest release. Tags with a non-numeric part (e.g. "1.8.0-rc1") stay
// rejected, so a prerelease never ranks as the newest tag.
func parseSemver(t string) ([3]int, bool) {
	var v [3]int
	t = strings.TrimPrefix(strings.TrimSpace(t), "v")
	parts := strings.Split(t, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}
