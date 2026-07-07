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
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Accept lists the manifest media types we ask for on a HEAD, covering both
// multi-arch indexes/lists and single-arch manifests, OCI and Docker flavors.
// All four are load-bearing: without an explicit Accept, Docker Hub falls back
// to the legacy schema1 manifest whose digest NEVER matches the pulled image's
// RepoDigests — every container would show a permanent phantom "digest drift".
// (HEADs on manifests also do not count against Docker Hub's pull rate limit,
// which is why the whole check pipeline is HEAD-only.)
const acceptManifests = "application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json"

// dockerHubRegistry is the canonical Docker Hub registry host (where docker.io
// images resolve) — the only host we ever attach Docker Hub credentials to.
const dockerHubRegistry = "registry-1.docker.io"

// ghcrHosts are the GitHub-backed registries whose anonymous token endpoint
// honours a GITHUB_TOKEN as HTTP Basic auth, raising the (otherwise sharply
// rate-limited) anonymous tags/list budget. lscr.io is LinuxServer's GHCR proxy
// and shares ghcr.io's token flow.
var ghcrHosts = map[string]bool{"ghcr.io": true, "lscr.io": true}

// Retry/throttle tuning. A sweep over ~55 containers bursts tags/list at one
// registry host (ghcr.io / lscr.io), which rate-limits anonymous reads with a
// 429. We retry the transient statuses a couple of times with backoff that
// honours Retry-After, and serialise per host with a tiny gap so we never burst.
const (
	maxRetries   = 3                      // attempts beyond the first on 429/503
	baseBackoff  = 500 * time.Millisecond // first backoff; doubles each retry
	maxBackoff   = 8 * time.Second        // cap so a sweep can't stall on one host
	hostMinGap   = 150 * time.Millisecond // minimum spacing between requests per host
	maxRetryWait = 30 * time.Second       // ignore an absurd Retry-After
)

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

	// Optional GitHub token. When set it is sent as HTTP Basic auth (any
	// username, the token as password) on the token request for ghcr.io / lscr.io,
	// which raises the anonymous tags/list rate limit those hosts enforce.
	ghToken string

	// gate serialises requests per registry host with a tiny minimum gap, so a
	// concurrent sweep over many containers on the same host never bursts.
	gate *hostGate

	// tokens caches bearer tokens per host|repo (Docker Hub tokens are
	// repo-scoped) until shortly before their advertised expiry, so a sweep
	// doesn't pay a 401 probe + token roundtrip for every single request.
	tokensMu sync.Mutex
	tokens   map[string]cachedToken

	// hostDown tracks a per-host circuit breaker: a 429 that survives the
	// per-request retries silences that host for an exponentially growing
	// window (1m..1h) instead of letting every remaining container of the
	// sweep run into the same rate limit. Any success clears it.
	hostDownMu sync.Mutex
	hostDown   map[string]*hostBackoff

	// sleep is the (injectable) wait used for backoff; tests stub it to run fast.
	sleep func(context.Context, time.Duration)

	// now is injectable so tests can control token expiry and breaker windows.
	now func() time.Time
}

// cachedToken is a bearer token with its use-by time.
type cachedToken struct {
	token string
	until time.Time
}

// hostBackoff is the circuit-breaker state for one registry host.
type hostBackoff struct {
	fails int
	until time.Time
}

// Circuit-breaker tuning: first trip mutes a host for breakerBase, doubling per
// consecutive trip up to breakerMax.
const (
	breakerBase = time.Minute
	breakerMax  = time.Hour
)

// New returns a Resolver with a sane default HTTP client and an HTTPS baseURL.
func New() *Resolver {
	return &Resolver{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    func(host string) string { return "https://" + host },
		gate:       newHostGate(hostMinGap),
		tokens:     map[string]cachedToken{},
		hostDown:   map[string]*hostBackoff{},
		sleep:      sleepCtx,
		now:        time.Now,
	}
}

// WithDockerHubAuth sets optional Docker Hub credentials (returns r for chaining).
// Empty values leave anonymous access unchanged.
func (r *Resolver) WithDockerHubAuth(user, token string) *Resolver {
	r.dhUser, r.dhToken = user, token
	return r
}

// WithGitHubToken sets the optional GitHub token used to authenticate against
// ghcr.io / lscr.io (returns r for chaining). An empty value leaves anonymous
// access unchanged.
func (r *Resolver) WithGitHubToken(token string) *Resolver {
	r.ghToken = token
	return r
}

// hostGate enforces a minimum gap between requests to the same registry host.
type hostGate struct {
	mu   sync.Mutex
	last map[string]time.Time
	gap  time.Duration
}

func newHostGate(gap time.Duration) *hostGate {
	return &hostGate{last: map[string]time.Time{}, gap: gap}
}

