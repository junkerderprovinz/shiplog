# ShipLog P0 — Engine Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A working, read-only ShipLog **engine** container that lists every container on the host, resolves which have updates, computes a deterministic risk verdict, fetches a changelog for OCI-labeled/GitHub-sourced images, stores results in SQLite, and serves a JSON API + minimal status page — shipped as a tiny multi-arch image gated by a boot smoke test.

**Architecture:** Single static Go binary. A scheduler polls the read-only Docker socket; per container an in-process pipeline runs `collector → resolver → risk → changelog → store`. A `net/http` server exposes `/api/*` and an embedded status page. No cgo (pure-Go SQLite) → `FROM scratch`-class image.

**Tech Stack:** Go 1.23, stdlib `net/http` (Docker socket via custom unix dialer; registries/GitHub via HTTPS), `modernc.org/sqlite` (pure Go), `go:embed` for the status page, golangci-lint, multi-arch Docker + GHCR/Docker Hub.

**Out of scope (later plans):** P1 = LSIO + curated changelog providers, Ollama summaries, Matrix notify, full 26-locale i18n catalogue. P2 = the Unraid `.plg` plugin (bubble injection + PHP proxy). P0 ships English-only status page with i18n-ready string keys.

---

## File Structure

```
shiplog/
  go.mod  go.sum
  cmd/shiplog/main.go              # wire config → banner → store → engine → http; READY banner
  internal/
    config/config.go               # env → Config struct
    bannerlog/banner.go            # ASCII init banner + "<APP> IS READY" banner
    model/model.go                 # shared types (Task 1) — every other unit imports this
    dockercli/dockercli.go         # minimal Docker Engine API client over the unix socket (ro)
    dockercli/dockercli_test.go
    resolver/resolver.go           # OCI registry v2: newest tag + digest for an image ref
    resolver/resolver_test.go
    risk/risk.go                   # pure: version delta → Kind + RiskLevel + reason
    risk/risk_test.go
    changelog/changelog.go         # Provider interface + Chain
    changelog/github.go            # OCI-source label → GitHub releases between tags
    changelog/fallback.go          # semver-delta + compare link (always succeeds)
    changelog/changelog_test.go
    store/store.go                 # SQLite schema + upsert/list/history
    store/store_test.go
    engine/engine.go               # poll loop: collector→resolver→risk→changelog→store
    engine/engine_test.go
    api/api.go                     # /api/containers, /api/container/{id}, /api/refresh
    api/api_test.go
    web/web.go  web/status.html    # embedded status page (go:embed)
  Dockerfile
  .dockerignore
  .github/workflows/lint.yml       # golangci-lint + go test (race)
  .github/workflows/build.yml      # multi-arch build + both-arch boot smoke gate
  .golangci.yml
  README.md
  LICENSE                          # MIT
```

Module path: `github.com/junkerderprovinz/shiplog`. Docker Engine API version pinned to `v1.43` (broadly compatible). Registry calls use the standard Docker Registry HTTP API v2 token flow.

---

### Task 1: Module scaffold + shared model

**Files:**
- Create: `go.mod`, `internal/model/model.go`, `internal/model/model_test.go`

- [ ] **Step 1: Init the module**

Run: `cd d:\nextcloud\it\github\shiplog && go mod init github.com/junkerderprovinz/shiplog && go mod edit -go=1.23`
Expected: `go.mod` created.

- [ ] **Step 2: Write the failing test for the shared enums/helpers**

`internal/model/model_test.go`:
```go
package model

import "testing"

func TestRiskLevelOrder(t *testing.T) {
	if !RiskHigh.MoreSevere(RiskMedium) || !RiskMedium.MoreSevere(RiskLow) || RiskLow.MoreSevere(RiskHigh) {
		t.Fatal("severity ordering wrong")
	}
	if RiskUnknown.MoreSevere(RiskLow) {
		t.Fatal("unknown must not outrank a real level")
	}
}

func TestUpdateStatusHasUpdate(t *testing.T) {
	s := UpdateStatus{Kind: KindNone}
	if s.HasUpdate() {
		t.Fatal("KindNone must not be an update")
	}
	s.Kind = KindMinor
	if !s.HasUpdate() {
		t.Fatal("KindMinor must be an update")
	}
}
```

- [ ] **Step 3: Run it, expect FAIL** — `go test ./internal/model/` → fails (undefined symbols).

- [ ] **Step 4: Implement the model**

