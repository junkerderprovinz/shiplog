# Update-All Button — Design / Spec

**Date:** 2026-07-04 · **Target release:** v1.3.0 (minor — new feature)

## 1. Goal

A ShipLog-injected button in Unraid's Docker tab, placed **left of the Basic/Advanced
view toggle**, that updates **all containers with a pending update** in one click by
triggering Unraid's own bulk-update flow.

## 2. Locked decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | Confirmation | **None** — one click fires the native flow directly |
| 2 | Label | **With counter**: "Update all (N)" / de "Alle aktualisieren (N)", WebGUI language via ShipLog's existing `T()` dict |
| 3 | Visibility | Rendered **only when N ≥ 1**; hidden at 0 or when the native mechanism is missing |
| 4 | Position | **Left of** `div.ToggleViewMode` (the advanced-view switch) |

## 3. Facts from the real sources (verified, not guessed)

- Unraid ships a native bulk update: `updateAll()` in
  `dynamix.docker.manager/javascript/docker.js` (line ~181). It collects
  `docker[i].update == 1` names and runs
  `openDocker('update_container ' + names.join('*'), …)` — Unraid's own multi-update
  dialog, which also disables the page buttons while running.
- The native trigger `<input type="button" id="updateAll" style="display:none">` lives
  in the page's **bottom** button row and is disabled when no update is pending
  (`DockerContainers.page` line ~60 and ~204).
- The advanced toggle is `<div class="ToggleViewMode"><span><input type="checkbox"
  class="advancedview"></span></div>` at the top of the container table
  (`DockerContainers.page` line ~32), rendered as a jQuery switchButton.
- The page-global `docker` array is ShipLog's same-page JS context — readable directly.

## 4. Behaviour

- **Count `N`** = `(window.docker || []).filter(d => d.update == 1).length` — exactly
  the set the native `updateAll()` will act on, so label and action can never diverge.
- **Click** → `window.updateAll()` when it is a function; fallback: `.click()` on the
  native `#updateAll` input. If neither exists, the button stays hidden (never a dead
  or broken control).
- **Read-only philosophy preserved:** ShipLog performs no Docker action itself — the
  button only relocates the trigger of Unraid's own flow (same principle as the
  existing per-container `triggerNativeUpdate()`).
- While Unraid's update dialog runs, Unraid disables page buttons itself; ShipLog
  re-evaluates count/visibility on its normal tick afterwards.

## 5. Implementation shape (in `plugin/src/shiplog/.../scripts/docker.js`)

- New `injectUpdateAllButton()` called from the existing observer/tick path:
  - Find `div.ToggleViewMode`; if absent → no-op.
  - **Idempotent** via mark guard (`data-sl-updall` / single element with class
    `sl-updall`) — hard rule for Docker-tab injection (observer + non-idempotent DOM =
    CPU loop). Everything wrapped in try/catch; never touch or hide native elements.
  - Insert the button element **before** `ToggleViewMode` (same parent, inline-flex),
    styled in ShipLog's chip look, light/dark aware via the existing `isLightBg()`.
  - Every tick: recompute `N`; update label text; `display:none` when `N === 0`.
- i18n: two new keys in the existing `T()` dict (`updateAll`: "Update all"/"Alle
  aktualisieren"; counter appended as ` (${n})`), plus a `title` tooltip reusing the
  read-only wording ("triggers Unraid's own update for all containers with a pending
  update" / de equivalent).
- CSS: extend the existing stylesheet with `.sl-updall` (chip-like, pointer cursor,
  subtle hover; no `!important` wars with native styles).

## 6. Error handling

- No `ToggleViewMode` (layout change) → button not injected, everything else works.
- No `updateAll`/`#updateAll` (future Unraid change) → button hidden.
- `docker` global missing/malformed → treated as N = 0 → hidden.
- All failures silent (console.debug at most), never a thrown error into the page.

## 7. Verification

- `node --check` on docker.js.
- **Headless-Chromium harness** against a fixture built from the real
  `DockerContainers.page` DOM (ToggleViewMode + a stub `docker` array + stub
  `updateAll()`): asserts (a) button appears left of the toggle with correct count,
  (b) click calls the stub exactly once, (c) N = 0 hides it, (d) double-tick injects
  exactly one element (idempotency).
- CI (Build + Lint) green; release via ShipLog's CI flow: `## 1.3.0` changelog block +
  tag `v1.3.0` → CI builds/publishes the .txz and flips version + SHA on main last.
