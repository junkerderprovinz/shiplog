# ShipLog Scheduled SemVer-gated Auto-Update — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let ShipLog auto-apply container image updates on a schedule, gated by a global SemVer-level threshold (off/patch/minor/major) plus a separate `:latest`/digest toggle, triggering Unraid's native update path.

**Architecture:** A new `internal/autoupdate` package holds the pure eligibility policy, the schedule/cadence, and a serial executor. It runs after the existing sweep, reads the already-classified `model.UpdateStatus.Kind` per container, filters by policy, and calls a new `internal/updater` that invokes Unraid's native update (pull + recreate from the user template). Config is ENV-driven like the rest of the engine; the plugin `.page` gains tabs + the auto-update controls.

**Tech Stack:** Go (host daemon, cgo-free), SQLite (modernc), Unraid plugin (`.page` PHP + JS), env-driven config sourced from `shiplog.cfg` by `rc.shiplog`.

## Global Constraints

- **Unraid-plugin only.** The auto-update WRITE path runs only in the host daemon. The generic `ghcr.io/junkerderprovinz/shiplog` container stays read-only — guard the executor/updater so it is a no-op (and logs why) when not on Unraid (no template dir).
- **Global policy only.** One threshold for all containers. NO per-container state/UI (out of scope).
- **`KindUnknown` (non-SemVer) is never auto-updated.** `KindDigest` (`:latest`) only via the separate digest toggle.
- **Default OFF.** `AUTOUPDATE_ENABLED=no`, `AUTOUPDATE_LEVEL=off`, `AUTOUPDATE_DIGEST=no`, `AUTOUPDATE_DRYRUN=no` — an upgrade changes nothing until the user opts in.
- **Repo texts English; no AI attribution in commits.** Config is env-driven (`config.Config` + `Load()`); a settings change takes effect on `rc.shiplog restart` (existing pattern).
- **Co-developed repo:** `git fetch` + reconcile before each work session and before any tag. Never tag/release without explicit user approval.
- Existing types to build on (do NOT redefine): `model.Kind` (`KindMajor|KindMinor|KindPatch|KindDigest|KindUnknown`), `model.RiskLevel`, `model.UpdateStatus{ID, RunningVersion, NewestTag, NewestDigest, Kind, Risk, RiskReason, ...}` with `HasUpdate()`, and `risk.Classify(cur, newest, curDigest, newDigest) (model.Kind, model.RiskLevel, string)`.

## File Structure

- `internal/autoupdate/policy.go` — pure eligibility: `Level`, `Policy`, `Eligible(model.UpdateStatus, Policy) bool`. NEW.
- `internal/autoupdate/policy_test.go` — table tests. NEW.
- `internal/autoupdate/schedule.go` — `Schedule` (mode+params) + `Due(last, now) bool` / `Next(now)`. NEW.
- `internal/autoupdate/schedule_test.go` — NEW.
- `internal/autoupdate/executor.go` — `Executor` iterates eligible statuses serially, calls the `Updater`, returns a `Result` (per-container outcome). NEW.
- `internal/autoupdate/executor_test.go` — mock updater/lister. NEW.
- `internal/updater/updater.go` — `Updater` interface + `UnraidUpdater` (native pull+recreate from template) + a `NoopUpdater` for non-Unraid. NEW (Unraid impl grounded by Task 1 spike).
- `internal/updater/unraid_test.go` — what is unit-testable (arg building, guard). NEW.
- `internal/config/config.go` — add `AutoUpdate` fields + Load parsing. MODIFY.
- `internal/config/config_test.go` — add parse cases. MODIFY (or NEW if absent).
- `internal/engine/engine.go` — expose a `List()` on the store seam + wire an auto-update tick that runs the executor. MODIFY.
- `internal/notify/*` — a run-summary notify method (reuse Matrix). MODIFY.
- `internal/store/*` — a small `AutoUpdateLog` table + insert/list. MODIFY.
- `cmd/shiplog/main.go` — construct the updater + executor + wire the tick. MODIFY.
- `.../plugins/shiplog/shiplog.page` — tabs + auto-update controls. MODIFY.
- `.../plugins/shiplog/scripts/settings.js` (or inline) — tab switching + form. NEW/MODIFY.
- `.../plugins/shiplog/lang/*.json` (en/de + 24) — new i18n keys. MODIFY.
- `.../plugins/shiplog/server/autoupdate-*.php` — same-origin proxy for the settings/history if the page reads live. NEW (only if needed).

