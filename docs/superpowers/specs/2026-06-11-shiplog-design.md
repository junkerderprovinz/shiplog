# ShipLog — design spec (2026-06-11)

⚓ **ShipLog** (repo slug `shiplog`, like BombVault/`bombvault`) is a
self-hosted, read-only **update advisor** for Docker hosts
(Unraid first): it tells you **what actually changes** between the image a
container is running and the newest available one — changelog, version jump,
and a risk verdict — before you press "update".

It deliberately does **not** update anything. Unraid (and the CA auto-update
plugin) already handle applying updates and basic "update available"
notifications; the missing piece is the *intelligence* — "what's in it, and
how risky is it?". shiplog fills exactly that gap.

## Decisions (brainstorm 2026-06-11)

| Topic | Decision |
|---|---|
| Name | **ShipLog** — CamelCase like BombVault (working title was "Kassandra"; collision check: only small namesakes in unrelated niches, `junkerderprovinz/shiplog` free on GitHub + Docker Hub, no CA entry) |
| Form | **Docker container**, own image (no Unraid plugin — plugins have no clean daemon model, run as root in the host OS, and are Unraid-only). A thin companion plugin that injects badges into the native Docker tab can come later (v2+); the engine stays in the container. |
| Role | **Advisor only, read-only.** Docker socket mounted read-only; shiplog never pulls, recreates, or stops anything. |
| Stack | **Go backend + embedded React SPA** in a single binary (BombVault pattern). Polling, registry calls, scheduling are Go's home turf; image stays tiny. |
| Intelligence | **Deterministic heuristic always** (version-jump → risk badge), **optional Ollama** summary on top. AI off by default; the tool is fully useful without it. |
| Notifications | **Optional Matrix push**, off by default — enriched (changelog summary + risk), sent once when an update is first sighted. No duplication of Unraid's bare notifications. |
| Persistence | SQLite (single file, no DB server) — house consistency. |
| Auth | None in v1 (read-only LAN tool; document trusted-LAN assumption). Revisit if a reverse-proxy-exposed use case emerges. |
| Distribution | GHCR + Docker Hub dual push, CA template in the `unraid-docker-templates` feed, boot smoke gate (both arches — fast Go web server), ASCII init banner + READY banner, 26-language i18n, dark/light UI (house standards). |

## Core loop (background, in-process scheduler)

Every `POLL_INTERVAL` (default 6h, configurable):

1. **Discover** running + stopped containers via the read-only Docker socket
   (`/var/run/docker.sock:ro`): name, image ref (repo:tag), image ID, current
   digest, OCI labels.
2. **Resolve upstream** per image: query the registry (Docker Hub, GHCR, lscr.io,
   quay.io — generic OCI registry v2 API, anonymous) for
   (a) the digest of the *same tag* (digest drift = update available even on
   `:latest`) and (b) the newest *semver tag* when the current tag is semver.
3. **Fetch the changelog** for the version span via a layered provider chain
   (first hit wins):
   1. OCI label `org.opencontainers.image.source` → GitHub/GitLab **releases
      between current and newest tag** (the gold source; most modern images set it).
   2. linuxserver.io convention (their changelog endpoint / release notes) —
      the fleet runs many LSIO images.
   3. Curated mapping file for popular images without labels (shipped in the
      image, extendable via a user-mounted JSON).
   4. Fallback: semver delta only + a link to the project's releases/compare page.
   **Honest degradation is a feature:** when no changelog is machine-findable,
   shiplog says so and shows what it *does* know, never pretends.
4. **Risk badge** (deterministic): digest-only = low · patch = low · minor =
   medium · major = high · non-semver/unknown = grey (with reason).
5. **Optional Ollama summary**: if `OLLAMA_URL` + `OLLAMA_MODEL` are set, the
   fetched changelog is summarized into "What changes / Breaking / Migration
   notes" with a risk rationale. Marked as AI-generated in the UI. Failures
   fall back to the raw changelog silently (logged).
6. **Store** results in SQLite; **notify** via Matrix (if enabled) on first
   sighting of a new update for a container.

## UI (embedded React SPA)

- **Container list** (the heart): one row per container — name, icon, current →
  newest, risk badge, "what changed" popover/panel (AI summary if available,
  raw changelog excerpt, source link, provider that found it).
- Filters: updates only · by risk · by registry. Sort by risk/name/age.
- Detail view per container: full changelog span (multiple releases listed),
  resolution trace ("found via OCI label → GitHub releases"), digest info.
- House standards: dark/light theme (cookie, no-flash), i18n (26 locales,
  count-neutral phrasing), flag LanguageSwitcher + sun/moon ThemeToggle,
  READY banner in the log when the server is up.

## Configuration (env, all surfaced in the Unraid template; fixed-option fields as dropdowns)

| Var | Default | Notes |
|---|---|---|
| `POLL_INTERVAL` | `6h` | dropdown: 1h/3h/6h/12h/24h |
| `GITHUB_TOKEN` | — | optional; raises the anonymous 60 req/h API limit for changelog fetching |
| `OLLAMA_URL` / `OLLAMA_MODEL` | — | optional AI summaries |
| `MATRIX_HOMESERVER` / `MATRIX_TOKEN` / `MATRIX_ROOM` | — | optional push, off when unset |
| `TZ` | Europe/Vienna | dropdown, full IANA list |
| `PORT` | 8484 | WebUI |
| Mounts | `/var/run/docker.sock:ro`, `/config` (SQLite + curated-mapping override) | socket read-only is part of the security story |

## Architecture (units, single binary)

- `collector` — Docker socket client (list containers/images, labels). Interface-mocked in tests.
- `resolver` — OCI registry client (tag list, manifest digests; per-registry auth-free token dance for Docker Hub/GHCR).
- `changelog` — provider chain (oci-source → lsio → curated → fallback), each provider a small interface implementation.
- `risk` — pure function (current, newest, digests) → badge + reason. Golden-table tested.
- `summarize` — Ollama client (optional).
- `notify` — Matrix client (optional).
- `store` — SQLite (modernc.org/sqlite or mattn; pick pure-Go for painless cross-compile).
- `api` — REST for the SPA (`/api/containers`, `/api/container/{id}`, `/api/refresh`).
- `web` — embedded SPA (go:embed).

## Error handling

- Per-container failures never kill the sweep: each row carries its own
  status/reason (e.g. "registry rate-limited", "no changelog found").
- GitHub rate limits: ETag/If-None-Match caching, exponential backoff, optional token.
- Registry quirks (Docker Hub anonymous token, GHCR) isolated in `resolver`.
- Ollama/Matrix failures: log + degrade, never block the core loop.

## Testing

- Go: table-driven unit tests per unit; `httptest` fakes for registries/GitHub/LSIO;
  golden tests for the risk engine and provider chain ordering.
- SPA: key-parity test across all 26 locales (house pattern), component smoke tests.
- CI: lint (golangci-lint) + tests + multi-arch build + **boot smoke gate on both
  arches** before any push (house rule; SIGPIPE-safe log assertions).

## Non-goals (v1 — YAGNI)

- No updating/recreating/stopping containers (read-only advisor).
- No per-container schedules, no private-registry auth, no queue of hosts
  (one host = one shiplog).
- No auth/login.
- No native-Docker-tab injection (candidate for a later thin companion plugin).

## v1 acceptance

A fresh install on Unraid (CA template) with only the socket mounted shows,
within one poll cycle, every container with its update status; for at least
the LSIO + OCI-labeled images the changelog span renders with a correct risk
badge; Ollama/Matrix stay silent until configured; the container survives a
restart with its history intact (SQLite in /config).