// wait blocks until at least gap has elapsed since the previous request to host,
// or until ctx is cancelled. It reserves the slot before sleeping so concurrent
// callers for the same host queue rather than all firing at once.
func (g *hostGate) wait(ctx context.Context, host string) {
	if g == nil || g.gap <= 0 {
		return
	}
	g.mu.Lock()
	now := time.Now()
	next := now
	if t, ok := g.last[host]; ok && t.After(now) {
		next = t
	}
	g.last[host] = next.Add(g.gap)
	delay := next.Sub(now)
	g.mu.Unlock()
	if delay > 0 {
		sleepCtx(ctx, delay)
	}
}

// breakerRemaining returns how long host is still muted, or 0 when it may be
// queried.
func (r *Resolver) breakerRemaining(host string) time.Duration {
	r.hostDownMu.Lock()
	defer r.hostDownMu.Unlock()
	if b, ok := r.hostDown[host]; ok {
		if wait := b.until.Sub(r.now()); wait > 0 {
			return wait
		}
	}
	return 0
}

// noteHostOutcome feeds the circuit breaker: a final 429 (already past the
// per-request retries in do) trips or extends it, any success clears it. Other
// statuses leave it untouched.
func (r *Resolver) noteHostOutcome(host string, status int) {
	r.hostDownMu.Lock()
	defer r.hostDownMu.Unlock()
	switch {
	case status == http.StatusTooManyRequests:
		b := r.hostDown[host]
		if b == nil {
			b = &hostBackoff{}
			r.hostDown[host] = b
		}
		// In-flight workers hitting the same rate-limit burst must count as ONE
		// trip, or three workers would jump the window straight to 4m.
		if b.until.After(r.now()) {
			return
		}
		b.fails++
		window := breakerBase << (b.fails - 1)
		if window > breakerMax || window <= 0 {
			window = breakerMax
		}
		b.until = r.now().Add(window)
	case status >= 200 && status < 300:
		delete(r.hostDown, host)
	}
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
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

	// A tripped breaker answers without any network: once a host hard-429s, the
	// rest of the sweep must not pile onto it. The engine carries prior verdicts
	// forward on errors, so badges stay put while we wait the window out.
	// No countdown in the message: the row may sit unrefreshed until the next
	// sweep, and a stored "retry in 43s" would read wrong for hours.
	if r.breakerRemaining(host) > 0 {
		return "", "", "", "", fmt.Errorf("%s is rate limiting us — backing off before the next attempt", host)
	}

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
		r.noteHostOutcome(host, resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			// A 429 that survives the retries is the registry rate-limiting our
			// anonymous reads; say so plainly so the engine can keep the last-known
			// result (and the user knows a token would help) instead of "unknown".
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, fmt.Errorf("tags/list: rate limited (429) for %s — set GITHUB_TOKEN for ghcr.io/lscr.io to raise the limit", pathRepo)
			}
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
	r.noteHostOutcome(host, resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("manifests/%s: unexpected status %d", tag, resp.StatusCode)
	}
	return resp.Header.Get("Docker-Content-Digest"), nil
}

type header struct{ key, value string }

// doAuthed performs the request and, on a 401 carrying a Bearer challenge,
// fetches an anonymous (or token-backed) bearer and retries once with
// Authorization set. Transient rate-limit/unavailable statuses are retried with
// backoff inside do.
//
// A valid cached token for host|repo is sent preemptively, so steady-state
// sweeps skip the 401 probe + token roundtrip entirely; a 401 despite the
// cached token (expired early, revoked) invalidates it and falls back to the
// normal challenge flow, which re-fills the cache.
func (r *Resolver) doAuthed(ctx context.Context, method, url, host, pathRepo string, extra ...header) (*http.Response, error) {
	cacheKey := host + "|" + pathRepo
	resp, err := r.do(ctx, method, url, host, r.cachedToken(cacheKey), extra...)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	r.dropToken(cacheKey)

	challenge := resp.Header.Get("WWW-Authenticate")
	_ = resp.Body.Close()
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return nil, fmt.Errorf("401 without bearer challenge for %s", url)
	}

	token, ttl, err := r.fetchToken(ctx, challenge, pathRepo, host)
	if err != nil {
		return nil, err
	}
	r.storeToken(cacheKey, token, ttl)
	return r.do(ctx, method, url, host, token, extra...)
}

// cachedToken returns the still-valid token for key, or "".
func (r *Resolver) cachedToken(key string) string {
	r.tokensMu.Lock()
	defer r.tokensMu.Unlock()
	if t, ok := r.tokens[key]; ok && r.now().Before(t.until) {
		return t.token
	}
	return ""
}

