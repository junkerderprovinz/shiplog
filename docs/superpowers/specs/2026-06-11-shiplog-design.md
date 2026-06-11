# ShipLog — design spec (2026-06-11)

⚓ **ShipLog** (repo slug `shiplog`, like BombVault/`bombvault`) is a
self-hosted, read-only **update advisor** for Docker hosts
(Unraid first): it tells you **what actually changes** between the image a
container is running and the newest available one — changelog, version jump,
and a risk verdict — before you press "update".

It deliberately does **not** update anything. Unraid (and the CA auto-update
plugin) already handle applying updates and basic "update available"
notifications; the missing piece is the *intelligence* — "what's in it, and
how risky is it?". ShipLog fills exactly that gap.

## Decisions (brainstorm 2026-06-11)

| Topic | Decision |
|---|---|
| Name | **ShipLog** — CamelCase like BombVault (working title was "Kassandra"; collision check: only small namesakes in unrelated niches, `junkerderprovinz/shiplog` free on GitHub + Docker Hub, no CA entry) |
| Form | **Hybrid (decided 2026-06-11 after UX clarification):** the user's primary UX is an **info bubble next to each container in Unraid's native Docker tab** — only a plugin can render there. So: **engine = Docker container** (all logic/data, isolated, house CI) **+ thin Unraid plugin** (`.plg`) that injects the bubble into the Docker tab and proxies API calls to the engine server-side (PHP → `localhost:<port>`, no CORS). Feasibility precedent: Folder View restructures the whole Docker tab via plugin JS. Known risk: DOM injection is undocumented territory and may need fixing per Unraid release — if it breaks, the engine keeps running and its status page is the fallback. |
| Role | **Advisor only, read-only.** Docker socket mounted read-only; ShipLog never pulls, recreates, or stops anything. |
| Stack | **Go engine** in a single binary (polling, registry calls, scheduling are Go's home turf; image stays tiny) with an embedded **minimal status page** — no SPA build pipeline; the heavy UI work lives in the plugin's bubble (vanilla JS injected into the Docker tab). |
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
   ShipLog says so and shows what it *does* know, never pretends.
4. **Risk badge** (deterministic): digest-only = low · patch = low · minor =
   medium · major = high · non-semver/unknown = grey (with reason).
5. **Optional Ollama summary**: if `OLLAMA_URL` + `OLLAMA_MODEL` are set, the
   fetched changelog is summarized into "What changes / Breaking / Migration
   notes" with a risk rationale. Marked as AI-generated in the UI. Failures
   fall back to the raw changelog silently (logged).
6. **Store** results in SQLite; **notify** via Matrix (if enabled) on first
   sighting of a new update for a container.

## UI

**Primary: the info bubble in Unraid's Docker tab** (rendered by the plugin,
data from the engine API). Anchored to each container row, mockup:
`docs/superpowers/specs/assets/bubble-mockup.{html,png}`:
- header: current → newest, risk badge, "skips N releases";
- AI summary section (marked *KI-generiert · Ollama*) when enabled;
- raw changelog excerpt (mono block);
- footer: resolution source ("OCI label → GitHub releases") + link to the
  full release page; optional deep-link to Unraid's own update action
  (read-only: we link, we never apply).
- A small per-row badge (risk color/dot) so the tab shows at a glance which
  containers have non-trivial updates.

**Secondary: engine status page** (minimal, served by the engine container) —
fallback when the plugin injection is broken by an Unraid update, and the only
UI on non-Unraid hosts: simple table (container, current → newest, risk,
changelog text), last/next poll, provider trace. No SPA framework heroics —
keep it deliberately small; the bubble is the product.

House standards apply to both: Carbon dark `#161616` (risk badges are the
single deliberate colour exception — green/amber/red/grey carry meaning),
dark/light, i18n (26 locales, count-neutral), READY banner in the engine log.

## Configuration (env, all surfaced in the Unraid template; fixed-option fields as dropdowns)

| Var | Default | Notes |
|---|---|---|
| `POLL_INTERVAL` | `6h` | dropdown: 1h/3h/6h/12h/24h |
| `GITHUB_TOKEN` | — | optional; raises the anonymous 60 req/h API limit for changelog fetching |
| `OLLAMA_URL` / `OLLAMA_MODEL` | — | optional AI summaries |
| `MATRIX_HOMESERVER` / `MATRIX_TOKEN` / `MATRIX_ROOM` | — | optional push, off when unset |
| `TZ` | Europe/Vienna | dropdown, full IANA list |
| `PORT` | 8484 | engine API + status page (the plugin proxies to it host-locally) |
| Mounts | `/var/run/docker.sock:ro`, `/config` (SQLite + curated-mapping override) | socket read-only is part of the security story |

