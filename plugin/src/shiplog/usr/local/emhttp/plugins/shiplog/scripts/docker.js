/* Adds a changelog control with a risk dot to the update column of each
 * container in Unraid's Docker tab, fed by the engine through the same-origin
 * proxy. Without the engine the tab stays untouched; DEMO previews the control.
 */
(function () {
  "use strict";

  const PROXY = "/plugins/shiplog/server/status.php";
  const MARK = "data-shiplog";
  const TAG = "[ShipLog]";
  const DEMO = false;

  // Set by shiplog.Docker.page from the plugin cfg. silentUpdate hides Unraid's
  // download log window.
  const PREFS = (window.shiplogPrefs && typeof window.shiplogPrefs === "object") ? window.shiplogPrefs : {};
  const confirmUpdate = PREFS.confirmUpdate !== false;
  const silentUpdate = PREFS.silentUpdate === true;

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

  const WARN_ICON =
    '<svg class="sl-ico" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">' +
    '<path d="M12 2 1 21h22L12 2zm0 6a1 1 0 0 1 1 1v5a1 1 0 1 1-2 0V9a1 1 0 0 1 1-1zm0 9.5a1.25 1.25 0 1 1 0 2.5 1.25 1.25 0 0 1 0-2.5z"/></svg>';

  const RISK_CLASS = { low: "low", medium: "mid", high: "high", critical: "crit", unknown: "grey" };

  // shiplog.Docker.page injects the d_* strings of Unraid's language as
  // window.shiplogI18n; EN is the fallback. The engine's risk reason is English.
  const EN = {
    changelog: "Changelog", clickHint: "click for the changelog", update: "Update",
    skips: "skips %n releases", newest: "newest %d",
    summary: "Summary", raw: "Changelog", source: "Source", close: "close", uptodate: "up to date",
    deprecated: "DEPRECATED",
    unmaintained: "Not maintained", unmaintainedHint: "No more updates. Consider migrating to a maintained alternative.",
    updateNow: "Update now", updateHint: "triggers Unraid's own update for this container",
    updateGone: "Couldn't find Unraid's update button. Use 'apply update' on the row.",
    confirmOne: "Update %s now?", confirmAll: "Update all %n containers with a pending update now?",
    updatingOne: "Updating the container", updatingAll: "Updating all Containers",
    updateAll: "Update all", updateAllHint: "triggers Unraid's own update for every container with a pending update",
    none: "No changelog text found for this image. Open the repo (top-right) for the release notes.",
    rateLimited: "GitHub's hourly rate limit was reached, so the release notes couldn't load. Add a GitHub token in ShipLog's settings (Sources) to raise it.",
    recent: "Recent releases",
    pinned: "pinned", localimg: "local image",
  };
  const I18N = (window.shiplogI18n && typeof window.shiplogI18n === "object") ? window.shiplogI18n : {};
  function T(k) { return I18N["d_" + k] || EN[k] || k; }

  // The page background's luminance tells a light Unraid theme from a dark one.
  function isLightBg() {
    try {
      const m = /(\d+),\s*(\d+),\s*(\d+)/.exec(getComputedStyle(document.body).backgroundColor);
      if (!m) return false;
      return (0.299 * +m[1] + 0.587 * +m[2] + 0.114 * +m[3]) / 255 > 0.5;
    } catch (e) { return false; }
  }

  function el(tag, cls, html) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (html != null) n.innerHTML = html;
    return n;
  }
  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
  }

  // renderMd escapes the whole release body before it rebuilds a few markdown
  // patterns as HTML, so no upstream HTML reaches innerHTML. URLs exclude quotes,
  // so they cannot break out of the href attribute.
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

  // unraidVerdict reads Unraid's own live verdict from its global docker[] array,
  // so ShipLog never contradicts what the Docker tab shows. Returns "update",
  // "current" or "unknown".
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
      return "unknown";
    } catch (e) { return "unknown"; }
  }

  // Unraid's verdict wins when it has one.
  function isUpdate(st) {
    const uv = unraidVerdict(st && st.container && st.container.name);
    if (uv === "update") return true;
    if (uv === "current") return false;
    return hasUpdate(st);
  }

  // A container without an upstream to check gets a grey dot and its own label
  // instead of a green "up to date".
  function noUpstream(st) {
    const c = (st && st.container) || {};
    return !!(c.pinned_digest || c.is_local || !c.repo);
  }
  function noUpstreamLabel(st) {
    const c = (st && st.container) || {};
    return c.is_local ? T("localimg") : T("pinned");
  }

  // kindLabel returns labels such as "MAJOR" or "2× MINOR".
  function kindLabel(st) {
    const k = (st.kind || "unknown").toUpperCase();
    const cl = st.changelog;
    const skipped = cl && cl.skipped_count > 1 ? cl.skipped_count + "× " : "";
    return skipped + k;
  }

  let byName = {}; // lower-cased name -> UpdateStatus

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

  // A Folder View header row is not a container; its update cell aggregates the
  // folder.
  function isFolderHeader(tr) {
    return !!(tr.classList.contains("folder") ||
      tr.querySelector(":scope > td.folder-name, :scope > td.folder-update"));
  }

  function findRows() {
    // Folder View re-emits the members of an expanded folder as tr.folder-element;
    // collapsed members have no update cell and get tagged once they appear.
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
        // An unknown skin still shows an icon image and some text.
        (tr.querySelector("td.ct-name, td.updatecolumn") ||
          (tr.querySelector("img") && tr.textContent.trim().length > 1)));
      if (rows.length) return rows;
    }
    return [];
  }
  function rowName(tr) {
    const appname = tr.querySelector("td.ct-name .appname");
    if (appname && appname.textContent.trim()) return appname.textContent.trim().slice(0, 60);
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
    const direct = tr.querySelector("td.updatecolumn:not(.folder-update)");
    if (direct) return direct;
    const cells = Array.from(tr.querySelectorAll("td"));
    for (const td of cells) {
      if (UPDATE_PHRASES.some((p) => td.textContent.toLowerCase().includes(p))) return td;
    }
    return cells[cells.length - 1] || tr;
  }

  let open = null;
  const SZ_KEY = "shiplog.bubbleSize";
  function close() {
    if (open) {
      if (open._ro) { try { open._ro.disconnect(); } catch (e) {} }
      if (open._backdrop) { try { open._backdrop.remove(); } catch (e) {} }
      open.remove();
      open = null;
    }
  }

  function bubbleHTML(st) {
    const c = st.container || {};
    const cl = st.changelog || {};
    const upd = isUpdate(st);
    const seUpd = hasUpdate(st); // the engine's view, which carries the risk detail
    // An update Unraid flags but the engine did not grade, such as a rebuild,
    // counts as low.
    const rc = upd ? (seUpd ? riskClass(st) : "low") : (noUpstream(st) ? "grey" : "ok");
    const verLike = (t) => /^v?\d+\.\d+/.test(t || "");
    const entries = Array.isArray(cl.entries) ? cl.entries : [];
    const newestRel = entries[0] && entries[0].tag ? entries[0].tag : "";
    // A tag like "latest" is not a version, so the running version the engine
    // remembers comes first.
    const shortDig = (d) => (d ? d.replace(/^sha256:/, "").slice(0, 12) : "");
    const cur = (verLike(st.running_version) ? st.running_version : "")
      || (verLike(c.tag) ? c.tag : "")
      || (!upd && verLike(newestRel) ? newestRel : "")
      // A digest pin without a tag shows the pin, not "latest".
      || (c.tag || shortDig(c.pinned_digest) || "latest");
    const next = (verLike(st.newest_tag) ? st.newest_tag : "")
      || (verLike(newestRel) ? newestRel : "")
      || newestRel || st.newest_tag || "?";
    const fmtDate = (iso) => {
      const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso || "");
      return m ? `${m[3]}.${m[2]}.${m[1]}` : ""; // DD.MM.YYYY
    };
    const relDate = entries[0] ? fmtDate(entries[0].published_at) : "";
    // An up-to-date container carries no version jump line.
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
    if (cl.rate_limited) {
      raw = `<div class="sl-sec"><div style="color:var(--sl-dim2)">${esc(T("rateLimited"))}</div></div>`;
    } else if (cl.recent && entries.length) {
      const secs = entries.map((e) => {
        const d = fmtDate(e.published_at);
        const meta = d ? ` <span class="sl-reld">${esc(d)}</span>` : "";
        const link = e.url ? ` <a class="sl-relx" href="${esc(e.url)}" target="_blank" rel="noopener">↗</a>` : "";
        return `<div class="sl-rel"><h5>${esc(e.tag || "")}${meta}${link}</h5><div class="sl-raw">${renderMd(e.body || "")}</div></div>`;
      }).join("");
      raw = `<div class="sl-sec"><h4>${esc(T("recent"))}</h4>${secs}</div>`;
    } else if (cl.raw) {
      raw = `<div class="sl-sec"><h4>${esc(T("raw"))}</h4><div class="sl-raw">${renderMd(cl.raw)}</div></div>`;
    } else if (!summary) {
      raw = `<div class="sl-sec"><div style="color:var(--sl-dim2)">${esc(T("none"))}</div></div>`;
    }

    // The button links to the repo root, because the engine's compare URL is
    // broken for :latest.
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

    // A rebuild of the same version shows just the version.
    const haveNext = verLike(next) && next !== "?" && norm(next) !== norm(cur);
    const verHdr = (upd && haveNext) ? `${esc(cur)} → <b>${esc(next)}</b>` : `<b>${esc(cur)}</b>`;
    const pillTxt = upd
      ? esc(seUpd ? kindLabel(st) : T("update"))
      : esc(noUpstream(st) ? noUpstreamLabel(st) : T("uptodate"));

    // A critical pill shows the warning glyph instead of the dot.
    const dotOrWarn = rc === "crit" ? "⚠ " : '<span class="sl-dot"></span>';
    return `
      <div class="sl-bh">
        <span class="sl-ver">${verHdr}</span>
        <span class="sl-pill sl-${rc}">${dotOrWarn}${pillTxt}</span>
        ${st.unmaintained ? `<span class="sl-pill sl-unmaint" title="${esc(st.unmaintained_reason || T("unmaintained"))}">⚠ ${esc(T("unmaintained"))}</span>` : ""}
        ${!st.unmaintained && st.ca_deprecated ? `<span class="sl-pill sl-dep" title="${esc(st.ca_deprecated_note || T("deprecated"))}">⚠ ${esc(T("deprecated"))}</span>` : ""}
        <span class="sl-jump" title="${esc(jump)}">${esc(jump)}</span>
        <span class="sl-bh-right">${upd ? `<a class="sl-upd" href="#" title="${esc(T("updateHint"))}">${esc(T("updateNow"))}</a>` : ""}${gh}<span class="sl-x" title="${esc(T("close"))}">✕</span></span>
      </div>
      ${st.unmaintained ? `<div class="sl-unmaint-note"><h4>⚠ ${esc(T("unmaintained"))}</h4>${esc(st.unmaintained_reason || T("unmaintained"))}. ${esc(T("unmaintainedHint"))}</div>` : ""}
      ${!st.unmaintained && st.ca_deprecated ? `<div class="sl-unmaint-note sl-dep-note"><h4>⚠ ${esc(T("deprecated"))}</h4>${esc(st.ca_deprecated_note || T("deprecated"))}</div>` : ""}
      ${summary}${raw}
      ${src ? `<div class="sl-bf"><span>${src}</span></div>` : ""}`;
  }

  // "Update now" runs Unraid's own update path and never touches the Docker
  // socket. Without the confirmation it hands openDocker() the command Unraid's
  // own confirm callback runs, so no dialog is ever clicked programmatically.

  // clickNativeAnchor clicks the row's own "apply update" or "rebuild ready" link.
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

  // anchorName reads the name from a native update link's updateContainer('<name>')
  // call, else from its row.
  function anchorName(a) {
    const oc = a.getAttribute("onclick") || "";
    const m = /updateContainer\(\s*['"]([^'"]+)['"]/i.exec(oc);
    if (m) return m[1];
    const tr = a.closest ? a.closest("tr") : null;
    return tr ? rowName(tr) : "";
  }

  function runNativeUpdateNoConfirm(name) {
    if (!name) return false;
    if (typeof window.openDocker === "function") {
      try {
        window.openDocker("update_container " + encodeURIComponent(name), T("updatingOne"), "", "loadlist");
        return true;
      } catch (e) { /* fall back to the native link */ }
    }
    return clickNativeAnchor(name);
  }

  function runNativeUpdateWithConfirm(name) {
    if (!name) return false;
    if (typeof window.updateContainer === "function") {
      try { window.updateContainer(name); return true; } catch (e) {}
    }
    return clickNativeAnchor(name);
  }

  // With a silent update, the row logo shows a spinner while the hidden log is up.
  // It is painted only once the log exists, so a cancelled confirm leaves none
  // behind. Unraid's re-render of the table drops it, so every mutation paints it
  // again for the names still updating.
  const updatingNames = new Set(); // lower-cased names
  let spinLive = false;            // the hidden log exists
  function armSpin(names) {
    const arr = Array.isArray(names) ? names : (names ? [names] : []);
    for (const nm of arr) { const n = norm(nm); if (n) updatingNames.add(n); }
    if (spinLive) refreshSpinners();
  }
  // The logo in td.ct-name is also Unraid's start/stop menu trigger.
  function rowIconHost(tr) {
    const cell = tr.querySelector("td.ct-name");
    if (!cell) return null;
    const img = cell.querySelector("img");
    if (img && img.parentElement) return img.parentElement;
    return cell.querySelector("span.hand") || cell;
  }
  function applySpinner(host) {
    if (!host || host.querySelector(":scope > .sl-spin")) return;
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
  function refreshSpinners() {
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
  // The containers Unraid's bulk update acts on.
  function pendingUpdateNames() {
    try {
      const list = window.docker;
      if (!Array.isArray(list)) return [];
      return list.filter((d) => d && Number(d.update) === 1).map((d) => d && d.name).filter(Boolean);
    } catch (e) { return []; }
  }

  // armSilentLogHide hides Unraid's progress log for a silent update and frees the
  // scroll lock. It acts only once the log exists, so a confirm dialog keeps its
  // backdrop. Unraid enables the log's Done button only on completion; clicking
  // it then runs Unraid's own teardown and reload, without any text match. The
  // shared dialog node gets no inline style, and a cap reveals a log that never
  // finishes.
  let silentArm = null;
  function armSilentLogHide(names) {
    armSpin(names);
    if (silentArm) return;
    let obs = null, poll = 0, cap = 0, armedBody = false, sawDisabled = false;
    const stop = () => {
      try { obs && obs.disconnect(); } catch (e) {}
      clearInterval(poll); clearTimeout(cap);
      document.body.classList.remove("sl-hide-nchan");
      clearSpinners();
      silentArm = null;
    };
    const tick = () => {
      // #swaltext tells the log from a confirm dialog that reuses the same node,
      // so a confirm is never hidden or dismissed.
      const log = document.querySelector(".sweet-alert.nchan");
      if (!log || !log.querySelector("#swaltext")) return;
      if (!armedBody) { document.body.classList.add("sl-hide-nchan"); armedBody = true; }
      spinLogAppeared();
      // Only a real change touches the class, so this cannot feed an observer.
      if (document.body.classList.contains("stop-scrolling")) document.body.classList.remove("stop-scrolling");
      const done = log.querySelector("button.confirm");
      if (!done) return;
      if (done.disabled) { sawDisabled = true; return; }
      if (sawDisabled) { done.click(); stop(); }
    };
    silentArm = { stop };
    try {
      obs = new MutationObserver(tick);
      // Attributes are not observed, so the body class written in tick() cannot
      // feed the observer; the poll catches the Done button's disabled flag.
      obs.observe(document.body, { childList: true, subtree: true });
    } catch (e) {}
    poll = setInterval(tick, 400);
    cap = setTimeout(stop, 15 * 60 * 1000);
    tick();
  }

  // armJumpMarquee scrolls the jump line only when it overflows and motion is
  // allowed. Two identical copies moved by translateX(-50%) loop seamlessly.
  function armJumpMarquee(bubble) {
    try {
      const jumpEl = bubble.querySelector(".sl-jump");
      if (!jumpEl) return;
      const reduceMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      const full = jumpEl.textContent;
      if (reduceMotion || !full || jumpEl.scrollWidth <= jumpEl.clientWidth) return;
      const track = el("span", "sl-jump-track");
      track.appendChild(el("span", "sl-jump-txt", esc(full)));
      const clone = el("span", "sl-jump-txt", esc(full));
      clone.setAttribute("aria-hidden", "true");
      track.appendChild(clone);
      jumpEl.textContent = "";
      jumpEl.appendChild(track);
      jumpEl.classList.add("sl-jump-marquee");
      // Longer text scrolls longer, so the reading speed stays about the same.
      track.style.animationDuration = Math.max(6, Math.min(24, full.length / 8)) + "s";
    } catch (e) {}
  }

  function openFor(anchor, st) {
    close();
    // The backdrop goes in first so it paints behind the window.
    const bd = el("div", "sl-backdrop");
    if (isLightBg()) bd.classList.add("sl-light");
    bd.addEventListener("click", () => close());
    document.body.appendChild(bd);
    const b = el("div", "sl-bubble", bubbleHTML(st));
    if (isLightBg()) b.classList.add("sl-light");
    document.body.appendChild(b);
    b._backdrop = bd;
    // The stored size is the border-box size, which round-trips only because
    // .sl-bubble uses box-sizing:border-box; otherwise it would grow on every open.
    try {
      const s = JSON.parse(localStorage.getItem(SZ_KEY) || "null");
      if (s && s.w) b.style.width = s.w + "px";
      if (s && s.h) b.style.height = s.h + "px";
    } catch (e) {}
    armJumpMarquee(b); // needs the restored width
    // CSS centres the window, so `anchor` goes unused.
    b.querySelector(".sl-x").addEventListener("click", (e) => { e.stopPropagation(); close(); });
    const updBtn = b.querySelector(".sl-upd");
    if (updBtn) updBtn.addEventListener("click", (e) => {
      e.preventDefault(); e.stopPropagation();
      const cname = st.container && st.container.name;
      if (silentUpdate) armSilentLogHide(cname);
      const ok = confirmUpdate ? runNativeUpdateWithConfirm(cname) : runNativeUpdateNoConfirm(cname);
      if (ok) close();
      else { updBtn.textContent = T("updateGone"); updBtn.classList.add("sl-upd-off"); }
    });
    try {
      const ro = new ResizeObserver(() => {
        try { localStorage.setItem(SZ_KEY, JSON.stringify({ w: b.offsetWidth, h: b.offsetHeight })); } catch (e) {}
      });
      ro.observe(b);
      b._ro = ro;
    } catch (e) {}
    open = b;
  }

  // The update-all button calls Unraid's own updateAll() and counts the same
  // containers it acts on. It shows only while an update is pending, and writes
  // only on change, so the MutationObserver does not feed on it.
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
          // updateAll() has no confirm dialog of its own.
          if (confirmUpdate && !window.confirm(T("confirmAll").replace("%n", n))) return;
          if (silentUpdate) armSilentLogHide(pendingUpdateNames());
          fireNativeUpdateAll();
        });
        // .ToggleViewMode is a right-aligned flex row, so its first child sits
        // directly left of the toggle.
        toggle.insertBefore(btn, toggle.firstChild);
      }

      const n = nativeUpdateAllAvailable() ? pendingUpdateCount() : 0;
      if (btn.dataset.slN !== String(n)) {
        btn.dataset.slN = String(n);
        btn.textContent = `${T("updateAll")} (${n})`;
      }
      const want = n >= 1 ? "" : "none";
      if (btn.style.display !== want) btn.style.display = want;
    } catch (e) {}
  }

  function tagRows() {
    let n = 0;
    for (const tr of findRows()) {
      const st = byName[norm(rowName(tr))];
      if (!st) continue;
      const cell = findUpdateCell(tr);
      if (!cell || cell.getAttribute(MARK)) continue;
      const upd = isUpdate(st), seUpd = hasUpdate(st);
      let chip;
      // An unmaintained or demoted app gets a warning chip in place of the
      // changelog chip; it still opens the window with the reason.
      if (st.unmaintained) {
        chip = el("a", "sl-chip sl-unmaint", `${WARN_ICON}<span>${esc(T("unmaintained"))}</span>`);
        chip.title = `ShipLog: ${st.unmaintained_reason || T("unmaintained")}`;
      } else if (st.ca_deprecated) {
        chip = el("a", "sl-chip sl-dep", `${WARN_ICON}<span>${esc(T("deprecated"))}</span>`);
        chip.title = `ShipLog: ${st.ca_deprecated_note || T("deprecated")}`;
      } else {
        const rc = upd ? (seUpd ? riskClass(st) : "low") : (noUpstream(st) ? "grey" : "ok");
        const label = upd ? (seUpd ? kindLabel(st) : T("update")) : T("uptodate");
        chip = el("a", "sl-chip", `${LOG_ICON}<span>${esc(T("changelog"))}</span><span class="sl-amp sl-${rc}"></span>`);
        chip.title = `ShipLog: ${label} · ${T("clickHint")}`;
      }
      chip.href = "#";
      chip.addEventListener("click", (e) => { e.preventDefault(); e.stopPropagation(); openFor(chip, st); });
      const row = el("div", "sl-chiprow");
      row.appendChild(chip);
      cell.appendChild(row);
      cell.setAttribute(MARK, "1");
      n++;
    }
    return n;
  }

  document.addEventListener("click", (e) => {
    if (open && !open.contains(e.target) && !e.target.closest(".sl-chip")) close();
  });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") close(); });

  // The confirm and silent preferences also apply to Unraid's own update links.
  // The capture phase acts before their onclick opens a dialog, and the match
  // covers update links only, so remove or stop dialogs stay untouched.
  document.addEventListener("click", (e) => {
    try {
      const a = e.target && e.target.closest ? e.target.closest("a[onclick], a.exec, [onclick]") : null;
      if (!a || a.closest(".sl-chip") || a.closest(".sl-chiprow") || a.closest(".sl-bubble")) return;
      const blob = norm(a.textContent) + " " + norm(a.getAttribute("onclick") || "");
      if (!/updatecontainer|apply update|aktualisierung anwenden|force update|update erzwingen|rebuild ready|installupdate/.test(blob)) return;
      if (silentUpdate) armSilentLogHide(anchorName(a));
      if (!confirmUpdate) {
        // Without openDocker the native handler runs with its confirm, since
        // clicking the same link again would loop.
        const name = anchorName(a);
        if (name && typeof window.openDocker === "function") {
          e.preventDefault(); e.stopPropagation();
          close(); // stopPropagation keeps the outside-click listener from closing it
          try { window.openDocker("update_container " + encodeURIComponent(name), T("updatingOne"), "", "loadlist"); }
          catch (err) {
            if (typeof window.updateContainer === "function") window.updateContainer(name);
            else clickNativeAnchor(name);
          }
        }
      }
    } catch (err) {}
  }, true);

  async function boot() {
    // The update-all button needs no engine.
    injectUpdateAllButton();
    // Unraid re-renders the table on its auto-refresh. An update streams its log
    // as hundreds of mutations a second, and a full rescan per mutation freezes
    // the tab, so mutations inside dialogs are ignored and bursts coalesce into a
    // leading and a trailing pass.
    let moT = null, moTrail = false;
    const moPass = () => {
      moTrail = false;
      tagRows(); injectUpdateAllButton(); refreshSpinners();
      moT = setTimeout(() => { moT = null; if (moTrail) moPass(); }, 60);
    };
    const mo = new MutationObserver((recs) => {
      let relevant = false;
      for (let i = 0; i < recs.length && !relevant; i++) {
        const t = recs[i].target;
        if (t && t.nodeType === 1 && t.closest && t.closest(".sweet-alert, .sweet-overlay, .sl-bubble, .sl-backdrop")) continue;
        relevant = true;
      }
      if (!relevant) return;
      if (moT) { moTrail = true; return; }
      moPass();
    });
    try { mo.observe(document.body, { childList: true, subtree: true }); } catch (e) {}

    const ok = await load();
    if (!ok) return;
    const n = tagRows();
    console.log(`${TAG} tagged ${n} container(s) with updates`);
  }
  boot();

  const DEMO_DATA = [
    { container: { name: "Immich", image: "ghcr.io/imagegenius/immich", tag: "v1.122.0" },
      newest_tag: "v1.124.2", kind: "minor", risk: "medium", risk_reason: "2 minor versions",
      changelog: { from_tag: "v1.122.0", to_tag: "v1.124.2", skipped_count: 2,
        raw: "## v1.124.2\n- fix: transcode leak\n## v1.124.0\n- breaking: requires PostgreSQL >= 15",
        source: "GitHub releases via OCI label", url: "https://github.com/immich-app/immich/releases" } },
  ];
})();