`internal/model/model.go`:
```go
// Package model holds the types shared across every ShipLog engine unit.
package model

import "time"

// Container is a single container as discovered on the host.
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`  // image ref as referenced, e.g. "ghcr.io/x/y:1.2.3" or "redis"
	Repo   string `json:"repo"`   // normalized, e.g. "ghcr.io/x/y" or "docker.io/library/redis"
	Tag    string `json:"tag"`    // "1.2.3" / "latest"
	Digest string `json:"digest"` // running image digest, "sha256:..."
	Source string `json:"source"` // org.opencontainers.image.source label, may be ""
	State  string `json:"state"`  // "running" / "exited" / ...
}

type RiskLevel string

const (
	RiskNone    RiskLevel = "none"
	RiskLow     RiskLevel = "low"
	RiskMedium  RiskLevel = "medium"
	RiskHigh    RiskLevel = "high"
	RiskUnknown RiskLevel = "unknown"
)

var riskRank = map[RiskLevel]int{RiskNone: 0, RiskUnknown: 1, RiskLow: 2, RiskMedium: 3, RiskHigh: 4}

// MoreSevere reports whether r outranks o (unknown never outranks a real level).
func (r RiskLevel) MoreSevere(o RiskLevel) bool { return riskRank[r] > riskRank[o] }

// Kind classifies the version delta.
type Kind string

const (
	KindNone    Kind = "none"
	KindDigest  Kind = "digest" // same tag, new digest (e.g. :latest moved)
	KindPatch   Kind = "patch"
	KindMinor   Kind = "minor"
	KindMajor   Kind = "major"
	KindUnknown Kind = "unknown" // non-semver tag, can't classify
)

// UpdateStatus is the per-container result the engine stores and serves.
type UpdateStatus struct {
	Container    Container  `json:"container"`
	NewestTag    string     `json:"newest_tag"`
	NewestDigest string     `json:"newest_digest"`
	Kind         Kind       `json:"kind"`
	Risk         RiskLevel  `json:"risk"`
	RiskReason   string     `json:"risk_reason"`
	Changelog    *Changelog `json:"changelog,omitempty"`
	CheckedAt    time.Time  `json:"checked_at"`
	Error        string     `json:"error,omitempty"` // per-container failure, never fatal
}

func (s UpdateStatus) HasUpdate() bool { return s.Kind != KindNone && s.Kind != "" }

// Changelog is the resolved "what changed" payload.
type Changelog struct {
	FromTag      string         `json:"from_tag"`
	ToTag        string         `json:"to_tag"`
	SkippedCount int            `json:"skipped_count"`
	Entries      []ReleaseEntry `json:"entries"` // newest first
	Raw          string         `json:"raw"`
	Summary      *AISummary     `json:"summary,omitempty"` // P1 (Ollama)
	Source       string         `json:"source"`            // human label, e.g. "GitHub releases via OCI label"
	URL          string         `json:"url"`               // releases/compare link
	Provider     string         `json:"provider"`          // "github" / "fallback" / ...
}

