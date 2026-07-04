# Update-All Button Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A ShipLog-injected "Update all (N)" button left of Unraid's Basic/Advanced view toggle that fires Unraid's own native bulk-update flow.

**Architecture:** One new idempotent injector function in the existing `docker.js` (same pattern as `tagRows()`), driven by the existing MutationObserver. Count from the page-global `docker` array (`d.update == 1`), action = native `window.updateAll()`. No engine dependency — the button works even when the ShipLog engine is unreachable, so it is wired BEFORE the engine gate in `boot()`.

**Tech Stack:** Vanilla JS (Unraid page context), CSS, headless Chrome (`--dump-dom` + PASS markers) for verification, ShipLog CI release flow.

## Global Constraints

- Label: en `Update all (N)` / de `Alle aktualisieren (N)` via the existing `T()` dict; rendered only when `N >= 1`.
- Position: inserted immediately BEFORE `div.ToggleViewMode` (same parent).
- Click: `window.updateAll()` if function, else `.click()` on `input#updateAll`; neither present → button hidden.
- Injection MUST be idempotent (single element, guarded) and MUST NOT write identical values on every observer tick (textContent/style writes only on change — otherwise the MutationObserver feeds itself into a CPU loop).
- Never touch/hide native elements; everything try/catch; failures silent.
- `.plg` `<CHANGES>` must stay valid XML (no raw `<` or `&`).
- Release: v1.3.0 via ShipLog CI flow (`### 1.3.0` block in `plugin/shiplog.plg` `<CHANGES>`, then tag `v1.3.0`).

---

### Task 1: Injector + i18n + CSS

**Files:**
- Modify: `plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/scripts/docker.js` (three spots: STR dict ~line 43/52, new function before `tagRows()`, `boot()` ~line 368)
- Modify: `plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/styles/docker.css` (append)

**Interfaces:**
- Consumes: existing `T(k)`, `el(tag, cls, html)`, `esc(s)`, `isLightBg()` helpers in docker.js.
- Produces: `injectUpdateAllButton()` — no args, no return; safe to call on every tick.

- [ ] **Step 1: Add the two i18n keys** to BOTH language blocks in the `STR` dict.

In `en` (after the `updateGone` line):
```js
      updateAll: "Update all", updateAllHint: "triggers Unraid's own update for every container with a pending update",
```
In `de` (after the German `updateGone` line):
```js
      updateAll: "Alle aktualisieren", updateAllHint: "löst Unraids eigenes Update für alle Container mit anstehendem Update aus",
```

- [ ] **Step 2: Add the injector** directly above the `// ──── inject` / `function tagRows()` block:

```js
  // ─────────────────────────────────────────────── update-all button
  // A ShipLog-placed trigger for Unraid's OWN bulk update: count comes from the
  // page-global `docker` array (exactly the set native updateAll() acts on) and
  // the click calls native updateAll() — ShipLog stays read-only. Injected left
  // of the Basic/Advanced toggle; visible only while at least one update is
  // pending. Writes are change-guarded so the MutationObserver that drives this
  // never feeds on our own DOM writes.
  function pendingUpdateCount() {
    try {
      const list = window.docker;
      if (!Array.isArray(list)) return 0;
      return list.filter((d) => d && Number(d.update) === 1).length;
    } catch (e) { return 0; }
  }

  function nativeUpdateAllAvailable() {
    return typeof window.updateAll === "function" || !!document.getElementById("updateAll");
  }

  function fireNativeUpdateAll() {
    try {
      if (typeof window.updateAll === "function") { window.updateAll(); return true; }
      const inp = document.getElementById("updateAll");
      if (inp) { inp.click(); return true; }
    } catch (e) {}
    return false;
  }

  function injectUpdateAllButton() {
    try {
      const toggle = document.querySelector("div.ToggleViewMode");
      if (!toggle || !toggle.parentNode) return;

      let btn = document.querySelector(".sl-updall");
      if (!btn) {
        btn = el("a", "sl-updall", "");
        btn.href = "#";
        btn.title = T("updateAllHint");
        if (isLightBg()) btn.classList.add("sl-light");
        btn.addEventListener("click", (e) => {
          e.preventDefault(); e.stopPropagation();
          fireNativeUpdateAll();
        });
        toggle.parentNode.insertBefore(btn, toggle);
      }

      const n = nativeUpdateAllAvailable() ? pendingUpdateCount() : 0;
      const label = `${T("updateAll")} (${n})`;
      // change-guarded writes: identical values must not touch the DOM
      if (btn.dataset.slN !== String(n)) {
        btn.dataset.slN = String(n);
        btn.textContent = label;
      }
      const want = n >= 1 ? "" : "none";
      if (btn.style.display !== want) btn.style.display = want;
    } catch (e) {}
  }
```

