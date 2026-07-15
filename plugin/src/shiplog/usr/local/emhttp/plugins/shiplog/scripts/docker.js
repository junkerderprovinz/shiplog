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
    summary: "Summary", raw: "Changelog", source: "Source", close: "close", uptodate: "up to date",
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

  // Minimal, SAFE markdown for release bodies (btTeddy: raw markdown with
  // literal &nbsp; entities was barely readable). The input is first DECODED
  // (bodies often carry literal "&nbsp;" and <samp> tags), then FULLY escaped,
  // then a small pattern set is rebuilt as HTML — no upstream HTML ever
  // reaches innerHTML, and URLs exclude quote characters so they can't break
  // out of the href attribute.
  function renderMd(md) {
    const decoded = String(md)
      .replace(/&nbsp;/gi, " ")
      .replace(/<\/?samp>/gi, "`") // [<samp>(5cd63)</samp>](url) reads as code
      .replace(/\r\n/g, "\n");
    const inline = (s) => s
      .replace(/\*\*([^*\n]+)\*\*/g, "<b>$1</b>")
      .replace(/`([^`\n]+)`/g, "<code>$1</code>")
      .replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)"']+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
      .replace(/(^|[^"'>=])(https?:\/\/[^\s<"']+)/g, '$1<a href="$2" target="_blank" rel="noopener">$2</a>');
    let html = "";
    let inList = false;
    for (const line of esc(decoded).split("\n")) {
      const l = line.trim();
      const li = /^[-*]\s+(.*)$/.exec(l);
      if (li) {
        if (!inList) { html += "<ul>"; inList = true; }
        html += `<li>${inline(li[1])}</li>`;
        continue;
      }
      if (inList) { html += "</ul>"; inList = false; }
      const h = /^#{1,6}\s*(.*)$/.exec(l);
      if (h) {
        if (h[1]) html += `<div class="h">${inline(h[1])}</div>`;
        continue;
      }
      if (l === "") { html += '<div class="sp"></div>'; continue; }
      html += `<div>${inline(l)}</div>`;
    }
    if (inList) html += "</ul>";
    return html;
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
      raw = `<div class="sl-sec"><h4>${esc(T("raw"))}</h4><div class="sl-raw">${renderMd(cl.raw)}</div></div>`;
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

  // "Update now" keeps ShipLog read-only: it never touches the Docker socket — it runs
  // Unraid's OWN per-container update, the same one the Docker tab exposes. Two executors,
  // chosen by the Ask-before-updating pref, plus a proven fallback:
  //
  //   • runNativeUpdateNoConfirm (Ask OFF) — runs the real update with NO confirm dialog.
  //     It calls Unraid's global openDocker() with EXACTLY the command Unraid's own
  //     "Yes, update it!" callback runs (webgui dynamix.docker.manager/javascript/docker.js:110
  //     -> openDocker('update_container '+encodeURIComponent(name),'Updating the container','',
  //     'loadlist')). openDocker POSTs to /webGui/include/StartCommand.php via jQuery (so
  //     Unraid's csrf_token is auto-appended by its global $.ajaxPrefilter — HeadInlineJS.php:
  //     539-545), which launches the DETACHED updater (nohup ... & — StartCommand.php:46-49),
  //     tails it in Unraid's progress log, and refreshes the row via loadlist. There is NO
  //     confirm swal, so there is nothing to auto-click and nothing to race — this alone
  //     removes the whole freeze/auto-confirm class of bug. If openDocker is unavailable it
  //     falls back to clicking the row's native "apply update" anchor so the update STILL runs.
  //
  //   • runNativeUpdateWithConfirm (Ask ON) — runs Unraid's fully native flow, untouched:
  //     updateContainer(name) shows "Are you sure?" (docker.js:106) and calls openDocker on
  //     confirm. The USER clicks confirm — ShipLog never programmatically clicks it.
  //
  //   • clickNativeAnchor — the proven fallback: find the row's native update anchor + click.
  //
  // No auto-confirm exists anywhere in this design: Ask ON = the user confirms; Ask OFF =
  // there is no confirm to click. So an unrelated dialog (delete/remove/stop/OS-update) can
  // never be auto-accepted.

  // Click the row's native "apply update"/"rebuild ready" anchor. Returns false if absent.
  function clickNativeAnchor(name) {
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

  // Read a container name from a native update anchor: its updateContainer('<name>') onclick,
  // else its row's name. Lets the Ask-OFF path route native clicks to the no-confirm executor.
  function anchorName(a) {
    const oc = a.getAttribute("onclick") || "";
    const m = /updateContainer\(\s*['"]([^'"]+)['"]/i.exec(oc);
    if (m) return m[1];
    const tr = a.closest ? a.closest("tr") : null;
    return tr ? rowName(tr) : "";
  }

  // Run Unraid's real update with NO confirm. Prefer the native openDocker global (identical
  // to Unraid's own "Yes, update it!" callback); fall back to the proven native anchor click.
  function runNativeUpdateNoConfirm(name) {
    if (!name) return false;
    if (typeof window.openDocker === "function") {
      try {
        window.openDocker("update_container " + encodeURIComponent(name), T("updatingOne"), "", "loadlist");
        return true;
      } catch (e) { /* fall through to the proven native click */ }
    }
    return clickNativeAnchor(name);
  }

  // Run Unraid's native update WITH its own "Are you sure?" confirm (untouched). Prefer the
  // updateContainer global; fall back to clicking the native anchor (which invokes the same).
  function runNativeUpdateWithConfirm(name) {
    if (!name) return false;
    if (typeof window.updateContainer === "function") {
      try { window.updateContainer(name); return true; } catch (e) {}
    }
    return clickNativeAnchor(name);
  }

  // Silent update: hide ONLY Unraid's live progress log (the reused SweetAlert node while it
  // carries `.nchan`) and its shared backdrop, and free the page scroll-lock — then dismiss the
  // log for the user the instant the DETACHED job finishes. This can NEVER freeze the page and
  // can NEVER block the update:
  //   • It touches nothing until a `.sweet-alert.nchan` actually exists, so a confirm dialog
  //     (Ask ON) keeps its backdrop and stays fully interactive (fixes the old "confirm loses
  //     its overlay" bug where hiding was armed before the confirm even existed).
  //   • While the log is up it hides it + the overlay (body.sl-hide-nchan CSS) and clears
  //     `stop-scrolling`, so the page stays usable — no dark, click-eating, scroll-locked backdrop.
  //   • openDocker DISABLES the log's confirm button during the run; openDone RE-ENABLES it ONLY
  //     on completion (_DONE_ — HeadInlineJS.php:324-338). We latch on that disabled->enabled
  //     transition and click the now-"Done" button, which runs Unraid's OWN teardown (removes the
  //     overlay, clears the scroll-lock, stops nchan, runs loadlist). Locale-proof (no text match)
  //     and safe: only ever the finished `.nchan` log's Done button — never a confirm-to-proceed
  //     or a delete/remove dialog.
  //   • Writes NO inline style/dataset onto the shared swal node (only a body class it removes on
  //     stop), so the 2nd/3rd update inherits a clean node. A safety cap reveals the log if the
  //     job never reports done, so the user is never left staring at a hidden, stuck modal.
  // ─────────────────────────────────────────────── silent-update spinner
  // A SILENT update shows no log (armSilentLogHide hides Unraid's .nchan window), so the row
  // gives no feedback. We overlay a CSS spinner on the container's row logo/icon for the LIFE
  // of that hidden log: painted the instant the `.nchan` progress log actually appears (so a
  // cancelled Ask-before confirm — no log — never leaves a stuck spinner) and cleared when
  // armSilentLogHide's stop() fires (done) or its safety cap. Purely decorative
  // (pointer-events:none, never touches the Docker socket) and self-cleaning: the spinner lives
  // inside the row, so Unraid's wholesale #docker_list re-render drops it — we re-apply it on
  // every mutation for names still updating, and clear the set on stop().
  const updatingNames = new Set(); // lower(name) → its row shows a spinner
  let spinLive = false;            // true once the hidden log exists → painting active
  function armSpin(names) {         // queue names; painting waits for the log to appear
    const arr = Array.isArray(names) ? names : (names ? [names] : []);
    for (const nm of arr) { const n = norm(nm); if (n) updatingNames.add(n); }
    if (spinLive) refreshSpinners(); // a later arm during a live log paints at once
  }
  // The container logo sits in td.ct-name inside span.hand (the icon = Unraid's start/stop
  // context-menu trigger). Overlay the img's own box; fall back to the hand span, then the cell.
  function rowIconHost(tr) {
    const cell = tr.querySelector("td.ct-name");
    if (!cell) return null;
    const img = cell.querySelector("img");
    if (img && img.parentElement) return img.parentElement; // span.hand / .hoverable
    return cell.querySelector("span.hand") || cell;
  }
  function applySpinner(host) {
    if (!host || host.querySelector(":scope > .sl-spin")) return; // no host / already on
    try { if (getComputedStyle(host).position === "static") host.classList.add("sl-spinhost"); }
    catch (e) { host.classList.add("sl-spinhost"); }
    const ov = el("span", "sl-spin", '<span class="sl-spin-ring"></span>');
    ov.setAttribute("aria-hidden", "true");
    host.appendChild(ov);
  }
  function stripSpinner(host) {
    if (!host) return;
    const ov = host.querySelector(":scope > .sl-spin");
    if (ov) ov.remove();
    host.classList.remove("sl-spinhost");
  }
  function refreshSpinners() { // paint updating rows, strip stray spinners; no-op until log is live
    if (!spinLive || !updatingNames.size) return;
    for (const tr of findRows()) {
      const host = rowIconHost(tr);
      if (updatingNames.has(norm(rowName(tr)))) applySpinner(host);
      else stripSpinner(host);
    }
  }
  function spinLogAppeared() { if (!spinLive) { spinLive = true; refreshSpinners(); } }
  function clearSpinners() {
    spinLive = false;
    updatingNames.clear();
    document.querySelectorAll(".sl-spin").forEach((n) => n.remove());
    document.querySelectorAll(".sl-spinhost").forEach((n) => n.classList.remove("sl-spinhost"));
  }
  // Names Unraid's bulk update acts on (update===1) — the set a SILENT "Update all" must cover.
  function pendingUpdateNames() {
    try {
      const list = window.docker;
      if (!Array.isArray(list)) return [];
      return list.filter((d) => d && Number(d.update) === 1).map((d) => d && d.name).filter(Boolean);
    } catch (e) { return []; }
  }

  let silentArm = null;
  function armSilentLogHide(names) {
    armSpin(names);        // queue the updating row(s); the spinner paints when the log appears
    if (silentArm) return; // already watching this flow
    let obs = null, poll = 0, cap = 0, armedBody = false, sawDisabled = false;
    const stop = () => {
      try { obs && obs.disconnect(); } catch (e) {}
      clearInterval(poll); clearTimeout(cap);
      document.body.classList.remove("sl-hide-nchan");
      clearSpinners();     // update finished (or safety cap) → clear the row spinner(s)
      silentArm = null;
    };
    const tick = () => {
      // The progress log is the reused SweetAlert node while it carries `.nchan` AND holds the
      // log body `#swaltext`. Gating on #swaltext distinguishes it from a confirm dialog that
      // could momentarily reuse the same node — so we NEVER hide or auto-dismiss a confirm
      // (this closes the only path that could resurrect an unwanted auto-confirm).
      const log = document.querySelector(".sweet-alert.nchan");
      if (!log || !log.querySelector("#swaltext")) return; // confirm phase / nothing yet → touch nothing
      if (!armedBody) { document.body.classList.add("sl-hide-nchan"); armedBody = true; }
      spinLogAppeared(); // the hidden log now exists → start painting the row spinner(s)
      // idempotent: only touch the class when present, so this write can't re-feed an observer
      if (document.body.classList.contains("stop-scrolling")) document.body.classList.remove("stop-scrolling");
      const done = log.querySelector("button.confirm");
      if (!done) return;
      if (done.disabled) { sawDisabled = true; return; }  // running
      if (sawDisabled) { done.click(); stop(); }          // finished → Unraid's own teardown + loadlist
    };
    silentArm = { stop };
    try {
      obs = new MutationObserver(tick);
      // childList catches the `.nchan` log node appearing; the 400ms poll catches openDone
      // flipping the Done button's `disabled` attribute. Deliberately NOT watching attributes,
      // so writing our own body class inside tick() can never re-feed this observer.
      obs.observe(document.body, { childList: true, subtree: true });
    } catch (e) {}
    poll = setInterval(tick, 400);          // robust re-poll: openDone flips attrs, not just class
    cap = setTimeout(stop, 15 * 60 * 1000); // reveal the log if the job never reports done
    tick();
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
      // Arm the silent log-hider BEFORE the log appears; it is a no-op until `.nchan` exists,
      // so it never touches the Ask-ON confirm dialog. Pass the name → spinner on this row.
      if (silentUpdate) armSilentLogHide(cname);
      // Ask ON  → Unraid's own "Are you sure?" IS the confirmation (native, user clicks it).
      // Ask OFF → run Unraid's real update directly — no confirm dialog to race or auto-click.
      const ok = confirmUpdate ? runNativeUpdateWithConfirm(cname) : runNativeUpdateNoConfirm(cname);
      if (ok) close();
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
          if (silentUpdate) armSilentLogHide(pendingUpdateNames()); // hide the log + spin every updating row
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

  // Extend the "ask before updating" / "silent update" prefs to Unraid's OWN native
  // per-container update controls (not just ShipLog's bubble button). Capture phase so we
  // act BEFORE Unraid's onclick opens the dialog. Scoped STRICTLY to update controls, so
  // unrelated confirm dialogs (remove/stop/OS-update) are never touched — and note there is
  // no auto-confirm here at all: Ask OFF skips the confirm by running openDocker directly.
  document.addEventListener("click", (e) => {
    try {
      const a = e.target && e.target.closest ? e.target.closest("a[onclick], a.exec, [onclick]") : null;
      if (!a || a.closest(".sl-chip") || a.closest(".sl-chiprow") || a.closest(".sl-bubble")) return;
      const blob = norm(a.textContent) + " " + norm(a.getAttribute("onclick") || "");
      // strictly update controls — never delete/remove/stop/pause/etc.
      if (!/updatecontainer|apply update|aktualisierung anwenden|force update|update erzwingen|rebuild ready|installupdate/.test(blob)) return;
      if (silentUpdate) armSilentLogHide(anchorName(a)); // hide the log + spin this row (both Ask modes)
      if (!confirmUpdate) {
        // Ask OFF → skip Unraid's confirm entirely: run the real update directly. Only prevent
        // the native handler when we can actually take over via openDocker — otherwise re-clicking
        // this same anchor would loop. Without a name/openDocker, let native run (with its confirm).
        const name = anchorName(a);
        if (name && typeof window.openDocker === "function") {
          e.preventDefault(); e.stopPropagation();
          close(); // our stopPropagation suppresses the bubble-close listener — close it ourselves
          try { window.openDocker("update_container " + encodeURIComponent(name), T("updatingOne"), "", "loadlist"); }
          catch (err) {
            if (typeof window.updateContainer === "function") window.updateContainer(name);
            else clickNativeAnchor(name); // last resort: the update still runs
          }
        }
      }
      // Ask ON → do nothing else; Unraid's native confirm+update runs untouched.
    } catch (err) {}
  }, true);

  async function boot() {
    // Native-data feature first: the update-all button works without the engine.
    injectUpdateAllButton();
    // Unraid re-renders the table on its auto-refresh — re-tag new rows and keep
    // the update-all counter current. tagRows() is a no-op while byName is empty.
    const mo = new MutationObserver(() => { tagRows(); injectUpdateAllButton(); refreshSpinners(); });
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
