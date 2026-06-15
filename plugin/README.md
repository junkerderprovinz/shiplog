# ShipLog — the Unraid plugin (bubble in the native Docker tab)

ShipLog has two parts:

| Part | What it is | Where it runs | Status |
|------|------------|---------------|--------|
| **Engine** | A read-only Go container: reads the Docker socket, resolves running→newest, computes the risk badge, fetches the changelog, exposes a small REST API + status page. | Any Docker host. | **Built** (`ghcr.io/junkerderprovinz/shiplog`). |
| **Plugin** | A thin Unraid `.plg` that injects a risk badge + changelog **bubble next to every container in Unraid's own Docker tab**, reading the engine through a same-origin PHP proxy. | Unraid only. | **In progress** — this folder. |

**Why a plugin is required.** Unraid's Docker tab is server-rendered by emhttp
(`DockerContainers.php`). A container can't draw into it; only code that ships as
an Unraid plugin can add to that page. So the engine is the brain (and works on
any Docker host via its status page), and the plugin is the thin Unraid-specific
layer that renders the bubble where you actually want it. Precedent: the
*Folder View* plugin restructures the whole Docker tab the same way.

## P2.0 — feasibility spike (run this first, on your box)

[`spike/docker-tab-inject.js`](spike/docker-tab-inject.js) proves the DOM hook on
**your** Unraid version before the full plugin is built, and renders the real
bubble design (with demo data) so you can see exactly how it'll look in place.

**Run it (no install):**

1. Open Unraid → the **Docker** tab.
2. Open DevTools (**F12** → **Console**).
3. Paste the whole file, press **Enter**.

A discreet, Unraid-native control appears in each container's **update column**
(next to "Aktualisierung anwenden" / "Auf dem neusten Stand"): a small log
glyph · **Changelog** · a risk traffic-light dot. It's dim link text (no pill),
so it never shifts the row layout. Click it for the bubble. A toast
(bottom-right) reports how many rows were tagged and which selector matched.

**If it does *not* tag your containers:** the script prints a `[ShipLog spike]`
diagnostics block (the Docker table structure). Copy that block back — it pins
down the exact selectors for your Unraid release so the real plugin targets them
precisely.

The spike uses **demo data**. The real plugin wires the bubble to the live engine
through a server-side PHP proxy (same-origin → no CORS, token stays on the
server). To preview live data now, set `ENGINE_URL` at the top of the script
(only works if the engine sends CORS headers; otherwise it falls back to demo).

## Build order

- **P2.0** feasibility spike (this) → confirm selectors on the real box.
- **P2.1** `.plg` skeleton + PHP proxy (`localhost:<PORT>` → engine).
- **P2.2** Docker-tab injection JS (per-row badge + bubble, fed by the proxy).
- **P2.3** plugin settings page (engine URL/port, enable/disable) + packaging.
- **P2.4** graceful degradation (engine down / Unraid update breaks injection →
  Docker tab unaffected, engine status page is the fallback).