- [ ] **Step 3: Wire it into `boot()`** — button + observer live BEFORE the engine gate (the feature needs no engine). Replace the existing `boot()`:

```js
  async function boot() {
    // Native-data feature first: the update-all button works without the engine.
    injectUpdateAllButton();
    // Unraid re-renders the table on its auto-refresh — re-tag rows and keep the
    // update-all counter current. tagRows() is a no-op while byName is empty.
    const mo = new MutationObserver(() => { tagRows(); injectUpdateAllButton(); });
    try { mo.observe(document.body, { childList: true, subtree: true }); } catch (e) {}

    const ok = await load();
    if (!ok) return; // engine unreachable → chips stay off, button still works
    const n = tagRows();
    console.log(`${TAG} tagged ${n} container(s) with updates`);
  }
```
(The old `boot()` created the observer with only `tagRows()` AFTER the gate — this replaces it; no second observer may remain.)

- [ ] **Step 4: Append the button style** to `styles/docker.css`:

```css
/* ── update-all button (left of the Basic/Advanced toggle) ─────────────── */
a.sl-updall {
  display: inline-block;
  margin-right: 12px;
  padding: 2px 10px;
  border: 1px solid rgba(128, 128, 128, 0.55);
  border-radius: 10px;
  font-size: 12px;
  text-decoration: none;
  cursor: pointer;
  vertical-align: middle;
  white-space: nowrap;
}
a.sl-updall:hover { border-color: rgba(128, 128, 128, 0.95); }
```

- [ ] **Step 5: Syntax check**

Run: `node --check "plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/scripts/docker.js"`
Expected: no output (exit 0).

- [ ] **Step 6: Commit**

```bash
git add plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/scripts/docker.js plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/styles/docker.css
git commit -m "feat: update-all button next to the advanced-view toggle (native updateAll trigger)"
```

---

### Task 2: Headless-Chrome verification harness

**Files:**
- Create: `plugin/spike/verify-updall.html`

**Interfaces:**
- Consumes: the real `docker.js` from Task 1 (loaded relative: `../src/shiplog/usr/local/emhttp/plugins/shiplog/scripts/docker.js`).
- Produces: a `#result` div whose text contains `ALL-PASS` when every assertion holds.

- [ ] **Step 1: Write the fixture** (real ToggleViewMode DOM from unraid/webgui `DockerContainers.page:32`, stub `docker` array + `updateAll()`):

```html
<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>sl-updall verify</title></head>
<body>
<!-- real Unraid structure: the toggle the button must sit LEFT of -->
<div id="host">
  <div class="ToggleViewMode"><span><input type="checkbox" class="advancedview"></span></div>
</div>
<table><tbody id="docker_list"></tbody></table>
<div id="result">RUNNING</div>
<script>
  // native stubs (what DockerContainers.page provides)
  window.docker = [
    { name: "a", update: 1 }, { name: "b", update: 0 }, { name: "c", update: 1 },
  ];
  window.__calls = 0;
  window.updateAll = function () { window.__calls++; };
</script>
<script src="../src/shiplog/usr/local/emhttp/plugins/shiplog/scripts/docker.js"></script>
<script>
  (async () => {
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
    const fails = [];
    await sleep(300); // let boot() run (its fetch to the absent engine rejects)

    const btns = document.querySelectorAll(".sl-updall");
    if (btns.length !== 1) fails.push("inject-count=" + btns.length);
    const btn = btns[0];
    if (btn && btn.nextElementSibling !== document.querySelector("div.ToggleViewMode"))
      fails.push("not-left-of-toggle");
    if (btn && !/Update all \(2\)/.test(btn.textContent)) fails.push("label=" + (btn && btn.textContent));
    if (btn && btn.style.display === "none") fails.push("hidden-despite-updates");

    if (btn) btn.click();
    if (window.__calls !== 1) fails.push("calls=" + window.__calls);

    // idempotency under mutations + hide at zero
    window.docker.forEach((d) => (d.update = 0));
    document.body.appendChild(document.createElement("i")); // force observer tick
    await sleep(300);
    if (document.querySelectorAll(".sl-updall").length !== 1) fails.push("dup-after-tick");
    if (document.querySelector(".sl-updall").style.display !== "none") fails.push("visible-at-zero");

    document.getElementById("result").textContent =
      fails.length ? "FAIL: " + fails.join(", ") : "ALL-PASS";
  })();
</script>
</body></html>
```