## Architecture (units, single binary)

- `collector` — Docker socket client (list containers/images, labels). Interface-mocked in tests.
- `resolver` — OCI registry client (tag list, manifest digests; per-registry auth-free token dance for Docker Hub/GHCR).
- `changelog` — provider chain (oci-source → lsio → curated → fallback), each provider a small interface implementation.
- `risk` — pure function (current, newest, digests) → badge + reason. Golden-table tested.
- `summarize` — Ollama client (optional).
- `notify` — Matrix client (optional).
- `store` — SQLite (modernc.org/sqlite or mattn; pick pure-Go for painless cross-compile).
- `api` — REST consumed by the plugin and the status page
  (`/api/containers`, `/api/container/{id}`, `/api/refresh`).
- `web` — embedded minimal status page (go:embed; no SPA build pipeline needed
  if plain templates suffice).
- **`plugin/` (separate artifact in the same repo)** — the Unraid `.plg`:
  injection JS for the Docker tab (match container rows by name, add badge +
  bubble), a PHP proxy endpoint to the engine (server-side, no CORS), a small
  settings page (engine URL/port, enable/disable). Versioned + released with
  the engine; installable via CA's plugin section.

## Error handling

- Per-container failures never kill the sweep: each row carries its own
  status/reason (e.g. "registry rate-limited", "no changelog found").
- GitHub rate limits: ETag/If-None-Match caching, exponential backoff, optional token.
- Registry quirks (Docker Hub anonymous token, GHCR) isolated in `resolver`.
- Ollama/Matrix failures: log + degrade, never block the core loop.

## Testing

- Go: table-driven unit tests per unit; `httptest` fakes for registries/GitHub/LSIO;
  golden tests for the risk engine and provider chain ordering.
- i18n: key-parity test across all 26 locale files (house pattern) — the same
  string catalogue feeds the engine status page and the plugin bubble.
- Plugin: the injection JS gets a small DOM-fixture test (matches mock Docker-tab
  rows, asserts the badge/bubble mount + the proxy fetch shape); the PHP proxy a
  smoke test that it only forwards to the configured local engine port.
- CI: engine — lint (golangci-lint) + tests + multi-arch build + **boot smoke
  gate on both arches** before any push (house rule; SIGPIPE-safe log
  assertions). Plugin — shellcheck the `.plg` install/remove scripts + a `.plg`
  XML/lint check.

## Non-goals (v1 — YAGNI)

- No updating/recreating/stopping containers (read-only advisor; the bubble
  may deep-link to Unraid's own update button, never apply anything itself).
- No per-container schedules, no private-registry auth, no queue of hosts
  (one host = one ShipLog).
- No auth/login on the engine API (LAN tool; the plugin proxy talks to it
  host-locally).
- No full SPA dashboard — the status page stays minimal by design.

## Roadmap candidates (post-v1, from the brainstorm — engine-side unless noted)

1. **Update history** per container (which version ran when — "since when is X
   broken?"); pairs naturally with BombVault restore points.
2. **BombVault integration**: "last backup of this container: 2 days ago ✓"
   inside the bubble (plugin renders, engine queries BombVault's API).
3. **EOL/deprecation warnings** (LSIO-style "image deprecated, migrate to X") —
   cheap, early candidate.
4. **Ignore/pin rules** (mute container, notify on major only).
5. **CVE delta** ("update fixes N known vulnerabilities") — heavy, needs a
   vulnerability feed; explicitly v2+.
6. Fleet freshness overview on the status page.

## v1 acceptance

On Unraid with the engine container (CA template) + the ShipLog plugin
installed: the Docker tab shows a risk badge next to every container with an
update, and clicking it opens the bubble (changelog span, risk, source) for at
least the LSIO + OCI-labeled images within one poll cycle. With the plugin
absent or broken, the engine's status page shows the same data. Ollama/Matrix
stay silent until configured; history survives a container restart (SQLite in
/config); the engine never writes to the Docker socket.
