# ShipLog — Scheduled, SemVer-gated Auto-Update — Design

**Status:** approved (brainstorming), pending implementation plan
**Date:** 2026-07-12

## Goal

Let ShipLog apply container image updates automatically on a schedule, gated by
SemVer level — auto-apply patches, hold minors/majors, or any threshold the user
picks — a smarter alternative to CA Auto Update (which updates blindly regardless
of version bump). ShipLog already classifies each container's available update as
major / minor / patch / digest; this feature adds the scheduling, the policy, and
the headless update trigger on top of that existing classification.

## Scope & platform

- **Unraid plugin only.** The engine runs as a host daemon on Unraid, so it can
  invoke Unraid's own container-update path (pull + recreate from the user
  template). The generic `ghcr.io/junkerderprovinz/shiplog` container stays a
  **read-only advisor** — it has no host template access and must not write.
- **Global policy**, not per-container (user decision). One threshold governs all
  containers. Per-container overrides and an exclude list are explicitly **out of
  scope** for v1 (noted as a possible future extension).

## Requirements (decided)

1. **Global level threshold:** `off` (default) | `patch` | `minor` | `major`.
   Auto-apply an update when its bump level is at or below the threshold
   (`major` allows all; `patch` allows only patch).
2. **Separate digest toggle** for `:latest` / digest-only updates (`KindDigest`),
   independent of the level threshold, because such updates carry no comparable
   SemVer level. Off by default.
3. **Unknown bumps** (non-SemVer tags, `KindUnknown`) are **never** auto-updated —
   they always require a manual update.
4. **Flexible schedule:** `off` | daily at `HH:MM` | at boot (once, after the
   array is online) | every N hours | every N days.
5. **Trigger via Unraid's native update path** (pull + recreate from
   `my-<name>.xml`), so containers come back identical to a manual update.
6. **Settings organised into tabs** (like CannonadeCommand), JS-driven in the
   `.page`: Auto-Update / Notifications / Checker / General.
7. **Dry-run mode** ("would-update" — notify only, apply nothing) so the user can
   build trust before arming real updates.
8. **CA Auto Update coexistence warning** when that plugin is detected (both
   arming updates would double-update).
9. **Notification + history** after each run, via the existing notifier.

## Architecture

New Go package `internal/autoupdate`, built on the existing sweep (which already
produces `model.UpdateStatus{Kind, Risk, RunningVersion, NewestTag, ...}` per
container via resolver → risk → changelog → store):

- **Policy** (pure): reads config (threshold, digest toggle, schedule, dry-run)
  and decides, given a container's `UpdateStatus`, whether it is eligible. This is
  the testable heart — no I/O.
- **Scheduler**: a cadence-driven tick (its own loop, or piggybacked on the sweep
  loop) that, when due, asks the store for current statuses and runs the executor.
  The daemon is already gated on array-online by `rc.shiplog`.
- **Executor**: iterates eligible containers **serially**, calls the updater, and
  aggregates the outcome for the notifier + history.

New package `internal/updater`: the headless Unraid-native update invocation
(pull + recreate from the user template). See "Trigger" below — this is the one
area that needs a short implementation spike.

Touched existing packages: `internal/config` (new settings), `internal/store`
(update history), `internal/notify` (a run-summary message), `internal/api` (a
config/history endpoint if the settings page reads it live), and the plugin
`.page` files (tabs + auto-update controls).

## Eligibility logic (pure function)

```
eligible(status, policy):
  if !status.HasUpdate():           return false
  switch status.Kind:
    case Patch:   return policy.threshold >= patch
    case Minor:   return policy.threshold >= minor
    case Major:   return policy.threshold >= major
    case Digest:  return policy.digestAuto            // :latest, level-less
    case Unknown: return false                        // never auto
```

Threshold ordering: `off(0) < patch(1) < minor(2) < major(3)`. `off` → nothing is
eligible (feature effectively disabled).

## Trigger (Unraid-native) — needs a spike

The executor applies an update the same way the native "apply update" button and
CA Auto Update do: pull the new image, then recreate the container from its Unraid
user template `/boot/config/plugins/dockerMan/templates-user/my-<name>.xml` (so
env / ports / paths / extra params are preserved — NOT reconstructed from
`docker inspect`, which can lose template-only settings).

**Spike (first task of the plan):** pin down the exact host hook — either invoke
Unraid's docker-manager update helper/script, or script `docker pull` + a recreate
that parses the template. CA Auto Update proves a host-side path exists; the spike
picks the cleanest one and confirms it recreates a test container identically.

Updates run one at a time (no parallel pulls); each is logged; a failure is
notified and the container is left as-is, then the batch continues.

