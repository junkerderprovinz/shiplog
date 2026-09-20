// Package resolver asks an OCI registry (Distribution v2 API) for the newest
// semver tag of an image and the digest currently behind its tag, which shows
// a moved :latest.
package resolver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrRepoNotFound means the registry answered 404 for the repository, so the
// image was deleted upstream rather than failing transiently.
var ErrRepoNotFound = errors.New("image repository not found in the registry")

// acceptManifests covers OCI and Docker indexes and manifests. Without it,
// Docker Hub serves the legacy schema1 manifest, whose digest never matches the
// pulled image and would show every container as drifted. Manifest HEADs do not
// count against Docker Hub's pull limit.
const acceptManifests = "application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json"

// dockerHubRegistry is the only host that receives Docker Hub credentials.
const dockerHubRegistry = "registry-1.docker.io"

// ghcrHosts accept a GitHub token as Basic auth on their token endpoint, which
// raises the tight anonymous limit. lscr.io is LinuxServer's proxy for ghcr.io.
var ghcrHosts = map[string]bool{"ghcr.io": true, "lscr.io": true}

// A sweep bursts tags/list requests at a single host, which answers anonymous
// bursts with 429, so requests are spaced per host and transient statuses are
// retried with backoff.
const (
	maxRetries   = 3                      // attempts beyond the first on 429/503
	baseBackoff  = 500 * time.Millisecond // first backoff; doubles each retry
	maxBackoff   = 8 * time.Second        // cap so a sweep can't stall on one host
	hostMinGap   = 150 * time.Millisecond // minimum spacing between requests per host
	maxRetryWait = 30 * time.Second       // ignore an absurd Retry-After
)

// Resolver resolves tags and digests against OCI registries.
type Resolver struct {
	httpClient *http.Client
	baseURL    func(host string) string

	dhUser  string
	dhToken string
	ghToken string

	gate *hostGate

	// tokens caches bearer tokens per host|repo, since Docker Hub scopes them to
	// a repo, so a sweep skips the 401 probe on most requests.
	tokensMu sync.Mutex
	tokens   map[string]cachedToken

	// hostDown is a per-host circuit breaker: a 429 that survives the retries
	// mutes the host for a growing window, so the rest of the sweep does not run
	// into the same limit. Any success clears it.
	hostDownMu sync.Mutex
	hostDown   map[string]*hostBackoff

	sleep func(context.Context, time.Duration)
	now   func() time.Time
}

type cachedToken struct {
	token string
	until time.Time
}

type hostBackoff struct {
	fails int
	until time.Time
}

// The breaker window starts at breakerBase and doubles per consecutive trip.
const (
	breakerBase = time.Minute
	breakerMax  = time.Hour
)

// New returns a Resolver for HTTPS registries.
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

// WithDockerHubAuth sets Docker Hub credentials, which raise the rate limit for
// docker.io images.
func (r *Resolver) WithDockerHubAuth(user, token string) *Resolver {
	r.dhUser, r.dhToken = user, token
	return r
}

// WithGitHubToken sets the token used against ghcr.io and lscr.io.
func (r *Resolver) WithGitHubToken(token string) *Resolver {
	r.ghToken = token
	return r
}

// hostGate keeps a minimum gap between requests to the same registry host.
type hostGate struct {
	mu   sync.Mutex
	last map[string]time.Time
	gap  time.Duration
}

func newHostGate(gap time.Duration) *hostGate {
	return &hostGate{last: map[string]time.Time{}, gap: gap}
}

// wait reserves the slot before sleeping, so concurrent callers for one host
// queue instead of firing together.
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

// noteHostOutcome trips or extends the breaker on a 429 that outlasted the
// retries in do and clears it on success.
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
		// Workers caught in the same burst count as one trip, or three of them
		// would push the window straight to 4m.
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

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// Resolve reports the newest semver tag (or the requested tag itself when that
// is not semver, so digest drift still shows), the digest behind the requested
// tag, the newest semver tag in the repo, and for a rolling tag that version's
// digest, which proves what a ":latest" container runs. curDigest is unused.
func (r *Resolver) Resolve(ctx context.Context, repo, tag, curDigest string) (newestTag, sameTagDigest, newestVerTag, newestVerDigest string, err error) {
	host, pathRepo := splitRepo(repo)
	base := r.baseURL(host)

	// The message carries no countdown, because the stored row may not be
	// refreshed until the next sweep.
	if r.breakerRemaining(host) > 0 {
		return "", "", "", "", fmt.Errorf("%s is rate limiting us; backing off before the next attempt", host)
	}

	tags, err := r.fetchTags(ctx, base, host, pathRepo)
	if err != nil {
		return "", "", "", "", err
	}

	newestVerTag = newestSemver(tags)
	newestTag = newestVerTag
	if !isSemver(tag) || newestTag == "" {
		newestTag = tag
	}

	sameTagDigest, err = r.fetchDigest(ctx, base, host, pathRepo, tag)
	if err != nil {
		return "", "", "", "", err
	}

	// A failure here only leaves the version unproven.
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
		return "registry-1.docker.io", "library/" + repo
	}
	host = repo[:first]
	pathRepo = repo[first+1:]
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}
	return host, pathRepo
}

