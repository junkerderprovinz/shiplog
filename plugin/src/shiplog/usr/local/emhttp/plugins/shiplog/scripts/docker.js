/* ShipLog — Docker-tab injection.
 *
 * Loaded by shiplog.Docker.page into Unraid's native Docker tab. For each
 * container row it fetches the engine's verdict via the same-origin PHP proxy
 * and, when an update is available, adds a discreet control in the update
 * column: a log glyph · "Changelog" · a risk traffic-light dot. Clicking opens
 * the changelog bubble. If the engine is unreachable, it stays silent (the
 * Docker tab is untouched) — set DEMO=true below to preview without the engine.
 */
(function () {
  "use strict";

  const PROXY = "/plugins/shiplog/server/status.php";
  const MARK = "data-shiplog";
  const TAG = "[ShipLog]";
  const DEMO = false; // true → show demo data even without the engine

  // Update behaviour, set by shiplog.Docker.page from the plugin cfg:
  //   confirmUpdate — ask before triggering the update (default on)
  //   silentUpdate  — suppress Unraid's pop-up download-log window (default off)
  const PREFS = (window.shiplogPrefs && typeof window.shiplogPrefs === "object") ? window.shiplogPrefs : {};
  const confirmUpdate = PREFS.confirmUpdate !== false; // default on
  const silentUpdate = PREFS.silentUpdate === true;    // default off

  const UPDATE_PHRASES = [
    "aktualisierung anwenden", "auf dem neu", "nicht verfügbar", "wird geprüft",
    "up-to-date", "up to date", "update ready", "apply update", "not available",
    "rebuild ready",
  ];

  const LOG_ICON =
    '<svg class="sl-ico" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" ' +
    'stroke-linecap="round"><rect x="3" y="2" width="10" height="12" rx="1.5"/>' +
    '<line x1="5.4" y1="5.5" x2="10.6" y2="5.5"/><line x1="5.4" y1="8" x2="10.6" y2="8"/>' +
    '<line x1="5.4" y1="10.5" x2="9" y2="10.5"/></svg>';

  // risk → css suffix used by both the chip dot and the bubble pill
  const RISK_CLASS = { low: "low", medium: "mid", high: "high", unknown: "grey" };

  // ──────────────────────────────────────────────────────── i18n
  // The bubble text follows Unraid's CONFIGURED language: shiplog.Docker.page selects
  // the active locale server-side and injects its strings as window.shiplogI18n (d_*
  // keys) from the plugin's locale files, for all ~26 locales. The engine-generated
  // risk reason stays English for now. EN below is the offline fallback (unprefixed).
  const EN = {
    changelog: "Changelog", clickHint: "click for the changelog", update: "Update",
    skips: "skips %n releases", newest: "newest %d",
    summary: "Summary", raw: "Changelog (raw)", source: "Source", close: "close", uptodate: "up to date",
    deprecated: "DEPRECATED",
    updateNow: "Update now", updateHint: "triggers Unraid's own update for this container",
    updateGone: "Couldn't find Unraid's update button — use 'apply update' on the row.",
    confirmOne: "Update %s now?", confirmAll: "Update all %n containers with a pending update now?",
    updatingOne: "Updating the container", updatingAll: "Updating all Containers",
    updateAll: "Update all", updateAllHint: "triggers Unraid's own update for every container with a pending update",
    none: "No changelog text found for this image — open the repo (top-right) for the release notes.",
    pinned: "pinned", localimg: "local image",
  };
  const I18N = (window.shiplogI18n && typeof window.shiplogI18n === "object") ? window.shiplogI18n : {};
  function T(k) { return I18N["d_" + k] || EN[k] || k; }

  // Light vs dark: detect from the page background luminance so the bubble
  // matches whatever Unraid theme is active (white/azure → light; black/gray → dark).
  function isLightBg() {
    try {
      const m = /(\d+),\s*(\d+),\s*(\d+)/.exec(getComputedStyle(document.body).backgroundColor);
      if (!m) return false;
      return (0.299 * +m[1] + 0.587 * +m[2] + 0.114 * +m[3]) / 255 > 0.5;
    } catch (e) { return false; }
  }

  // ──────────────────────────────────────────────────────── helpers
  function el(tag, cls, html) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (html != null) n.innerHTML = html;
    return n;
  }
  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
  }
  function norm(s) { return String(s || "").trim().toLowerCase(); }

  function hasUpdate(st) {
    const k = st && st.kind;
    return k && k !== "none";
  }
  function riskClass(st) { return RISK_CLASS[st && st.risk] || "grey"; }

  // Unraid's OWN update verdict for a container, read from its page-global docker[]
  // array (d.update: 1 = update available, 0 = up to date). This is the SAME digest-
  // based signal the Docker tab shows, and it's live — so ShipLog's verdict follows
  // it and can never contradict what Unraid displays. ShipLog's engine (version +
  // changelog + risk) then only enriches the detail. Returns "update"|"current"|"unknown".
  function unraidVerdict(name) {
    try {
      const list = window.docker;
      if (!Array.isArray(list) || !name) return "unknown";
      const nn = norm(name);
      const d = list.find((x) => x && norm(x.name) === nn);
      if (!d || d.update == null || d.update === "") return "unknown";
      const u = Number(d.update);
      if (u === 1) return "update";
      if (u === 0) return "current";
      return "unknown"; // -1 / not-yet-checked → let ShipLog's own verdict stand
    } catch (e) { return "unknown"; }
  }

  // Reconcile ShipLog's engine verdict with Unraid's live one: Unraid wins the
  // update/up-to-date decision when it has an opinion, else ShipLog's own stands.
  function isUpdate(st) {
    const uv = unraidVerdict(st && st.container && st.container.name);
    if (uv === "update") return true;
    if (uv === "current") return false;
    return hasUpdate(st);
  }

  // Containers without an upstream to check (digest-pinned, image-ID-referenced,
  // locally built) must not look like an affirmed "up to date": neutral grey
  // dot + an honest label instead of the green pill.
  function noUpstream(st) {
    const c = (st && st.container) || {};
    return !!(c.pinned_digest || c.is_local || !c.repo);
  }
  function noUpstreamLabel(st) {
    const c = (st && st.container) || {};
    return c.is_local ? T("localimg") : T("pinned");
  }

  // Build a short human label from kind + skipped count, e.g. "MAJOR", "2× MINOR".
  function kindLabel(st) {
    const k = (st.kind || "unknown").toUpperCase();
    const cl = st.changelog;
    const skipped = cl && cl.skipped_count > 1 ? cl.skipped_count + "× " : "";
    return skipped + k;
  }

  // ──────────────────────────────────────────────────────── data
  let byName = {}; // lower(name) → UpdateStatus

  function index(list) {
    byName = {};
    (list || []).forEach((st) => {
      const n = norm(st.container && st.container.name);
      if (n) byName[n] = st;
    });
  }

  async function load() {
    if (DEMO) { index(DEMO_DATA); return true; }
    try {
      const res = await fetch(PROXY, { headers: { Accept: "application/json" } });
      if (!res.ok) { console.warn(`${TAG} proxy ${res.status}`); return false; }
      const data = await res.json();
      if (data && data.error) { console.warn(`${TAG} engine: ${data.error}`); return false; }
      index(Array.isArray(data) ? data : []);
      return true;
    } catch (e) {
      console.warn(`${TAG} proxy unreachable`, e);
      return false;
    }
  }

  // ──────────────────────────────────────────────────────── rows
  // A folder-header row is NOT a container — skip it. The "Folder View" plugin
  // (v3 and the older docker.folder) groups containers into collapsible folder
  // rows; the header carries .folder / .folder-id-* and its update cell is the
  // folder's own .folder-update aggregate, never a real container verdict.
  function isFolderHeader(tr) {
    return !!(tr.classList.contains("folder") ||
      tr.querySelector(":scope > td.folder-name, :scope > td.folder-update"));
  }

  function findRows() {
    // Canonical Unraid Docker tab: tbody#docker_list inside table#docker_containers.
    // A plain container row is tr.sortable; under Folder View v3 an *expanded*
    // folder's members are re-emitted as tr.folder-element siblings (collapsed
    // members live inside the folder's .folder-storage and have no rendered
    // update cell, so they're correctly invisible to us until expanded — the
    // MutationObserver re-tags them when they appear). Both carry td.ct-name +
    // td.updatecolumn, so one selector covers native AND Folder View.
    const candidates = [
      "#docker_list tr.sortable, #docker_list tr.folder-element",
      "#docker_list > tr",
      "table#docker_containers tbody tr",
      "table.tablesorter tbody tr",
      "div.tabs table tbody tr",
      "table tbody tr",
    ];
    for (const sel of candidates) {
      const rows = Array.from(document.querySelectorAll(sel)).filter((tr) =>
        !isFolderHeader(tr) &&
        // real container row: either Unraid's name/update cells, or (unknown
        // skin fallback) an icon image with some text.
        (tr.querySelector("td.ct-name, td.updatecolumn") ||
          (tr.querySelector("img") && tr.textContent.trim().length > 1)));
      if (rows.length) return rows;
    }
    return [];
  }
  function rowName(tr) {
    // Prefer Unraid's own name cell (stable across native + Folder View).
    const appname = tr.querySelector("td.ct-name .appname");
    if (appname && appname.textContent.trim()) return appname.textContent.trim().slice(0, 60);
    // Unraid (and Folder View) tag each container row id="ct-<name>".
    const id = tr.id || "";
    if (/^ct-/.test(id)) return id.slice(3).slice(0, 60);
    const img = tr.querySelector("img");
    const cell = img ? img.closest("td") || tr : tr;
    const a = cell.querySelector("a");
    const name = a && a.textContent.trim()
      ? a.textContent.trim()
      : (cell.textContent || tr.textContent).trim().split("\n")[0].trim();
    return name.slice(0, 60);
  }
  function findUpdateCell(tr) {
    // Unraid's update-status cell carries .updatecolumn (native + Folder View
    // container rows alike). Never a folder's .folder-update — folder headers
    // are already filtered out of findRows().
    const direct = tr.querySelector("td.updatecolumn:not(.folder-update)");
    if (direct) return direct;
    const cells = Array.from(tr.querySelectorAll("td"));
    for (const td of cells) {
      if (UPDATE_PHRASES.some((p) => td.textContent.toLowerCase().includes(p))) return td;
    }
    return cells[cells.length - 1] || tr;
  }

  // ──────────────────────────────────────────────────────── bubble
  let open = null;
  const SZ_KEY = "shiplog.bubbleSize";
  function close() {
    if (open) {
      if (open._ro) { try { open._ro.disconnect(); } catch (e) {} }
      open.remove();
      open = null;
    }
  }

  function bubbleHTML(st) {
    const c = st.container || {};
    const cl = st.changelog || {};
    const upd = isUpdate(st);       // Unraid's live verdict wins the update/current call
    const seUpd = hasUpdate(st);    // ShipLog engine's own opinion (drives the risk detail)
    // up to date → green pill; no upstream to check → neutral grey, not green. When
    // Unraid flags an update ShipLog didn't grade (a rebuild / stale engine data), use low.
    const rc = upd ? (seUpd ? riskClass(st) : "low") : (noUpstream(st) ? "grey" : "ok");
    // changelog from/to are the image TAGS ("latest"/"7dtd"), not versions —
    // show a real version when we have one. Newest release tag comes from the
    // resolved release entries; current is the running tag if it looks like a
    // version, else the short digest (a :latest digest can't be mapped to a tag).
    const verLike = (t) => /^v?\d+\.\d+/.test(t || "");
    const entries = Array.isArray(cl.entries) ? cl.entries : [];
    const newestRel = entries[0] && entries[0].tag ? entries[0].tag : "";
    // Current version to show. The engine REMEMBERS the running version per
    // container (running_version) — for a pinned tag it's the tag, for a
    // ":latest" it's resolved when the running image is the registry's current
    // one and then carried forward, so a later update shows a real "1.7 -> 1.8".
    // Fall back to the running tag, then (when up to date) the newest release,
    // then the tag — never the cryptic digest hash.
    const shortDig = (d) => (d ? d.replace(/^sha256:/, "").slice(0, 12) : "");
    const cur = (verLike(st.running_version) ? st.running_version : "")
      || (verLike(c.tag) ? c.tag : "")
      || (!upd && verLike(newestRel) ? newestRel : "")
      // A digest-pinned ref without a tag has NO tag — show the pin, not "latest".
      || (c.tag || shortDig(c.pinned_digest) || "latest");
    const next = (verLike(st.newest_tag) ? st.newest_tag : "")
      || (verLike(newestRel) ? newestRel : "")
      || newestRel || st.newest_tag || "?";
    const fmtDate = (iso) => {
      const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso || "");
      return m ? `${m[3]}.${m[2]}.${m[1]}` : ""; // DD.MM.YYYY
    };
    const relDate = entries[0] ? fmtDate(entries[0].published_at) : "";
    // Only surface ShipLog's version-jump reason when we're actually flagging an
    // update (so an "up to date" per Unraid never carries a stray "minor bump" line).
    let jump = upd ? (cl.skipped_count > 1 ? T("skips").replace("%n", cl.skipped_count) : (st.risk_reason || "")) : "";
    if (relDate) jump = (jump ? jump + " · " : "") + T("newest").replace("%d", relDate);

    let summary = "";
    if (cl.summary && (cl.summary.bullets || cl.summary.breaking)) {
      const lis = []
        .concat((cl.summary.breaking || []).map((t) => `<li class="sl-warn">${esc(t)}</li>`))
        .concat((cl.summary.bullets || []).map((t) => `<li>${esc(t)}</li>`))
        .join("");
      summary = `<div class="sl-sec"><h4>⚡ ${esc(T("summary"))} <span class="sl-ai">AI · ${esc(cl.summary.model || "Ollama")}</span></h4><ul>${lis}</ul></div>`;
    }

    let raw = "";
    if (cl.raw) {
      const body = esc(cl.raw)
        .replace(/^(#{1,3}.*)$/gm, '<span class="h">$1</span>')
        .replace(/(https?:\/\/[^\s<]+)/g, '<a href="$1" target="_blank" rel="noopener">$1</a>');
      raw = `<div class="sl-sec"><h4>${esc(T("raw"))}</h4><div class="sl-raw">${body}</div></div>`;
    } else if (!summary) {
      raw = `<div class="sl-sec"><div style="color:var(--sl-dim2)">${esc(T("none"))}</div></div>`;
    }

    // Link to the repo's MAIN page. We don't use the engine's changelog URL
    // verbatim (it's a .../compare/from...to link, broken for :latest), but its
    // repo ROOT is good — so take the root from the OCI source, else from the
    // changelog URL, else derive it from a ghcr image path.
    const ghFromImage = (r) => {
      const m = /^ghcr\.io\/([^/]+)\/([^/:@]+)/.exec(r || "");
      return m ? "https://github.com/" + m[1] + "/" + m[2] : "";
    };
    const repoRoot = (u) => {
      const m = /^(https?:\/\/[^/]+\/[^/]+\/[^/]+)/.exec(u || "");
      return m ? m[1].replace(/\.git$/, "") : "";
    };
    const href = repoRoot(c.source) || repoRoot(cl.url) || ghFromImage(c.repo || c.image);
    const gh = href
      ? `<a class="sl-gh" href="${esc(href)}" target="_blank" rel="noopener">Repository ↗</a>`
      : "";
    const src = cl.source ? `${esc(T("source"))}: ${esc(cl.source)}` : "";

    // Only show a "cur → next" jump when there's a real, different target version;
    // an Unraid-flagged rebuild (same version, new digest) shows just the version.
    const haveNext = verLike(next) && next !== "?" && norm(next) !== norm(cur);
    const verHdr = (upd && haveNext) ? `${esc(cur)} → <b>${esc(next)}</b>` : `<b>${esc(cur)}</b>`;
    const pillTxt = upd
      ? esc(seUpd ? kindLabel(st) : T("update"))
      : esc(noUpstream(st) ? noUpstreamLabel(st) : T("uptodate"));

    return `
      <div class="sl-bh">
        <span class="sl-ver">${verHdr}</span>
        <span class="sl-pill sl-${rc}"><span class="sl-dot"></span>${pillTxt}</span>
        ${cl.deprecated ? `<span class="sl-pill sl-high" title="upstream repository is archived">⚠ ${esc(T("deprecated"))}</span>` : ""}
        <span class="sl-jump">${esc(jump)}</span>
        <span class="sl-bh-right">${upd ? `<a class="sl-upd" href="#" title="${esc(T("updateHint"))}">${esc(T("updateNow"))}</a>` : ""}${gh}<span class="sl-x" title="${esc(T("close"))}">✕</span></span>
      </div>
      ${summary}${raw}
      ${src ? `<div class="sl-bf"><span>${src}</span></div>` : ""}`;
  }

  // "Update now" keeps ShipLog read-only: it never touches the Docker socket — it
  // triggers Unraid's OWN per-container update (the "apply update" control the
  // Docker tab already shows) by finding it in the row's update cell and clicking
  // it; Unraid runs the update. Returns false if it can't be found.
  function triggerNativeUpdate(name) {
    if (!name) return false;
    for (const tr of findRows()) {
      if (norm(rowName(tr)) !== norm(name)) continue;
      const cell = findUpdateCell(tr);
      if (!cell) return false;
      const cands = Array.from(cell.querySelectorAll("a[onclick],a[href],span[onclick],[onclick]"))
        .filter((n) => !n.closest(".sl-chip") && !n.closest(".sl-chiprow"));
      const blob = (n) => norm(n.textContent) + " " + norm(n.getAttribute("onclick") || "");
      const apply = cands.find((n) => /apply update|aktualisierung anwenden|rebuild ready|applyupdate|updatecontainer|installupdate|installxml/.test(blob(n)));
      const target = apply || cands.find((n) => /update|aktualisier/.test(blob(n)));
      if (target) { target.click(); return true; }
      return false;
    }
    return false;
  }

  // BEST-EFFORT automation of Unraid's OWN update dialogs after ShipLog clicks the native
  // control: auto-confirm the "are you sure?" swal when Ask-before-updating is OFF, and
  // hide the progress download-log window (.sweet-alert.nchan) when Silent is ON. If
  // Unraid's dialog markup ever differs, every branch here is a harmless no-op and the
  // update still proceeds visibly — the core update can NEVER break because of this.
  function armUpdateSwals() {
    const autoConfirm = !confirmUpdate;
    const hideLog = silentUpdate;
    if (!autoConfirm && !hideLog) return;
    const off = (n) => { if (!n) return; n.style.setProperty("opacity", "0", "important"); n.style.setProperty("visibility", "hidden", "important"); n.style.setProperty("pointer-events", "none", "important"); };
    const tick = () => {
      if (autoConfirm) {
        // Unraid's confirm dialog = a visible .sweet-alert that is NOT the nchan progress
        // log; click its confirm button once ("Yes, update it!").
        const box = document.querySelector(".sweet-alert.visible:not(.nchan), .sweet-alert.showSweetAlert:not(.nchan)");
        if (box && !box.dataset.slConfirmed) {
          const btn = box.querySelector("button.confirm, .sa-button-container button.confirm, button.sa-confirm");
          if (btn) { box.dataset.slConfirmed = "1"; btn.click(); }
        }
      }
      if (hideLog) {
        const prog = document.querySelector(".sweet-alert.nchan");
        if (prog && !prog.dataset.slHidden) {
          prog.dataset.slHidden = "1";
          off(prog); off(document.querySelector(".sweet-overlay"));
          document.body.classList.remove("stop-scrolling");
          document.body.style.setProperty("overflow", "auto", "important");
        }
      }
    };
    tick();
    let obs;
    try {
      obs = new MutationObserver(tick);
      obs.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ["class"] });
    } catch (e) {}
    setTimeout(() => { try { obs && obs.disconnect(); } catch (e) {} }, 12000);
  }

  // runUpdate performs Unraid's OWN per-container update the PROVEN way — it clicks the
  // native "apply update" control (triggerNativeUpdate), exactly what worked before the
  // update options existed — and layers the two prefs on top as best-effort via
  // armUpdateSwals (auto-confirm when Ask is off, hide the log when Silent is on). If
  // that automation can't find Unraid's dialogs, the update STILL runs, just visibly.
  // Returns false only if the native control itself can't be found (caller shows a hint).
  function runUpdate(name) {
    armUpdateSwals();
    return triggerNativeUpdate(name);
  }

  function openFor(anchor, st) {
    close();
    const b = el("div", "sl-bubble", bubbleHTML(st));
    if (isLightBg()) b.classList.add("sl-light"); // match Unraid's light themes
    document.body.appendChild(b);
    // restore the user's saved size (the bubble is resizable, drag the corner)
    try {
      const s = JSON.parse(localStorage.getItem(SZ_KEY) || "null");
      if (s && s.w) b.style.width = s.w + "px";
      if (s && s.h) b.style.height = s.h + "px";
    } catch (e) {}
    const r = anchor.getBoundingClientRect();
    const w = b.offsetWidth || 640;
    const maxLeft = window.scrollX + document.documentElement.clientWidth - w - 12;
    let left = Math.min(window.scrollX + r.left, maxLeft);
    left = Math.max(window.scrollX + 8, left);
    b.style.left = left + "px";
    b.style.top = window.scrollY + r.bottom + 8 + "px";
    b.querySelector(".sl-x").addEventListener("click", (e) => { e.stopPropagation(); close(); });
    const updBtn = b.querySelector(".sl-upd");
    if (updBtn) updBtn.addEventListener("click", (e) => {
      e.preventDefault(); e.stopPropagation();
      const cname = st.container && st.container.name;
      // runUpdate clicks Unraid's native control; its own "are you sure?" dialog serves
      // as the confirmation (auto-confirmed when Ask-before-updating is off).
      if (runUpdate(cname)) close();
      else { updBtn.textContent = T("updateGone"); updBtn.classList.add("sl-upd-off"); }
    });
    // persist size whenever the user drags the resize handle
    try {
      const ro = new ResizeObserver(() => {
        try { localStorage.setItem(SZ_KEY, JSON.stringify({ w: b.offsetWidth, h: b.offsetHeight })); } catch (e) {}
      });
      ro.observe(b);
      b._ro = ro;
    } catch (e) {}
    open = b;
  }

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
          const n = pendingUpdateCount();
          if (n === 0) return;
          // native updateAll() has no confirm dialog of its own, so ShipLog owns the ask
          if (confirmUpdate && !window.confirm(T("confirmAll").replace("%n", n))) return;
          armUpdateSwals();           // best-effort: hide the log window when Silent is on
          fireNativeUpdateAll();
        });
        // .ToggleViewMode is a full-width flex row with justify-content:flex-end —
        // as its FIRST child the button packs to the right, directly LEFT of the
        // toggle (inserting before the div would flow to the far left instead).
        toggle.insertBefore(btn, toggle.firstChild);
      }

      const n = nativeUpdateAllAvailable() ? pendingUpdateCount() : 0;
      // change-guarded writes: identical values must not touch the DOM
      if (btn.dataset.slN !== String(n)) {
        btn.dataset.slN = String(n);
        btn.textContent = `${T("updateAll")} (${n})`;
      }
      const want = n >= 1 ? "" : "none";
      if (btn.style.display !== want) btn.style.display = want;
    } catch (e) {}
  }

  // ──────────────────────────────────────────────────────── inject
  function tagRows() {
    let n = 0;
    for (const tr of findRows()) {
      const st = byName[norm(rowName(tr))];
      if (!st) continue; // no engine data for this row → leave it untouched
      const cell = findUpdateCell(tr);
      if (!cell || cell.getAttribute(MARK)) continue;
      const upd = isUpdate(st), seUpd = hasUpdate(st); // Unraid's live verdict wins
      const rc = upd ? (seUpd ? riskClass(st) : "low") : (noUpstream(st) ? "grey" : "ok");
      const label = upd ? (seUpd ? kindLabel(st) : T("update")) : T("uptodate");
      const chip = el("a", "sl-chip", `${LOG_ICON}<span>${esc(T("changelog"))}</span><span class="sl-amp sl-${rc}"></span>`);
      chip.href = "#";
      chip.title = `ShipLog: ${label} — ${T("clickHint")}`;
      chip.addEventListener("click", (e) => { e.preventDefault(); e.stopPropagation(); openFor(chip, st); });
      const row = el("div", "sl-chiprow");
      row.appendChild(chip);
      cell.appendChild(row);
      cell.setAttribute(MARK, "1");
      n++;
    }
    return n;
  }

  // ──────────────────────────────────────────────────────── run
  document.addEventListener("click", (e) => {
    if (open && !open.contains(e.target) && !e.target.closest(".sl-chip")) close();
  });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") close(); });

  async function boot() {
    // Native-data feature first: the update-all button works without the engine.
    injectUpdateAllButton();
    // Unraid re-renders the table on its auto-refresh — re-tag new rows and keep
    // the update-all counter current. tagRows() is a no-op while byName is empty.
    const mo = new MutationObserver(() => { tagRows(); injectUpdateAllButton(); });
    try { mo.observe(document.body, { childList: true, subtree: true }); } catch (e) {}

    const ok = await load();
    if (!ok) return; // engine unreachable → chips stay off, the button still works
    const n = tagRows();
    console.log(`${TAG} tagged ${n} container(s) with updates`);
  }
  boot();

  // ──────────────────────────────────────────────────────── demo
  const DEMO_DATA = [
    { container: { name: "Immich", image: "ghcr.io/imagegenius/immich", tag: "v1.122.0" },
      newest_tag: "v1.124.2", kind: "minor", risk: "medium", risk_reason: "2 minor versions",
      changelog: { from_tag: "v1.122.0", to_tag: "v1.124.2", skipped_count: 2,
        raw: "## v1.124.2\n- fix: transcode leak\n## v1.124.0\n- breaking: requires PostgreSQL >= 15",
        source: "GitHub releases via OCI label", url: "https://github.com/immich-app/immich/releases" } },
  ];
})();