## Config (shiplog.cfg keys, tentative)

```
AUTOUPDATE_ENABLED="no"        # master on/off
AUTOUPDATE_LEVEL="off"         # off | patch | minor | major
AUTOUPDATE_DIGEST="no"         # auto-apply :latest/digest updates
AUTOUPDATE_DRYRUN="no"         # notify only, apply nothing
AUTOUPDATE_SCHED_MODE="daily"  # off | daily | boot | hours | days
AUTOUPDATE_SCHED_TIME="04:00"  # for daily
AUTOUPDATE_SCHED_EVERY="6"     # for hours/days
```

## Settings UX (tabs)

Reorganise the existing settings `.page` into JS-driven tabs (mirror CC's approach
so the page stays tidy as it grows): **Auto-Update** (this feature's controls),
**Notifications** (existing Matrix/etc.), **Checker** (existing Docker Hub / GitHub
credential checks), **General**. No functional change to the existing settings —
just grouped under tabs. All new i18n keys go into `en`/`de` plus the 24 locale
JSON files (translate-all-locales convention).

## Notification + history

After a run, one summary via `internal/notify` (Matrix today): e.g.
`Auto-updated 2 containers: plex 1.2.3→1.2.4 (patch), sonarr 4.0→4.1 (minor); 1 failed: radarr (pull error)`.
Dry-run sends the same as "would update …". Each action is recorded in `store`
(container, from→to, level, outcome, timestamp) for an audit trail / possible UI.

## Error handling

- Pull / recreate failure → log + notify, leave the container running on the old
  image, continue the batch. Never leave a container half-updated/broken.
- Registry 429 / unreachable is already handled by the sweep; the executor acts
  only on already-classified statuses.
- If the array is not online / template missing → skip that container, log why.

## Testing

- **Policy** — table tests over threshold × Kind (patch/minor/major/digest/unknown)
  × digest-toggle; assert exactly the expected containers are eligible.
- **Schedule** — cadence parse + "is it due now" for each mode.
- **Executor** — mock updater: only eligible containers triggered, serial order, a
  mid-batch failure does not abort the rest, summary reflects successes/failures.
- **Updater (Unraid-native)** — hard to unit-test (host-specific); validated by the
  spike + manual on-box run, with dry-run as the safe first gate.

## Out of scope (v1)

- Per-container level overrides and an exclude list (global-only was chosen).
- Auto-update in the generic ghcr container (stays read-only advisory).
- Rollback (BombVault's job; ShipLog only updates).

## Open questions / risks

- The exact Unraid update hook (spike, first plan task) — the main risk.
- Whether to piggyback the auto-update tick on the existing sweep loop or add a
  dedicated timer (implementation detail, decided in the plan).

## Trigger mechanism (resolved — Task 1 spike, off-box research)

The Unraid updater calls Unraid's OWN native container-update entrypoint — the
exact code the "apply update" button and CA Auto Update run — headless as root:

```
/usr/local/emhttp/plugins/dynamix.docker.manager/scripts/update_container '<url-encoded name>'
```

It reads the user template `/boot/config/plugins/dockerMan/templates-user/my-<Name>.xml`,
pulls the newest image, and recreates the container identically (env/ports/paths/
network + the load-bearing `net.unraid.docker.*` labels — WebUI/icon/managed).
Result is byte-identical to a manual update, and future Unraid changes come for free.

Contract used by `internal/updater` (`Unraid.Update`):
- `name` is `argv[1]`, `url.QueryEscape`'d (the script `rawurldecode`s then splits
  on `*`, its multi-container delimiter).
- Guard: verify `my-<Name>.xml` exists (clean "no template" error) and the script
  exists before invoking.
- **Success = the process exit code** (0 ok). Do NOT rely on the nchan progress
  publish. Do NOT pre-stop the container (the script preserves running state).
- ShipLog only calls it for a container it already classified as having an eligible
  update, so the script's unconditional pull+recreate never touches an up-to-date one.
- Fallback for Unraid ≤6.9 (the `CreateDocker.php` `$_GET['updateContainer']` include
  shim) is documented but NOT implemented — ShipLog targets 6.12+/7.x; on an older
  box `Update` returns a clear "update script not found" error.

**MUST be verified on a real box** before trusting in production (the two blockers):
1. the `update_container` path exists + is executable on the target Unraid build;
2. exit-code semantics on failure (a failed pull / missing template returns non-zero)
   — the script was written for a browser flow, not a caller checking `$?`.
Also verify: running-state preservation, label/WebUI/icon fidelity, and a br0 static-IP
container keeps its IP.
