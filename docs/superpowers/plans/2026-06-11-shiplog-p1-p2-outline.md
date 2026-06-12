# ShipLog P1 + P2 — plan outline

> **Status:** outline only. The detailed task-by-task TDD plans (via
> superpowers:writing-plans) are written **after** the P0 Bottich verification
> (spike), because what the real host reveals — which registries/changelog
> sources actually resolve across the user's ~55 containers, GitHub rate-limit
> behaviour, socket/permission realities — directly shapes provider priority and
> the plugin injection. This mirrors BombVault's P0 → spike → P1 sequence.

## Gate before P1: P0 Bottich verification (spike)

Install the CA template on Bottich, mount the socket read-only, and confirm the
engine lists the real fleet with correct current→newest, risk badges, and
changelogs for the OCI-labelled + LSIO images within one poll cycle. Capture:
which images resolve a changelog vs fall back, any registry that 401s/ratelimits,
and the GitHub anonymous-limit headroom. Those findings seed the P1 plan.

---

## P1 — changelog depth, AI, notifications, i18n (engine-side)

Each unit slots into the existing interfaces (`changelog.Provider`, and new
`summarize`/`notify` collaborators the engine already has stubs for), so P1 is
additive — no P0 rework.

**P1.1 — linuxserver.io changelog provider.** A `Provider` that recognises an
LSIO image and fetches its changelog (LSIO publishes a structured changelog; use
their convention/endpoint). Inserted in the chain after the GitHub provider,
before the fallback. *Why first:* the fleet runs many LSIO images, so this is
the biggest single coverage win after the spike confirms it.

**P1.2 — curated mapping provider.** A `Provider` backed by a JSON map
(image → source repo / changelog URL), shipped in the image and override-mounted
from `/config`. Covers popular images that set no OCI source label. Seeded from
the spike's "no changelog found" list.

**P1.3 — Ollama summaries (`summarize` unit).** When `OLLAMA_URL`+`OLLAMA_MODEL`
are set, summarise a fetched changelog into "what changes / breaking / migration"
+ a one-line risk rationale; attach as `model.AISummary` (already in the model),
marked AI-generated in the UI. Off by default; failures fall back to the raw
changelog silently.

**P1.4 — Matrix notify (`notify` unit).** When the `MATRIX_*` vars are set, post
an enriched message (changelog summary + risk + link) the first time a new update
is sighted for a container (dedupe via the stored status). Off by default.

**P1.5 — full 26-locale i18n.** Promote the P0 English string keys to the house
26-locale catalogue (same pattern as featherdrop/BombVault, count-neutral),
wired into the status page; key-parity test across all locales. The same catalogue
later feeds the plugin bubble (P2).

**P1 acceptance:** with Ollama set, an update shows an AI summary; with Matrix
set, a new update posts once with context; LSIO + curated images resolve a
changelog; the status page is localised. All optional features stay silent until
configured. CI: lint + race tests + both-arch boot smoke gate, all green.

---

## P2 — the Unraid plugin (the bubble in the native Docker tab)

The deliverable the user actually wants: a risk badge + changelog bubble next to
each container **in Unraid's own Docker tab**. Only a plugin can render there;
all logic stays in the engine container (the plugin only reads its API).

**P2.0 — feasibility spike (do first, on Bottich).** A throwaway `.plg` (or even
a userscript) that injects a dummy badge next to one container row in the live
Docker tab, to prove the DOM hook against the user's actual Unraid version before
committing to the full plugin. Folder View is the precedent that it's possible;
this confirms the exact selectors.

**P2.1 — `.plg` skeleton + PHP proxy.** Standard Unraid plugin packaging
(install/remove scripts, version, update URL). A PHP endpoint under the plugin
that calls the engine host-locally (`http://localhost:<PORT>/api/...`) so the
browser never makes a cross-origin call (no CORS, token stays server-side).

**P2.2 — Docker-tab injection JS.** Vanilla JS that, on the Docker page, matches
each container row by name, fetches that container's status from the PHP proxy,
and renders the per-row risk badge + the click-to-open bubble (the
`bubble-mockup.html` design: version jump, risk, AI summary, raw changelog,
source link). Reuse the P1 i18n catalogue.

**P2.3 — settings page + packaging.** A small plugin settings page (engine
URL/port, enable/disable). CI: shellcheck the `.plg` install/remove scripts + a
`.plg` XML/lint check. Install path: CA's plugin section (separate from the
container template).

**P2.4 — graceful degradation.** If the engine is down or an Unraid update breaks
the injection, the Docker tab is unaffected (no badges) and the engine's status
page remains the fallback. Document the per-Unraid-release fragility.

**P2 acceptance:** on Unraid, the Docker tab shows a risk badge next to every
container with an update; clicking opens the bubble with the engine's changelog;
it survives the engine restarting; with the engine stopped, the tab is clean and
the status page still works.

---

## Roadmap (post-P2, from the brainstorm — engine-side unless noted)

Update history per container · BombVault backup-status in the bubble (engine
queries BombVault's API, plugin renders) · EOL/deprecation warnings · ignore/pin
rules · CVE delta. Each is an additive engine unit + a bubble field.