// fetchTags follows the Link pagination of /v2/<repo>/tags/list, since a repo
// with hundreds of tags can keep its newest ones on a later page.
func (r *Resolver) fetchTags(ctx context.Context, base, host, pathRepo string) ([]string, error) {
	var all []string
	url := base + "/v2/" + pathRepo + "/tags/list?n=500"
	for page := 0; page < 100; page++ { // cap against a misbehaving registry
		resp, err := r.doAuthed(ctx, http.MethodGet, url, host, pathRepo)
		if err != nil {
			return nil, err
		}
		r.noteHostOutcome(host, resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return nil, ErrRepoNotFound
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, fmt.Errorf("tags/list: rate limited (429) for %s; set GITHUB_TOKEN for ghcr.io/lscr.io to raise the limit", pathRepo)
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

// nextLink returns the rel="next" target of a Link header, resolved against
// base because registries usually send a root-relative path.
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

// doAuthed sends a cached bearer token when it has one. On a 401 it drops that
// token, answers the Bearer challenge and retries once.
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

func (r *Resolver) cachedToken(key string) string {
	r.tokensMu.Lock()
	defer r.tokensMu.Unlock()
	if t, ok := r.tokens[key]; ok && r.now().Before(t.until) {
		return t.token
	}
	return ""
}

// storeToken keeps a token until a minute before it expires.
func (r *Resolver) storeToken(key, token string, ttl time.Duration) {
	// Registries often omit expires_in, but Docker Hub and ghcr issue 300s
	// tokens.
	if ttl < 300*time.Second {
		ttl = 300 * time.Second
	}
	ttl -= 60 * time.Second
	r.tokensMu.Lock()
	defer r.tokensMu.Unlock()
	r.tokens[key] = cachedToken{token: token, until: r.now().Add(ttl)}
}

func (r *Resolver) dropToken(key string) {
	r.tokensMu.Lock()
	defer r.tokensMu.Unlock()
	delete(r.tokens, key)
}

// do retries 429 and 503 with backoff that honours Retry-After, and returns
// the last response even when it is still one of them.
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
		wait := retryAfter(resp.Header.Get("Retry-After"), backoff(attempt))
		_ = resp.Body.Close()
		r.sleep(ctx, wait)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
}

func isTransient(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable
}

// backoff returns the jittered exponential delay for a zero-based attempt.
func backoff(attempt int) time.Duration {
	d := baseBackoff << attempt
	if d > maxBackoff {
		d = maxBackoff
	}
	return d + time.Duration(rand.Int63n(int64(baseBackoff)))
}

// retryAfter parses a Retry-After header in seconds or as an HTTP date, falling
// back to def.
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

func clampWait(d time.Duration) time.Duration {
	if d > maxRetryWait {
		return maxRetryWait
	}
	if d < 0 {
		return 0
	}
	return d
}

// fetchToken answers a Bearer challenge with a pull-scoped token request and
// returns the token and its lifetime (0 when none is advertised). Credentials go
// only to the host they belong to: Docker Hub's to registry-1.docker.io, the
// GitHub token to ghcr.io and lscr.io.
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
		if resp.StatusCode == http.StatusNotFound {
			return "", 0, ErrRepoNotFound
		}
		// Token throttling is the most common source of 429s, so it trips the
		// registry's breaker as well.
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

// parseChallenge reads the key="value" pairs of a challenge such as
// `Bearer realm="https://auth",service="reg",scope="repository:x:pull"`.
func parseChallenge(challenge string) map[string]string {
	out := map[string]string{}
	rest := strings.TrimSpace(challenge)
	if i := strings.IndexByte(rest, ' '); i >= 0 {
		rest = rest[i+1:]
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

// queryEscape percent-encodes a query value but keeps ':' and '/' readable in
// the scope.
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

func less(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func isSemver(t string) bool {
	_, ok := parseSemver(t)
	return ok
}

// parseSemver accepts two-part tags as well, since some projects publish only
// those. A non-numeric part such as "1.8.0-rc1" is rejected, so a prerelease
// never ranks as the newest tag.
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
