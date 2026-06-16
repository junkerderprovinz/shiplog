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
  // Follows Unraid's page language (full 26-locale catalogue is P1.5). The
  // risk reason itself is engine-generated and stays English for now.
  const LANG = (document.documentElement.lang || navigator.language || "en").slice(0, 2).toLowerCase();
  const STR = {
    en: {
      changelog: "Changelog", clickHint: "click for the changelog",
      skips: "skips %n releases", newest: "newest %d",
      summary: "Summary", raw: "Changelog (raw)", source: "Source", close: "close", uptodate: "up to date",
      deprecated: "DEPRECATED",
      none: "No changelog text found for this image — open the repo (top-right) for the release notes.",
    },
    de: {
      changelog: "Changelog", clickHint: "Changelog anzeigen",
      skips: "überspringt %n Releases", newest: "neuestes %d",
      summary: "Zusammenfassung", raw: "Changelog (roh)", source: "Quelle", close: "schließen", uptodate: "aktuell",
      deprecated: "VERALTET",
      none: "Kein Changelog-Text für dieses Image gefunden — öffne das Repo (oben rechts) für die Release Notes.",
    },
  };
  function T(k) { return ((STR[LANG] || STR.en)[k]) || STR.en[k]; }

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
  function findRows() {
    const candidates = [
      "table#docker_list tbody tr",
      "table.tablesorter tbody tr",
      "div.tabs table tbody tr",
      "table tbody tr",
    ];
    for (const sel of candidates) {
      const rows = Array.from(document.querySelectorAll(sel))
        .filter((tr) => tr.querySelector("img") && tr.textContent.trim().length > 1);
      if (rows.length) return rows;
    }
    return [];
  }
  function rowName(tr) {
    const img = tr.querySelector("img");
    const cell = img ? img.closest("td") || tr : tr;
    const a = cell.querySelector("a");
    const name = a && a.textContent.trim()
      ? a.textContent.trim()
      : (cell.textContent || tr.textContent).trim().split("\n")[0].trim();
    return name.slice(0, 60);
  }
  function findUpdateCell(tr) {
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
    const upd = hasUpdate(st);
    const rc = upd ? riskClass(st) : "grey";
    // changelog from/to are the image TAGS ("latest"/"7dtd"), not versions —
    // show a real version when we have one. Newest release tag comes from the
    // resolved release entries; current is the running tag if it looks like a
    // version, else the short digest (a :latest digest can't be mapped to a tag).
    const verLike = (t) => /^v?\d+\.\d+/.test(t || "");
    const entries = Array.isArray(cl.entries) ? cl.entries : [];
    const newestRel = entries[0] && entries[0].tag ? entries[0].tag : "";
    // Current = the running tag (a version, or "latest"); never the cryptic
    // digest hash — a :latest image can't be mapped back to a version number.
    const cur = verLike(c.tag) ? c.tag : (c.tag || "latest");
    const next = (verLike(st.newest_tag) ? st.newest_tag : "")
      || (verLike(newestRel) ? newestRel : "")
      || newestRel || st.newest_tag || "?";
    const fmtDate = (iso) => {
      const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso || "");
      return m ? `${m[3]}.${m[2]}.${m[1]}` : ""; // DD.MM.YYYY
    };
    const relDate = entries[0] ? fmtDate(entries[0].published_at) : "";
    let jump = cl.skipped_count > 1 ? T("skips").replace("%n", cl.skipped_count) : (st.risk_reason || "");
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
      ? `<a class="sl-gh" href="${esc(href)}" target="_blank" rel="noopener">GitHub ↗</a>`
      : "";
    const src = cl.source ? `${esc(T("source"))}: ${esc(cl.source)}` : "";

    const verHdr = upd ? `${esc(cur)} → <b>${esc(next)}</b>` : `<b>${esc(cur)}</b>`;
    const pillTxt = upd ? esc(kindLabel(st)) : esc(T("uptodate"));

    return `
      <div class="sl-bh">
        <span class="sl-ver">${verHdr}</span>
        <span class="sl-pill sl-${rc}"><span class="sl-dot"></span>${pillTxt}</span>
        ${cl.deprecated ? `<span class="sl-pill sl-high" title="upstream repository is archived">⚠ ${esc(T("deprecated"))}</span>` : ""}
        <span class="sl-jump">${esc(jump)}</span>
        <span class="sl-bh-right">${gh}<span class="sl-x" title="${esc(T("close"))}">✕</span></span>
      </div>
      ${summary}${raw}
      ${src ? `<div class="sl-bf"><span>${src}</span></div>` : ""}`;
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

  // ──────────────────────────────────────────────────────── inject
  function tagRows() {
    let n = 0;
    for (const tr of findRows()) {
      const st = byName[norm(rowName(tr))];
      if (!st) continue; // no engine data for this row → leave it untouched
      const cell = findUpdateCell(tr);
      if (!cell || cell.getAttribute(MARK)) continue;
      const rc = hasUpdate(st) ? riskClass(st) : "ok";
      const label = hasUpdate(st) ? kindLabel(st) : T("uptodate");
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
    const ok = await load();
    if (!ok) return; // engine unreachable → leave the Docker tab untouched
    const n = tagRows();
    console.log(`${TAG} tagged ${n} container(s) with updates`);
    // Unraid re-renders the table on its auto-refresh — re-tag new rows.
    const mo = new MutationObserver(() => tagRows());
    try { mo.observe(document.body, { childList: true, subtree: true }); } catch (e) {}
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