// storeToken caches token for key until shortly before its advertised expiry.
func (r *Resolver) storeToken(key, token string, ttl time.Duration) {
	// Registries advertise expires_in only sometimes; the spec default is 60s
	// but Docker Hub/ghcr issue 300s tokens. Use at least the spec-typical 300s
	// floor UnraidDeck also relies on, minus a safety margin.
	if ttl < 300*time.Second {
		ttl = 300 * time.Second
	}
	ttl -= 60 * time.Second
	r.tokensMu.Lock()
	defer r.tokensMu.Unlock()
	r.tokens[key] = cachedToken{token: token, until: r.now().Add(ttl)}
}

// dropToken invalidates the cached token for key.
func (r *Resolver) dropToken(key string) {
	r.tokensMu.Lock()
	defer r.tokensMu.Unlock()
	delete(r.tokens, key)
}

// do issues a request (optionally with a bearer token), gating per host and
// retrying transient rate-limit (429) / unavailable (503) responses with
// bounded exponential backoff that honours Retry-After. The last response is
// returned even when it's still a 429/503, so the caller can surface it.
func (r *Resolver) do(ctx context.Context, method, url, host, token string, extra ...header) (*http.Response, error) {
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		r.gate.wait(ctx, host)
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
		resp, err = r.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if !isTransient(resp.StatusCode) || attempt >= maxRetries {
			return resp, nil
		}
		// Transient and we have a retry left: honour Retry-After, else back off.
		wait := retryAfter(resp.Header.Get("Retry-After"), backoff(attempt))
		_ = resp.Body.Close()
		r.sleep(ctx, wait)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
}

// isTransient reports whether a status is worth retrying: 429 (rate limited) or
// 503 (temporarily unavailable).
func isTransient(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable
}

// backoff returns the exponential delay for a zero-based attempt with a small
// jitter, capped at maxBackoff.
func backoff(attempt int) time.Duration {
	d := baseBackoff << attempt
	if d > maxBackoff {
		d = maxBackoff
	}
	return d + time.Duration(rand.Int63n(int64(baseBackoff)))
}

// retryAfter parses a Retry-After header (delta-seconds or HTTP-date) and clamps
// it to maxRetryWait; it falls back to def when absent or unparseable.
func retryAfter(h string, def time.Duration) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return def
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return clampWait(time.Duration(secs) * time.Second)
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return clampWait(d)
		}
		return 0
	}
	return def
}

// clampWait bounds a wait to [0, maxRetryWait].
func clampWait(d time.Duration) time.Duration {
	if d > maxRetryWait {
		return maxRetryWait
	}
	if d < 0 {
		return 0
	}
	return d
}

// fetchToken parses a Bearer WWW-Authenticate challenge, GETs the realm with
// the advertised service and an enforced repository:<repo>:pull scope, and
// returns the token plus its advertised lifetime (0 when the realm sends no
// expires_in). It attaches HTTP Basic auth so the issued token carries the
// authenticated (higher) rate limit, host-scoped: Docker Hub credentials only on
// registry-1.docker.io, and a GitHub token (any user, token as password) only on
// ghcr.io / lscr.io. Credentials are never sent to any other registry.
func (r *Resolver) fetchToken(ctx context.Context, challenge, pathRepo, host string) (string, time.Duration, error) {
	params := parseChallenge(challenge)
	realm := params["realm"]
	if realm == "" {
		return "", 0, fmt.Errorf("bearer challenge missing realm: %q", challenge)
	}

	sep := "?"
	if strings.Contains(realm, "?") {
		sep = "&"
	}
	url := realm + sep + "service=" + queryEscape(params["service"]) +
		"&scope=" + queryEscape("repository:"+pathRepo+":pull")

	var extra []header
	switch {
	case host == dockerHubRegistry && r.dhUser != "" && r.dhToken != "":
		cred := base64.StdEncoding.EncodeToString([]byte(r.dhUser + ":" + r.dhToken))
		extra = append(extra, header{"Authorization", "Basic " + cred})
	case ghcrHosts[host] && r.ghToken != "":
		// ghcr.io accepts the PAT as the password with any username.
		cred := base64.StdEncoding.EncodeToString([]byte("x:" + r.ghToken))
		extra = append(extra, header{"Authorization", "Basic " + cred})
	}

	resp, err := r.do(ctx, http.MethodGet, url, host, "", extra...)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// A rate-limited TOKEN realm must trip the registry's breaker too:
		// auth.docker.io/ghcr token throttling is the most common 429 source,
		// and without this the sweep would keep re-hammering it per container.
		if resp.StatusCode == http.StatusTooManyRequests {
			r.noteHostOutcome(host, resp.StatusCode)
		}
		return "", 0, fmt.Errorf("token: unexpected status %d from realm %s", resp.StatusCode, realm)
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", 0, fmt.Errorf("token: decode: %w", err)
	}
	ttl := time.Duration(body.ExpiresIn) * time.Second
	if body.Token != "" {
		return body.Token, ttl, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, ttl, nil
	}
	return "", 0, fmt.Errorf("token: empty token from realm %s", realm)
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