type ReleaseEntry struct {
	Tag         string    `json:"tag"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// AISummary is populated only when Ollama is configured (P1).
type AISummary struct {
	Bullets  []string `json:"bullets"`
	Breaking []string `json:"breaking"`
	Risk     string   `json:"risk"`
	Model    string   `json:"model"`
}
```

- [ ] **Step 5: Run it, expect PASS** — `go test ./internal/model/` → PASS.

- [ ] **Step 6: Commit**
```
git add go.mod internal/model
git commit -m "feat(model): shared engine types (Container, UpdateStatus, Risk, Changelog)"
```

---

### Task 2: Risk engine (pure, golden-tested) — PARALLEL-SAFE (only depends on Task 1)

**Files:** Create `internal/risk/risk.go`, `internal/risk/risk_test.go`

- [ ] **Step 1: Write the golden table test**

`internal/risk/risk_test.go`:
```go
package risk

import (
	"testing"
	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		cur, newest      string
		curDig, newDig   string
		wantKind         model.Kind
		wantRisk         model.RiskLevel
	}{
		{"1.2.3", "1.2.3", "sha256:a", "sha256:a", model.KindNone, model.RiskNone},
		{"latest", "latest", "sha256:a", "sha256:b", model.KindDigest, model.RiskLow},
		{"1.2.3", "1.2.4", "", "", model.KindPatch, model.RiskLow},
		{"1.2.3", "1.3.0", "", "", model.KindMinor, model.RiskMedium},
		{"1.2.3", "2.0.0", "", "", model.KindMajor, model.RiskHigh},
		{"v1.2.3", "v1.4.0", "", "", model.KindMinor, model.RiskMedium}, // v-prefix tolerated
		{"stable", "stable", "sha256:a", "sha256:a", model.KindNone, model.RiskNone},
		{"weird-tag", "other-tag", "", "", model.KindUnknown, model.RiskUnknown},
	}
	for _, c := range cases {
		k, r, reason := Classify(c.cur, c.newest, c.curDig, c.newDig)
		if k != c.wantKind || r != c.wantRisk {
			t.Errorf("Classify(%q,%q)=%s/%s want %s/%s", c.cur, c.newest, k, r, c.wantKind, c.wantRisk)
		}
		if reason == "" {
			t.Errorf("Classify(%q,%q) empty reason", c.cur, c.newest)
		}
	}
}
```

- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/risk/`.

- [ ] **Step 3: Implement**

`internal/risk/risk.go` — parse semver (strip leading `v`, split major.minor.patch on `.`, ignore pre-release/build for classification). Logic:
- both tags equal: if digests known and differ → `KindDigest/RiskLow` ("same tag, new image digest"); else `KindNone/RiskNone`.
- both parse as semver: compare major→minor→patch; first differing level decides Kind (major/minor/patch) and Risk (high/medium/low) with a reason string like `"major version bump 1.x → 2.x — review breaking changes"`.
- otherwise: `KindUnknown/RiskUnknown` ("non-semver tags, cannot compare automatically").
Signature: `func Classify(cur, newest, curDigest, newDigest string) (model.Kind, model.RiskLevel, string)`. No external deps; hand-roll the parse (do not pull a semver lib for P0).

- [ ] **Step 4: Run, expect PASS.**

- [ ] **Step 5: Commit** — `git commit -m "feat(risk): deterministic version-delta risk classifier"`.

---

### Task 3: SQLite store — PARALLEL-SAFE (depends on Task 1)

**Files:** Create `internal/store/store.go`, `internal/store/store_test.go`

- [ ] **Step 1: Failing test** (uses a temp-file DB; pure-Go driver, no cgo):
```go
package store

import (
	"testing"
	"time"
	"github.com/junkerderprovinz/shiplog/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/shiplog.db")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndList(t *testing.T) {
	s := newTestStore(t)
	st := model.UpdateStatus{
		Container: model.Container{ID: "abc", Name: "immich", Repo: "ghcr.io/x/immich", Tag: "1.122.0"},
		NewestTag: "1.124.2", Kind: model.KindMinor, Risk: model.RiskMedium, CheckedAt: time.Now(),
	}
	if err := s.Upsert(st); err != nil { t.Fatal(err) }
	st.Risk = model.RiskHigh
	if err := s.Upsert(st); err != nil { t.Fatal(err) } // same ID → update, not duplicate
	all, err := s.List()
	if err != nil { t.Fatal(err) }
	if len(all) != 1 { t.Fatalf("want 1 row, got %d", len(all)) }
	if all[0].Risk != model.RiskHigh { t.Fatal("upsert did not update") }
}

func TestHistoryAppendOnVersionChange(t *testing.T) {
	s := newTestStore(t)
	base := model.UpdateStatus{Container: model.Container{ID: "abc", Name: "immich", Tag: "1.122.0"}, CheckedAt: time.Now()}
	_ = s.Upsert(base)
	base.Container.Tag = "1.124.2" // running version changed → history row
	_ = s.Upsert(base)
	h, err := s.History("abc")
	if err != nil { t.Fatal(err) }
	if len(h) < 1 { t.Fatal("expected a history entry on running-version change") }
}
```

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement** `internal/store/store.go`:
  - `import _ "modernc.org/sqlite"`; `sql.Open("sqlite", path)`; set `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`.
  - Idempotent schema in `Open` (`CREATE TABLE IF NOT EXISTS`): `status(container_id TEXT PRIMARY KEY, name, repo, image, tag, digest, newest_tag, newest_digest, kind, risk, risk_reason, changelog_json, checked_at, error)` and `history(container_id, name, from_tag, to_tag, seen_at)`.
  - `Upsert`: `INSERT ... ON CONFLICT(container_id) DO UPDATE SET ...`. Before writing, read the prior `tag`; if it differs from the new running tag (and prior non-empty), insert a `history` row. Marshal `Changelog` to JSON for `changelog_json`.
  - `List() ([]model.UpdateStatus, error)` ordered by risk rank desc then name; `Get(id)`, `History(id)`, `Close()`.
  - `go get modernc.org/sqlite` first.

- [ ] **Step 4: Run, expect PASS.**
- [ ] **Step 5: Commit** — `git commit -m "feat(store): SQLite status + history with WAL, pure-Go driver"`.

---

### Task 4: Docker socket client — PARALLEL-SAFE (depends on Task 1)

**Files:** Create `internal/dockercli/dockercli.go`, `_test.go`

- [ ] **Step 1: Failing test** using an `httptest` server over a real unix socket fixture. Spin a `net.Listen("unix", tmpSock)` serving canned `/v1.43/containers/json` + `/v1.43/containers/{id}/json` responses, point the client at it, assert it returns `[]model.Container` with Name (leading `/` stripped), Repo/Tag split, Digest from `Image`/`ImageID`, and the `org.opencontainers.image.source` label surfaced. (Full fixture JSON in the test.)

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement**:
  - `http.Client` with `Transport{DialContext: unix-dial to socketPath}` and base URL `http://docker/v1.43`.
  - `List(ctx) ([]model.Container, error)`: GET `/containers/json?all=1`; for each, GET `/containers/{id}/json` (for `Config.Labels` + `Image` digest) OR `/images/{img}/json`. Parse `Names[0]` (strip `/`), split `Image` into Repo/Tag (normalize bare names to `docker.io/library/...`), read `org.opencontainers.image.source`.
  - Socket path from config (default `/var/run/docker.sock`). The client never issues writes — list/inspect only.

- [ ] **Step 4: Run, expect PASS.**
- [ ] **Step 5: Commit** — `git commit -m "feat(dockercli): read-only Docker Engine API client over the unix socket"`.

---

### Task 5: Registry resolver — PARALLEL-SAFE (depends on Task 1)

**Files:** Create `internal/resolver/resolver.go`, `_test.go`

- [ ] **Step 1: Failing test** with `httptest.NewServer` faking the registry v2 endpoints (token at `/token`, `/v2/<repo>/tags/list`, `HEAD /v2/<repo>/manifests/<ref>` returning `Docker-Content-Digest`). Assert `Resolve(ctx, repo, tag, digest)` returns the newest semver tag and the same-tag digest, and flags `UpdateAvailable` on digest drift.

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement** `func (r *Resolver) Resolve(ctx, repo, tag, curDigest string) (newestTag, sameTagDigest string, err error)`:
  - Registry host from repo (`docker.io`→`registry-1.docker.io`, else the host prefix). Anonymous bearer token: on 401, parse `WWW-Authenticate`, GET the `realm?service=&scope=repository:<repo>:pull`, use the token.
  - `tags/list` → pick newest by semver (reuse the parse approach from risk; ignore non-semver tags for "newest"); if current tag is non-semver (e.g. `latest`), newestTag = current tag.
  - `HEAD manifests/<tag>` with `Accept` for v2+OCI manifest/index → `Docker-Content-Digest`.
  - Make the HTTP base injectable for tests (a `baseURLFor func(host) string` field defaulting to `https://`).

- [ ] **Step 4: Run, expect PASS.**
- [ ] **Step 5: Commit** — `git commit -m "feat(resolver): OCI registry v2 newest-tag + digest resolution (Docker Hub/GHCR)"`.

---

### Task 6: Changelog provider chain (GitHub + fallback) — depends on Task 1

**Files:** Create `internal/changelog/changelog.go`, `github.go`, `fallback.go`, `changelog_test.go`

- [ ] **Step 1: Failing test**: a `Chain` of `[github(fakeAPI), fallback]`; for a container whose `Source` is `https://github.com/o/r`, GitHub provider (httptest-faked `/repos/o/r/releases`) returns entries for tags between `from` and `to`, newest first, with `SkippedCount`; when Source is empty, the chain falls through to `fallback` which always returns a `Changelog` with a compare URL and `Provider=="fallback"`.

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement**:
  - `type Provider interface { Get(ctx, model.Container, fromTag, toTag string) (*model.Changelog, bool) }` (bool = handled).
  - `Chain []Provider` → `Get` returns the first handled result.
  - `github.go`: parse owner/repo from the Source URL; GET `/repos/{o}/{r}/releases?per_page=100` (optional `GITHUB_TOKEN` + `If-None-Match` ETag cache field); keep releases whose tag is `> fromTag && <= toTag` (semver compare, v-prefix tolerant); newest first; `Raw` = concatenated bodies; `URL` = `compare/{from}...{to}`; `Source="GitHub releases (OCI label)"`.
  - `fallback.go`: always handled; `Raw` empty, `URL` = best-effort releases page if Source is a known forge else ""; `Source="version delta only"`.

- [ ] **Step 4: Run, expect PASS.**
- [ ] **Step 5: Commit** — `git commit -m "feat(changelog): provider chain — GitHub releases + always-on fallback"`.

---

### Task 7: Engine orchestrator (poll loop) — depends on Tasks 1–6

**Files:** Create `internal/engine/engine.go`, `_test.go`; `internal/config/config.go`

- [ ] **Step 1: config** — `Config{ DockerSocket, Port, PollInterval, GithubToken, ... }` from env with the spec defaults (PORT 8484, POLL_INTERVAL 6h). Small unit test for parsing `POLL_INTERVAL` durations + defaults.

- [ ] **Step 2: Failing engine test** — inject fakes for the four collaborators (interfaces: `Collector`, `Resolver`, `Changelogger`, `Store`); run one `Sweep(ctx)`; assert that for a container with a newer tag the stored `UpdateStatus` has the right Kind/Risk and a Changelog, and that a collaborator error for one container is captured in that row's `Error` without aborting the sweep (the other container still succeeds).

- [ ] **Step 3: Implement** `Engine` with those interfaces (so units stay mockable). `Sweep`: collect → bounded worker pool (e.g. 6) → per container `resolver.Resolve` → `risk.Classify` → if update, `changelog.Get` → `store.Upsert`; recover/record per-container errors. `Run(ctx)`: sweep once immediately, then on a `time.Ticker(PollInterval)`; `Refresh()` triggers an out-of-band sweep (for the API).

- [ ] **Step 4: Run, expect PASS** (`go test -race ./internal/engine/`).
- [ ] **Step 5: Commit** — `git commit -m "feat(engine): concurrent poll loop wiring collector→resolver→risk→changelog→store"`.

---

### Task 8: HTTP API + embedded status page — depends on Tasks 1,3,7

**Files:** Create `internal/api/api.go`, `_test.go`, `internal/web/web.go`, `internal/web/status.html`

- [ ] **Step 1: Failing test** — `httptest` against the handler: `GET /api/containers` returns the store's list as JSON; `GET /api/container/{id}` returns one or 404; `POST /api/refresh` calls the engine's `Refresh` (fake) and returns 202; `GET /` returns 200 `text/html` containing a known container name.

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement** — `net/http.ServeMux`; JSON handlers read from `store`; `/` renders `status.html` (go:embed `//go:embed status.html`) with the list (table: container, current→newest, risk badge span with a CSS class per level, changelog text, last/next poll). Risk-badge colour vs monochrome via a `?mono=1` query / cookie toggle. Keep strings as keys in a small `map[string]string` (English) so P1 can swap in the 26-locale catalogue.

- [ ] **Step 4: Run, expect PASS.**
- [ ] **Step 5: Commit** — `git commit -m "feat(api): REST endpoints + embedded status page with toggleable risk badges"`.

---

### Task 9: main, banners, wiring

**Files:** Create `cmd/shiplog/main.go`, `internal/bannerlog/banner.go`

- [ ] **Step 1:** `bannerlog.Init()` prints the ASCII init banner at startup; `bannerlog.Ready(port)` prints a loud `███ SHIPLOG IS READY — http://0.0.0.0:8484 ███`-style banner once the HTTP listener is up (house [[ready-log-banner-standard]]). Tiny test asserting `Ready` writes a line containing `SHIPLOG IS READY`.
- [ ] **Step 2:** `main.go`: load config → `bannerlog.Init` → open store → build collaborators (dockercli, resolver, changelog chain) → engine → start engine goroutine → start HTTP, and call `bannerlog.Ready` from the server's `BaseContext`/after `Listen`. Graceful shutdown on SIGTERM (PID 1 in the container).
- [ ] **Step 3:** `go build ./...` succeeds; `go vet ./...` clean.
- [ ] **Step 4: Commit** — `git commit -m "feat(cmd): wire engine + http server, ASCII init + READY banners"`.

---

### Task 10: Dockerfile + .dockerignore (tiny static image)

**Files:** Create `Dockerfile`, `.dockerignore`

- [ ] **Step 1:** multi-stage:
```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/shiplog ./cmd/shiplog

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shiplog /usr/local/bin/shiplog
ENV PORT=8484 DOCKER_SOCKET=/var/run/docker.sock DATA_DIR=/config
EXPOSE 8484
VOLUME /config
ENTRYPOINT ["/usr/local/bin/shiplog"]
```
(`distroless/static` ships CA certs for the HTTPS registry/GitHub calls; `nonroot` user — the read-only socket mount must be group-readable, document that.) `.dockerignore`: `.git`, `docs`, `*.md`, test artifacts.

- [ ] **Step 2:** local build (amd64) `docker build -t shiplog:dev .` if Docker is available; otherwise CI proves it. Commit — `git commit -m "build: distroless static multi-arch Dockerfile"`.

---

### Task 11: CI — lint + test, and the boot smoke gate

**Files:** Create `.github/workflows/lint.yml`, `.github/workflows/build.yml`, `.golangci.yml`

- [ ] **Step 1: lint.yml** — on push/PR: `golangci-lint run` + `go test -race ./...`.

- [ ] **Step 2: build.yml** — on push to main + dispatch: QEMU + Buildx, login GHCR (+ Docker Hub when the var/secret pair is set), metadata (`:latest` + `:sha-`). **Smoke gate before push** (both arches; SIGPIPE-safe — capture logs to a var, bash substring match, never `docker logs | grep -q` under pipefail, per [[ci-boot-smoke-gate]]):
```bash
smoke(){ tag=$1; plat=$2; budget=$3; name=sl-${plat##*/}
  docker run -d --name "$name" --platform "$plat" -p 8484:8484 \
    -v /var/run/docker.sock:/var/run/docker.sock:ro "$tag"
  for i in $(seq 1 "$budget"); do
    logs=$(docker logs "$name" 2>&1 || true)
    if [[ "$logs" == *"SHIPLOG IS READY"* ]] && curl -fsS -o /dev/null http://localhost:8484/; then
      echo "✅ $plat ready+serving (${i}s)"; docker rm -f "$name" >/dev/null; return 0; fi
    sleep 1
  done
  echo "❌ $plat not ready in ${budget}s"; docker logs "$name" || true; docker rm -f "$name" >/dev/null; return 1; }
smoke shiplog:smoke-amd64 linux/amd64 30
smoke shiplog:smoke-arm64 linux/arm64 90
```
Build each arch with `load:true` + `cache-to: gha,mode=max`, smoke both, then the multi-arch `Build & push`.

- [ ] **Step 3: Commit** — `git commit -m "ci: golangci-lint + race tests; multi-arch build with both-arch boot smoke gate"`.

---

### Task 12: README + LICENSE (canonical layout)

- [ ] Per [[canonical-readme-order]]: heading → (banner placeholder) → CI badges first → short desc → BMAC button → ToC → numbered sections (What/Features/Install on Unraid/Configuration table with the env vars/How it works/Roadmap/License). State read-only + the socket-ro security note prominently. MIT LICENSE.
- [ ] Commit — `git commit -m "docs: README (canonical layout) + MIT license"`.

---

## Self-Review

**Spec coverage:** core loop (Tasks 4,5,6,7) ✓ · risk badge (2) ✓ · SQLite + history (3) ✓ · API + status page + badge toggle (8) ✓ · honest changelog degradation = fallback provider (6) ✓ · banners (9) ✓ · tiny image (10) ✓ · both-arch SIGPIPE-safe smoke gate (11) ✓ · read-only socket (4,10) ✓. **Deferred by design (P1/P2, noted in header):** LSIO+curated providers, Ollama summarize, Matrix notify, full 26-locale catalogue, the `.plg` plugin, the CA template — each gets its own plan; P0 keeps stable interfaces (`Provider`, `summarize`/`notify` are additive) so they slot in without rework.

**Placeholder scan:** no "TBD/handle errors appropriately"; each task names exact files, test code, and commit. Implementation prose for Tasks 3–6 specifies signatures + behaviour concretely (kept tight because a Go subagent fills idiomatic bodies against the provided tests — the tests are the contract).

**Type consistency:** all units import `internal/model`; `Classify` signature, `Provider.Get`, `Store.Upsert/List/History`, `Engine.Sweep/Run/Refresh` are referenced identically across Tasks 2–9.

## Execution

Suited to **subagent-driven-development**: Tasks 2,3,4,5 are independent (only depend on Task 1's model) → one parallel wave after Task 1; then 6, then 7 (needs 4–6), then 8 (needs 3,7), then 9–12 sequentially. Each task is TDD with its own commit; review between tasks.