---

### Task 1: SPIKE — the Unraid native container-update hook

**Goal:** determine, on a real Unraid box, the exact host-side mechanism that pulls a container's new image and recreates it from its user template `my-<name>.xml` — the same thing the native "apply update" button and CA Auto Update do — and wrap it behind a stable Go interface. This is discovery + a proof, NOT TDD.

**Files:**
- Create: `internal/updater/updater.go` (the interface + a first `UnraidUpdater` impl)
- Doc: append findings to `docs/superpowers/specs/2026-07-12-shiplog-scheduled-auto-update-design.md` under a new "## Trigger mechanism (resolved)" section.

**Interfaces produced (every later task depends on this exact shape):**
```go
package updater

// Updater applies the newest image for one container, recreating it identically.
type Updater interface {
    // Update pulls the container's newest image and recreates it from its Unraid
    // user template. Returns nil on a successful recreate; an error leaves the
    // container running on its old image (never half-updated).
    Update(ctx context.Context, name string) error
    // Supported reports whether this host can perform updates (Unraid template dir
    // present). The generic container returns false → the executor no-ops.
    Supported() bool
}
```

- [ ] **Step 1: Enumerate the candidate hooks.** On the box (or from CA Auto Update's source), determine which is available + cleanest:
  - (a) Unraid's docker-manager PHP: `/usr/local/emhttp/plugins/dynamix.docker.manager/include/DockerClient.php` / `DockerUpdate.php` — is there a CLI-invokable update? Check for a script under `dynamix.docker.manager/scripts/`.
  - (b) The template path: parse `/boot/config/plugins/dockerMan/templates-user/my-<name>.xml`, `docker pull` the image, `docker stop/rm`, then recreate via Unraid's own create (so labels `net.unraid.docker.*` + WebUI/icon are preserved).
  - (c) How CA Auto Update does it (its plugin scripts) — replicate the proven call.
- [ ] **Step 2: Manual proof.** Pick one throwaway test container with a pending update. Trigger the chosen hook by hand on the box; confirm: new image running, container recreated with identical env/ports/paths/extra-params, WebUI icon + Unraid labels intact, "update ready" cleared.
- [ ] **Step 3: Implement `UnraidUpdater.Update`** to invoke exactly that hook (shell out / exec the script or the pull+recreate sequence). `Supported()` returns `os.Stat("/boot/config/plugins/dockerMan/templates-user")` == nil.
- [ ] **Step 4: Add `NoopUpdater`** (`Update` returns `errors.New("auto-update is Unraid-only")`, `Supported()` false) for the generic container / tests.
- [ ] **Step 5: Record the resolved mechanism** in the spec's new section (which hook, why, the exact command) so the plan is reproducible.
- [ ] **Step 6: Commit.** `git add internal/updater docs/... && git commit -m "spike: Unraid native container-update hook + Updater interface"`

**Deliverable:** a working `UnraidUpdater` proven on-box, plus the interface the executor consumes.

---

### Task 2: Config — auto-update settings

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces produced:**
```go
// On config.Config:
AutoUpdate AutoUpdateConfig
// where
type AutoUpdateConfig struct {
    Enabled   bool          // AUTOUPDATE_ENABLED ("yes"/"no")
    Level     string        // AUTOUPDATE_LEVEL: off|patch|minor|major
    Digest    bool          // AUTOUPDATE_DIGEST
    DryRun    bool          // AUTOUPDATE_DRYRUN
    SchedMode string        // AUTOUPDATE_SCHED_MODE: off|daily|boot|hours|days
    SchedTime string        // AUTOUPDATE_SCHED_TIME "HH:MM" (daily)
    SchedEvery int          // AUTOUPDATE_SCHED_EVERY (hours|days)
}
```

- [ ] **Step 1: Failing test** in `config_test.go`:
```go
func TestLoadAutoUpdateDefaults(t *testing.T) {
    for _, k := range []string{"AUTOUPDATE_ENABLED","AUTOUPDATE_LEVEL","AUTOUPDATE_DIGEST","AUTOUPDATE_DRYRUN","AUTOUPDATE_SCHED_MODE","AUTOUPDATE_SCHED_TIME","AUTOUPDATE_SCHED_EVERY"} {
        t.Setenv(k, "")
    }
    au := config.Load().AutoUpdate
    if au.Enabled || au.Level != "off" || au.Digest || au.DryRun || au.SchedMode != "off" {
        t.Fatalf("defaults must be safe/off, got %+v", au)
    }
}
func TestLoadAutoUpdateParsed(t *testing.T) {
    t.Setenv("AUTOUPDATE_ENABLED","yes"); t.Setenv("AUTOUPDATE_LEVEL","minor")
    t.Setenv("AUTOUPDATE_DIGEST","yes"); t.Setenv("AUTOUPDATE_SCHED_MODE","hours"); t.Setenv("AUTOUPDATE_SCHED_EVERY","6")
    au := config.Load().AutoUpdate
    if !au.Enabled || au.Level != "minor" || !au.Digest || au.SchedMode != "hours" || au.SchedEvery != 6 {
        t.Fatalf("parse failed: %+v", au)
    }
}
```
- [ ] **Step 2: Run** `go test ./internal/config/ -run AutoUpdate -v` → FAIL (field missing).
- [ ] **Step 3: Implement.** Add `AutoUpdateConfig` + the field; in `Load()` add:
```go
AutoUpdate: AutoUpdateConfig{
    Enabled:    yes("AUTOUPDATE_ENABLED"),
    Level:      levelOrOff(env("AUTOUPDATE_LEVEL","off")),
    Digest:     yes("AUTOUPDATE_DIGEST"),
    DryRun:     yes("AUTOUPDATE_DRYRUN"),
    SchedMode:  env("AUTOUPDATE_SCHED_MODE","off"),
    SchedTime:  env("AUTOUPDATE_SCHED_TIME","04:00"),
    SchedEvery: atoiMin1("AUTOUPDATE_SCHED_EVERY"),
},
```
with helpers `yes(k) bool { return strings.EqualFold(os.Getenv(k),"yes") }`, `levelOrOff` (validates one of off/patch/minor/major, else "off"), `atoiMin1` (parse int, min 1, default 6).
- [ ] **Step 4: Run** the tests → PASS.
- [ ] **Step 5: Commit.** `feat(config): auto-update settings (default off)`

---

### Task 3: Policy — eligibility (the testable heart)

**Files:**
- Create: `internal/autoupdate/policy.go`, `internal/autoupdate/policy_test.go`

**Interfaces produced:**
```go
type Level int
const (LevelOff Level = iota; LevelPatch; LevelMinor; LevelMajor)
func ParseLevel(s string) Level                       // off|patch|minor|major → Level (unknown→Off)
type Policy struct { Level Level; Digest bool }
func Eligible(st model.UpdateStatus, p Policy) bool
```

- [ ] **Step 1: Failing table test** `policy_test.go`:
```go
func TestEligible(t *testing.T) {
    up := func(k model.Kind) model.UpdateStatus { return model.UpdateStatus{Kind: k, NewestDigest: "sha256:new", /* HasUpdate true */} }
    cases := []struct{ name string; st model.UpdateStatus; p Policy; want bool }{
        {"patch under patch",  up(model.KindPatch),  Policy{LevelPatch,false}, true},
        {"minor under patch",  up(model.KindMinor),  Policy{LevelPatch,false}, false},
        {"minor under minor",  up(model.KindMinor),  Policy{LevelMinor,false}, true},
        {"major under minor",  up(model.KindMajor),  Policy{LevelMinor,false}, false},
        {"major under major",  up(model.KindMajor),  Policy{LevelMajor,false}, true},
        {"digest off",         up(model.KindDigest), Policy{LevelMajor,false}, false},
        {"digest on",          up(model.KindDigest), Policy{LevelOff,true},   true},
        {"unknown never",      up(model.KindUnknown),Policy{LevelMajor,true}, false},
        {"off nothing",        up(model.KindPatch),  Policy{LevelOff,false},  false},
    }
    for _, c := range cases {
        if got := Eligible(c.st, c.p); got != c.want { t.Errorf("%s: got %v want %v", c.name, got, c.want) }
    }
}
```
(Verify `HasUpdate()` returns true for these fixtures; set whatever field it checks — likely a non-empty `NewestDigest` differing from the running digest, per model.go. If `HasUpdate` needs more, populate it.)
- [ ] **Step 2: Run** `go test ./internal/autoupdate/ -run Eligible -v` → FAIL (undefined).
- [ ] **Step 3: Implement `policy.go`:**
```go
func Eligible(st model.UpdateStatus, p Policy) bool {
    if !st.HasUpdate() { return false }
    switch st.Kind {
    case model.KindPatch: return p.Level >= LevelPatch
    case model.KindMinor: return p.Level >= LevelMinor
    case model.KindMajor: return p.Level >= LevelMajor
    case model.KindDigest: return p.Digest
    default: return false // KindUnknown and anything unclassified
    }
}
```
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit.** `feat(autoupdate): eligibility policy`

---

### Task 4: Schedule — cadence

**Files:** Create `internal/autoupdate/schedule.go`, `schedule_test.go`

**Interfaces produced:**
```go
type Schedule struct { Mode string; Time string; Every int } // mode: off|daily|boot|hours|days
// Due reports whether a run is due at now, given the last run time (zero = never ran).
func (s Schedule) Due(last, now time.Time) bool
```

- [ ] **Step 1: Failing test** covering: `off`→never; `daily` due when now passed HH:MM and last run was before today's HH:MM; `hours`/`days` due when `now.Sub(last) >= every*unit`; `boot` due exactly once when `last.IsZero()`. Use fixed clocks.
```go
func TestScheduleDue(t *testing.T) {
    base := time.Date(2026,7,12,4,0,0,0,time.Local)
    if (Schedule{Mode:"off"}).Due(time.Time{}, base) { t.Fatal("off never due") }
    if !(Schedule{Mode:"boot"}).Due(time.Time{}, base) { t.Fatal("boot due when never ran") }
    if (Schedule{Mode:"boot"}).Due(base.Add(-time.Hour), base) { t.Fatal("boot not due after first run") }
    if !(Schedule{Mode:"hours",Every:6}).Due(base.Add(-7*time.Hour), base) { t.Fatal("hours due after 7h") }
    if (Schedule{Mode:"hours",Every:6}).Due(base.Add(-5*time.Hour), base) { t.Fatal("hours not due after 5h") }
    if !(Schedule{Mode:"daily",Time:"04:00"}).Due(base.Add(-25*time.Hour), base) { t.Fatal("daily due past HH:MM") }
}
```
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** `Due` for each mode (parse "HH:MM"; for `daily`, due when `now >= today@HH:MM` AND `last < today@HH:MM`; `hours`/`days` compare `now.Sub(last)` to the interval; `boot` = `last.IsZero()`; `off` = false).
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit.** `feat(autoupdate): schedule cadence`

---

### Task 5: Executor — serial run over eligible statuses

**Files:** Create `internal/autoupdate/executor.go`, `executor_test.go`

**Interfaces produced:**
```go
type Lister interface { List() ([]model.UpdateStatus, error) } // the store
type Executor struct { list Lister; upd updater.Updater; log func(model.UpdateStatus, string, error) }
type Outcome struct { Name, From, To, Level string; Updated bool; Err error }
type Result struct { Outcomes []Outcome; DryRun bool }
func (e *Executor) Run(ctx context.Context, p Policy, dryRun bool) Result
```

- [ ] **Step 1: Failing test** with a fake Lister (3 statuses: 1 patch, 1 major, 1 unknown) + a fake Updater recording calls + one made to error:
```go
func TestExecutorSerialEligibleOnly(t *testing.T) {
    list := fakeLister{sts: []model.UpdateStatus{stPatch("a"), stMajor("b"), stUnknown("c")}}
    upd := &fakeUpdater{}
    e := New(list, upd, nil)
    res := e.Run(context.Background(), Policy{Level: LevelPatch}, false)
    // only "a" (patch under patch) is updated; b (major) and c (unknown) skipped
    if got := upd.calls; !reflect.DeepEqual(got, []string{"a"}) { t.Fatalf("updated %v, want [a]", got) }
    if n := countUpdated(res); n != 1 { t.Fatalf("outcomes updated=%d want 1", n) }
}
func TestExecutorDryRunAppliesNothing(t *testing.T) {
    upd := &fakeUpdater{}
    e := New(fakeLister{sts: []model.UpdateStatus{stPatch("a")}}, upd, nil)
    res := e.Run(context.Background(), Policy{Level: LevelMajor}, true)
    if len(upd.calls) != 0 { t.Fatal("dry-run must not call Update") }
    if !res.DryRun || !res.Outcomes[0].Updated { t.Fatal("dry-run outcome should mark would-update") }
}
func TestExecutorFailureDoesNotAbort(t *testing.T) {
    upd := &fakeUpdater{failOn: "a"}
    e := New(fakeLister{sts: []model.UpdateStatus{stPatch("a"), stPatch("d")}}, upd, nil)
    res := e.Run(context.Background(), Policy{Level: LevelPatch}, false)
    if upd.calls[len(upd.calls)-1] != "d" { t.Fatal("must continue past a failure to d") }
    if res.Outcomes[0].Err == nil || res.Outcomes[1].Err != nil { t.Fatal("a failed, d succeeded") }
}
```
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement `Run`:** list statuses; for each, `if !Eligible(st,p) { continue }`; build an Outcome (Name=st.ID, From=st.RunningVersion, To=st.NewestTag, Level=kind string); if `dryRun` mark Updated=true without calling; else `err := upd.Update(ctx, st.ID)`, set `Outcome.Updated = err==nil`, `Outcome.Err = err`, call the `log` hook; serial (no goroutines). Guard: if `!upd.Supported()` return an empty Result (log once).
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit.** `feat(autoupdate): serial executor with dry-run + failure isolation`

---

### Task 6: Store — auto-update history

**Files:** Modify `internal/store/*` (add a table + methods); add a test.

**Interfaces produced:**
```go
// On the store:
func (s *Store) List() ([]model.UpdateStatus, error)                 // if not already present (satisfies executor.Lister)
func (s *Store) LogAutoUpdate(rec model.AutoUpdateRecord) error
func (s *Store) AutoUpdateHistory(limit int) ([]model.AutoUpdateRecord, error)
// model.AutoUpdateRecord{ Name, FromVer, ToVer, Level string; Success bool; Err string; At int64 }
```

- [ ] **Step 1: Failing test:** open an in-memory/temp store, `LogAutoUpdate` two records, `AutoUpdateHistory(10)` returns them newest-first with fields intact.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement:** `CREATE TABLE IF NOT EXISTS autoupdate_log (name TEXT, from_ver TEXT, to_ver TEXT, level TEXT, success INTEGER, err TEXT, at INTEGER)` in the migration path; the insert + the ordered select; add `List()` if the store lacks a public list (the API layer already lists — reuse/expose it).
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit.** `feat(store): auto-update history`

---

### Task 7: Notify — run summary

**Files:** Modify `internal/notify/matrix.go` (add a summary method) + wire the executor's log/summary through it; test the message rendering.

**Interfaces produced:**
```go
// Notifier gains (new interface method used only by the executor path):
type RunNotifier interface { NotifyAutoUpdate(ctx context.Context, res autoupdate.Result) error }
```

- [ ] **Step 1: Failing test** for the message builder (pure): given a Result with 2 updated + 1 failed, the rendered text contains each name, from→to, level, and the failure reason; dry-run renders "would update".
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** a `renderAutoUpdate(res) string` (pure, tested) + `NotifyAutoUpdate` that sends it via the existing Matrix send path. No new channel.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit.** `feat(notify): auto-update run summary`

---

### Task 8: Engine wiring — the auto-update tick

**Files:** Modify `internal/engine/engine.go` + `cmd/shiplog/main.go`.

**Interfaces consumed:** `config.AutoUpdateConfig`, `autoupdate.{Policy,Schedule,Executor}`, `updater.Updater`, store `List`/`LogAutoUpdate`, `RunNotifier`.

- [ ] **Step 1: Failing test** (engine-level, fake clock + fake executor): with `AutoUpdate.Enabled=true`, `SchedMode="boot"`, after the first sweep the engine invokes the executor exactly once; with `Enabled=false` it never does.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement:** in the engine run loop, after a sweep (or on a dedicated ticker), if `cfg.AutoUpdate.Enabled` and `schedule.Due(lastRun, now)`, run `executor.Run(ctx, policy, dryRun)`, persist each Outcome via `store.LogAutoUpdate`, then `notifier.NotifyAutoUpdate(res)`, and record `lastRun`. In `main.go`: build `updater.NewUnraid()` (falls back to Noop when `!Supported()`), the `Executor`, and pass the `AutoUpdateConfig` in. Keep it OFF when `!Supported()` even if enabled (log a one-time warning "auto-update requires the Unraid plugin").
- [ ] **Step 4: Run** → PASS; `go build ./... && go vet ./...`.
- [ ] **Step 5: Commit.** `feat(engine): scheduled auto-update tick`

---

### Task 9: Settings UI — tabs + auto-update controls

**Files:** Modify `.../plugins/shiplog/shiplog.page`; add/modify a settings JS for tab switching; write config back to `shiplog.cfg`; `rc.shiplog restart` on save (existing pattern).

- [ ] **Step 1:** Wrap the existing settings sections in a tab shell (JS-driven, mirror CannonadeCommand's `.page` tab pattern — read `plugin/src/.../settings` there if present, else a minimal `hidden`-class toggle). Tabs: **Auto-Update / Notifications / Checker / General**. Move existing controls under the right tabs (no behavior change).
- [ ] **Step 2:** Add the Auto-Update tab controls bound to the `AUTOUPDATE_*` cfg keys: master toggle, level select (off/patch/minor/major), digest toggle, dry-run toggle, schedule mode select + time/every inputs (show/hide per mode). Use ShipLog's existing form-save mechanism (writes `shiplog.cfg`, restarts the daemon).
- [ ] **Step 3:** `node --check` any new JS; load the settings page on the box, flip each control, confirm it persists to `shiplog.cfg` and the daemon restarts with the new env.
- [ ] **Step 4: Commit.** `feat(ui): settings tabs + auto-update controls`

---

### Task 10: CA Auto Update coexistence warning

**Files:** the Auto-Update tab (a server-side check via a small PHP or the engine).

- [ ] **Step 1:** Detect CA Auto Update: `is_dir('/usr/local/emhttp/plugins/ca.update.applications')` (or its known slug — verify on box). Surface a boolean to the page.
- [ ] **Step 2:** When present AND ShipLog auto-update is enabled, render a warning banner in the tab ("CA Auto Update is also installed — both will update containers; use one to avoid double updates.").
- [ ] **Step 3: Commit.** `feat(ui): warn when CA Auto Update coexists`

---

### Task 11: i18n + docs

**Files:** `lang/en.json`, `lang/de.json` + the 24 other `lang/*.json`; `README.md`; a release note draft.

- [ ] **Step 1:** Add the new UI keys to `en.json` + `de.json` (write both yourself). Keys for: tab titles, every auto-update control label + hint, the dry-run/CA-warning strings.
- [ ] **Step 2:** Translate into the 24 remaining locales (translate-all-locales convention) — one writer, verify key parity across all files (same key set as en).
- [ ] **Step 3:** README: add an "Auto-update" section (what it does, Unraid-only, level semantics, dry-run first). Draft `.github/release-notes/<next>.md` (do NOT tag — release is user-approved).
- [ ] **Step 4: Commit.** `docs+i18n: auto-update strings, README, release note`

---

## Self-Review

**Spec coverage:** global threshold → Tasks 2/3; digest toggle → 2/3; unknown-never → 3; flexible schedule → 2/4; Unraid-native trigger → 1/5/8; tabs → 9; dry-run → 2/5/8; CA warning → 10; notify+history → 6/7/8; Unraid-plugin-only guard → 1(`Supported`)/5/8; out-of-scope (per-container/exclude/ghcr-write) → honored (Noop guard, no per-container UI). All covered.

**Placeholder scan:** Task 1 is a spike by nature (discovery), with a concrete interface deliverable + on-box proof — not a code placeholder. All other tasks carry real code/tests. Config keys, types, and signatures are concrete.

**Type consistency:** `model.Kind`/`UpdateStatus`/`HasUpdate` used as defined in the real `internal/model`; `updater.Updater{Update,Supported}` consumed identically in Tasks 5/8; `autoupdate.{Policy,Level,Schedule,Executor,Result,Outcome}` names consistent across Tasks 3/4/5/7/8; `config.AutoUpdateConfig` fields consistent Tasks 2/8. `Eligible` signature stable.

**Risk note:** Task 1 (the Unraid hook) is the one item needing the real box; everything in Tasks 2–7 is pure Go, unit-testable on any machine. Sequence: build 2–7 locally (fully tested), then 1/8/9/10 on/for the box.
