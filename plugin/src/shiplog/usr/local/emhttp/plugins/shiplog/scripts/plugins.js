/* ShipLog in Unraid's Plugins tab: a changelog chip beside each plugin's
   version, as docker.js puts one beside each container, and the same window
   with the release notes. The engine's /api/plugins reads only what Unraid's
   own plugin check fetched, so the chip never disagrees with the Status
   column. Updates still run through Unraid's own update link. */
(function () {
  "use strict";
  const PROXY = "/plugins/shiplog/server/status.php";
  const MARK = "data-shiplog";
  const TAG = "[ShipLog]";
  const LOG_ICON =
    '<svg class="sl-ico" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" ' +
    'stroke-linecap="round"><rect x="3" y="2" width="10" height="12" rx="1.5"/>' +
    '<line x1="5.4" y1="5.5" x2="10.6" y2="5.5"/><line x1="5.4" y1="8" x2="10.6" y2="8"/>' +
    '<line x1="5.4" y1="10.5" x2="9" y2="10.5"/></svg>';
  const G = window.shiplogGlyphs || {};
  const I18N = (window.shiplogI18n && typeof window.shiplogI18n === "object") ? window.shiplogI18n : {};
  const EN = {
    changelog: "Changelog", clickHint: "click for the changelog", uptodate: "up to date",
    raw: "Changelog", close: "close", updateNow: "Update now",
    plgUpdate: "Update available",
    plgUpdateHint: "Runs Unraid's own update for this plugin",
    plgIncompatible: "Needs another Unraid",
    plgIncompatibleHint: "Version %v needs another Unraid version (%r), so Unraid will not install it here.",
    plgUnchecked: "Not checked yet",
    plgUncheckedHint: "Unraid has not checked this plugin for updates since boot. Click Check for Updates at the top of the Plugins tab.",
    plgNoNotes: "This plugin ships no release notes.",
    plgSupport: "Support thread",
  };
  function T(k) { return I18N["d_" + k] || EN[k] || k; }

  function el(tag, cls, html) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (html != null) n.innerHTML = html;
    return n;
  }
  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }
  function labelled(glyph, words) {
    return `<span class="sl-g">${glyph || ""}</span><span class="sl-l">${esc(words)}</span>`;
  }
  function isLightBg() {
    try {
      const m = /(\d+),\s*(\d+),\s*(\d+)/.exec(getComputedStyle(document.body).backgroundColor);
      return !!m && (0.299 * +m[1] + 0.587 * +m[2] + 0.114 * +m[3]) / 255 > 0.5;
    } catch (e) { return false; }
  }
  // The same markdown subset as docker.js: the text is escaped whole before a
  // few patterns become HTML, so nothing from a .plg reaches innerHTML as markup.
  function renderMd(md) {
    const inline = (s) => s
      .replace(/\*\*([^*\n]+)\*\*/g, "<b>$1</b>")
      .replace(/`([^`\n]+)`/g, "<code>$1</code>")
      .replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)"']+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
      .replace(/(^|[^"'>=])(https?:\/\/[^\s<"']+)/g, '$1<a href="$2" target="_blank" rel="noopener">$2</a>');
    let html = "", inList = false;
    const escaped = String(md).replace(/\r\n/g, "\n").replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
    for (const line of escaped.split("\n")) {
      const l = line.trim();
      const li = /^[-*]\s+(.*)$/.exec(l);
      if (li) {
        if (!inList) { html += "<ul>"; inList = true; }
        html += `<li>${inline(li[1])}</li>`;
        continue;
      }
      if (inList) { html += "</ul>"; inList = false; }
      const h = /^#{1,6}\s*(.*)$/.exec(l);
      if (h) { if (h[1]) html += `<div class="h">${inline(h[1])}</div>`; continue; }
      if (l === "") { html += '<div class="sp"></div>'; continue; }
      html += `<div>${inline(l)}</div>`;
    }
    if (inList) html += "</ul>";
    return html;
  }

  // The chip's dot: plugins carry no risk grade, so a waiting update is amber,
  // one this Unraid cannot install and one not checked yet are grey.
  const DOT = { available: "mid", incompatible: "grey", unchecked: "grey", current: "ok" };
  function dot(p) { return DOT[p.state] || "grey"; }
  function stateLabel(p) {
    switch (p.state) {
      case "available": return T("plgUpdate");
      case "incompatible": return T("plgIncompatible");
      case "unchecked": return T("plgUnchecked");
    }
    return T("uptodate");
  }
  function range(p) {
    return [p.min ? "≥ " + p.min : "", p.max ? "≤ " + p.max : ""].filter(Boolean).join(", ");
  }

  function windowHTML(p) {
    const upd = p.state === "available";
    const ver = (upd || p.state === "incompatible") && p.latest
      ? `${esc(p.installed)} → <b>${esc(p.latest)}</b>` : `<b>${esc(p.installed)}</b>`;
    const support = p.support
      ? `<a class="sl-gh" href="${esc(p.support)}" target="_blank" rel="noopener" title="${esc(T("plgSupport"))}">${labelled(G.ExternalLink, T("plgSupport"))}</a>` : "";
    const update = upd
      ? `<a class="sl-upd" href="#" title="${esc(T("plgUpdateHint"))}">${labelled(G.Sync, T("updateNow"))}</a>` : "";
    let note = "";
    if (p.state === "incompatible") {
      note = `<div class="sl-unmaint-note sl-newer-note">${esc(T("plgIncompatibleHint").replace("%v", () => p.latest).replace("%r", () => range(p)))}</div>`;
    } else if (p.state === "unchecked") {
      note = `<div class="sl-unmaint-note sl-newer-note">${esc(T("plgUncheckedHint"))}</div>`;
    }
    const notes = p.changes
      ? `<div class="sl-sec"><h4>${esc(T("raw"))}</h4><div class="sl-raw">${renderMd(p.changes)}</div></div>`
      : `<div class="sl-sec"><div class="sl-none" style="color:var(--sl-dim2)">${esc(T("plgNoNotes"))}</div></div>`;
    return `
      <div class="sl-bh">
        <span class="sl-ver">${ver}</span>
        <span class="sl-pill sl-${dot(p)}"><span class="sl-dot"></span>${esc(stateLabel(p))}</span>
        <span class="sl-jump" title="${esc(p.name)}">${esc(p.name)}</span>
        <span class="sl-bh-right">${update}${support}<span class="sl-x" role="button" tabindex="0" aria-label="${esc(T("close"))}" title="${esc(T("close"))}">${G.Close || "×"}</span></span>
      </div>
      ${note}${notes}`;
  }

  let open = null;
  function close() {
    if (!open) return;
    if (open._backdrop) open._backdrop.remove();
    open.remove();
    open = null;
  }

  // The row's own update link, the one Unraid's Status column shows while an
  // update waits. It opens Unraid's dialog, alert check included.
  function nativeUpdate(tr) {
    if (!tr) return null;
    for (const a of tr.querySelectorAll("a, span, input")) {
      const on = String(a.getAttribute("onclick") || "");
      if (/plugin update /.test(on) && !(a.closest && a.closest(".sl-chiprow"))) return a;
    }
    return null;
  }

  function openFor(p, tr) {
    close();
    const bd = el("div", "sl-backdrop");
    const b = el("div", "sl-bubble", windowHTML(p));
    if (isLightBg()) { bd.classList.add("sl-light"); b.classList.add("sl-light"); }
    b.setAttribute("role", "dialog");
    b.setAttribute("aria-modal", "true");
    b.setAttribute("aria-label", "ShipLog: " + p.name);
    bd.addEventListener("click", close);
    document.body.appendChild(bd);
    document.body.appendChild(b);
    b._backdrop = bd;
    const x = b.querySelector(".sl-x");
    if (x) {
      x.addEventListener("click", (e) => { e.stopPropagation(); close(); });
      x.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); close(); } });
    }
    const u = b.querySelector(".sl-upd");
    if (u) u.addEventListener("click", (e) => {
      e.preventDefault(); e.stopPropagation();
      const a = nativeUpdate(tr);
      close();
      if (a) a.click();
    });
    open = b;
  }

  function rowOf(n) {
    while (n && n.tagName !== "TR") n = n.parentNode;
    return n;
  }
  function cells(tr) { return Array.from(tr.children).filter((c) => c.tagName === "TD"); }

  let byFile = {};
  // Each row carries its .plg file name on its remove checkbox. The version is
  // the fourth cell, which Unraid rewrites once its own check answers, so a
  // cell whose chip went missing gets it back.
  function tagRows() {
    let n = 0;
    for (const box of document.querySelectorAll("input.remove")) {
      const p = byFile[box.getAttribute("data")];
      const tr = rowOf(box);
      if (!p || !tr) continue;
      const cell = cells(tr)[3];
      if (!cell || cell.querySelector(":scope > .sl-chiprow")) continue;
      const chip = el("a", "sl-chip", `${LOG_ICON}<span>${esc(T("changelog"))}</span><span class="sl-amp sl-${dot(p)}"></span>`);
      chip.href = "#";
      chip.title = `ShipLog: ${stateLabel(p)} · ${T("clickHint")}`;
      chip.addEventListener("click", (e) => { e.preventDefault(); e.stopPropagation(); openFor(p, tr); });
      const row = el("div", "sl-chiprow");
      row.appendChild(chip);
      cell.appendChild(row);
      cell.setAttribute(MARK, "1");
      n++;
    }
    return n;
  }

  async function load() {
    try {
      const r = await fetch(PROXY + "?path=plugins", { credentials: "same-origin" });
      if (!r.ok) return false;
      const list = await r.json();
      if (!Array.isArray(list)) return false;
      byFile = {};
      for (const p of list) if (p && p.file) byFile[p.file] = p;
      return true;
    } catch (e) {
      console.log(`${TAG} no plugin list: ${e}`);
      return false;
    }
  }

  document.addEventListener("keydown", (e) => { if (e.key === "Escape") close(); });

  async function boot() {
    if (!(await load())) return;
    console.log(`${TAG} tagged ${tagRows()} plugin(s)`);
    // The list arrives by AJAX and its cells are rewritten as Unraid's checks
    // land, so every change gets one coalesced pass.
    let pending = false;
    const mo = new MutationObserver((recs) => {
      for (const r of recs) {
        const t = r.target;
        if (t && t.closest && t.closest(".sl-bubble, .sl-backdrop, .sweet-alert")) continue;
        if (pending) return;
        pending = true;
        setTimeout(() => { pending = false; tagRows(); }, 60);
        return;
      }
    });
    try { mo.observe(document.body, { childList: true, subtree: true }); } catch (e) {}
    // Check for Updates refetches /tmp/plugins. Unraid disables its button
    // while the checks run, so the list is read again once it is enabled.
    document.addEventListener("click", (e) => {
      const t = e.target;
      if (!t || !t.closest || !t.closest("#checkall")) return;
      const started = Date.now();
      const wait = setInterval(async () => {
        const b = document.querySelector("#checkall input");
        if (b && b.disabled && Date.now() - started < 120000) return;
        clearInterval(wait);
        if (!(await load())) return;
        for (const c of document.querySelectorAll(".sl-chiprow")) c.remove();
        tagRows();
      }, 1000);
    }, true);
  }
  boot();
  window.shiplogPlugins = { windowHTML, dot, tagRows };
})();