- [ ] **Step 2: Run it headless and assert ALL-PASS**

Run (Git Bash):
```bash
cd "d:/nextcloud/it/github/shiplog"
"/c/Program Files/Google/Chrome/Application/chrome.exe" --headless=new --disable-gpu \
  --virtual-time-budget=4000 --dump-dom "file:///D:/nextcloud/it/github/shiplog/plugin/spike/verify-updall.html" \
  | grep -o "ALL-PASS\|FAIL: [^<]*"
```
Expected output: `ALL-PASS`. Any `FAIL: …` names the broken assertion — fix Task 1 and re-run.

- [ ] **Step 3: Commit**

```bash
git add plugin/spike/verify-updall.html
git commit -m "test: headless-chrome harness for the update-all button"
```

---

### Task 3: README + release v1.3.0 + feed sync

**Files:**
- Modify: `README.md` (feature bullet in the feature list)
- Modify: `plugin/shiplog.plg` (`<CHANGES>`: new `### 1.3.0` block at the top; XML must stay valid — no raw `<`/`&`)
- Modify: `d:\nextcloud\it\github\unraid-apps\shiplog\shiplog.xml` (`<Overview>`: mention the one-click update-all)

**Interfaces:** none.

- [ ] **Step 1: README** — add to the feature list (matching its existing bullet style):

```markdown
- **Update all in one click** — a counter button next to the Basic/Advanced toggle
  triggers Unraid's own bulk update for every container with a pending update
  (ShipLog itself stays read-only).
```

- [ ] **Step 2: `.plg` changelog block** — prepend inside `<CHANGES>`:

```
### 1.3.0
- NEW: "Update all (N)" button next to the Basic/Advanced view toggle. One click
  triggers Unraid's own bulk update for every container with a pending update.
  ShipLog stays read-only: it only relocates the native trigger.
```

- [ ] **Step 3: Validate the .plg as XML**

Run: `powershell -Command "try { [xml](Get-Content 'plugin/shiplog.plg' -Raw) | Out-Null; 'XML valid' } catch { $_.Exception.Message }"`
Expected: `XML valid`.

- [ ] **Step 4: Commit, push, tag** (CI builds/publishes the .txz and flips version+SHA on main last):

```bash
git add README.md plugin/shiplog.plg
git commit -m "docs: v1.3.0 changelog + README bullet (update-all button)"
git push origin main
git tag v1.3.0
git push origin v1.3.0
```

- [ ] **Step 5: Watch the release CI to green**

Run: `gh run list --repo junkerderprovinz/shiplog --limit 3` then `gh run watch <release-run-id> --repo junkerderprovinz/shiplog --exit-status`
Expected: release workflow green; release `v1.3.0` exists with the `.txz` asset; `plugin/shiplog.plg` on main carries version 1.3.0 + new SHA (CI commit).

- [ ] **Step 6: Feed overview sync** — in `unraid-apps/shiplog/shiplog.xml` extend the `<Overview>` feature sentence with the update-all button, validate XML the same way, then:

```bash
cd d:/nextcloud/it/github/unraid-apps
git add shiplog/shiplog.xml
git commit -m "docs(shiplog): overview mentions the one-click update-all button"
git push origin main
```

---

## Self-Review

- **Spec coverage:** label+counter (T1 S2), position left of toggle (T1 S2 insertBefore), hidden at 0 / no native mechanism (T1 S2), native updateAll + #updateAll fallback (T1 S2), engine-independent wiring (T1 S3), idempotency + change-guarded writes (T1 S2, verified T2), i18n (T1 S1), CSS (T1 S4), verification (T2), release flow + feed sync (T3). ✓
- **Placeholders:** none — full code in every step. ✓
- **Consistency:** `injectUpdateAllButton` / `.sl-updall` / `dataset.slN` names identical across tasks; harness path matches the created file. ✓
