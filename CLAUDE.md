# ShipLog — Repo Guide

Read-only Docker update advisor: for each running container it fetches the changelog between the running image and the newest tag, classifies the risk (patch/minor/major), and serves it on a status page + JSON API. Pure-Go (cgo-free) engine on a distroless image; the headline UX (a changelog bubble in Unraid's native Docker tab) ships as a companion Unraid plugin. The engine never writes to the Docker socket — updates are handed to Unraid's own update path.

## Layout

- `cmd/shiplog/main.go` — entrypoint: prints the READY banner, loads config, wires the engine + HTTP server.
- `internal/engine/` — the poll loop (discover -> resolve -> changelog -> risk -> store) run every `POLL_INTERVAL`.
- `internal/dockercli/` — read-only Docker socket client (GET only). `internal/resolver/` — newest tag + digest from the registry (HEAD-only manifest checks).
- `internal/changelog/` — provider chain (OCI `source` label -> GitHub releases -> version-delta fallback). `internal/sources/` — curated / user source overrides.
- `internal/risk/` — pure version-delta -> risk classifier. `internal/model/` — shared types.
- `internal/autoupdate/` — scheduled auto-update policy/schedule/executor (Unraid-plugin surface; gated by SemVer level). `internal/updater/` — one-click bulk update trigger.
- `internal/store/` — SQLite (`modernc.org/sqlite`, cgo-free). `internal/config/` — env config. `internal/api/`, `internal/templates/` — HTTP API + status page. `internal/notify/` (Matrix), `internal/summarize/` (Ollama), `internal/bannerlog/` (READY banner).
- `plugin/` — the Unraid plugin: `src/shiplog/...` tree (PHP/JS/`.page`/26 `lang/*.json`), `pkg_build.sh` (.txz builder), `shiplog.plg` (install manifest + `<CHANGES>`).
- `.github/workflows/` — `lint.yml` (test), `build.yml` (image), `release.yml` (plugin .txz).

## Build / test / lint (run before every push)

```sh
go build ./...            # compile
go vet ./...
gofmt -l .                # must print nothing (CI + the pre-push hook fail on output)
go test -race ./...       # CI runs the race detector; no external deps needed
golangci-lint run --timeout 5m   # CI gate (.golangci.yml: errcheck/govet/ineffassign/staticcheck/unused/misspell + gofmt)
hadolint Dockerfile
```

`just check` runs the whole chain in one go (see `justfile`). The global pre-push hook already runs gofmt + hadolint + gitleaks.

Docker image: `docker build -t shiplog:dev .` (multi-stage: `golang:1.25-bookworm` build -> `gcr.io/distroless/static-debian12`, `CGO_ENABLED=0`). Plugin package: `bash plugin/pkg_build.sh <version>` -> `plugin/out/shiplog-<version>-x86_64-1.txz`.

## Release (NEVER tag without explicit approval)

Two independent release surfaces:

- **Docker engine image** — auto-built and pushed on every push to `main` (no tag needed): boot-smoke-gates BOTH arches, then pushes `:latest` + `:sha-<short>` to GHCR (+ Docker Hub when the mirror secrets are set). Nothing to do manually.
- **Unraid plugin** — cut from a `vX.Y.Z` tag by `release.yml`:
  1. Add a `### X.Y.Z` block to `plugin/shiplog.plg` `<CHANGES>` (this is the canonical release-notes source — CI extracts it into the GitHub release body).
  2. `git tag vX.Y.Z && git push origin vX.Y.Z` (SemVer 3-digit; release title = version only, no repo-name heading).
  3. CI builds the `.txz`, publishes the GitHub release (asset live first), confirms it's reachable, then flips `<!ENTITY version>` + `<SHA256>` in `plugin/shiplog.plg` on `main` **last** — Unraid reads that `.plg` from raw `main` and verifies the `.txz` against `<SHA256>`, so `main` must flip last. Do NOT bump the entity by hand.
  4. `workflow_dispatch` on `release.yml` is a build-only dry run (uploads the `.txz` artifact, no release, no `main` change).

The `.plg` `<SHA256>` must always match the published `.txz`; a stale hash aborts the install.

## CI gates

- **Lint** (`lint.yml`) — `gofmt -l`, `go vet`, `go test -race ./...`, and `golangci-lint run`.
- **Build & Push Docker Image** (`build.yml`) — build amd64 + arm64 images locally -> boot-smoke BOTH arches (must print `SHIPLOG IS READY` and serve `/`) -> non-blocking Trivy CVE scan of `shiplog:smoke-amd64` (SARIF -> Security tab) -> multi-arch build+push with SBOM + provenance attestations -> Docker Hub README/description mirror.
- **Release** (`release.yml`) — tag-only; builds + publishes the plugin `.txz`.

## Conventions / gotchas

- **Read-only by construction** — the engine only ever issues GET calls over the Docker socket (mounted `:ro`). Update actions are triggered on Unraid's own update tooling; the engine never pulls/recreates/stops. Keep it that way.
- **Registry-friendly** — manifest checks are HEAD (don't count against Docker Hub's pull limit); bearer tokens cached, one lookup per distinct image per sweep, host-wide backoff after a 429. Don't turn these into GETs.
- **Distroless-as-root is intentional** — it must READ the root-owned socket and write SQLite under `/config`; security comes from the `:ro` socket mount, not from a non-root uid.
- **Plugin files must be LF** — a CRLF `.page` breaks Unraid's PageBuilder and a trailing CR breaks shell shebangs. `.gitattributes` forces `eol=lf`; `pkg_build.sh` re-normalises as belt-and-suspenders. Strip CR (`sed -i 's/\r$//'`) on any new file authored on Windows.
- **Async cleanup can flake on Linux CI** — wait for the goroutine. A cancelled exec returns `*ExitError`; remap via `ctx.Err()`.
- **restic-style hash mismatch on stable input = bad RAM**, not a code bug.
- **i18n** — new UI strings go into all 26 `plugin/.../lang/*.json` locales in the same change.
- No real user data / IPs in the repo. This repo is and has always been **public**.
