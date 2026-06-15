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
  function close() { if (open) { open.remove(); open = null; } }

  function bubbleHTML(st) {
    const c = st.container || {};
    const cl = st.changelog || {};
    const rc = riskClass(st);
    const cur = c.tag ? (c.tag === "latest" ? ":latest" : c.tag) : (c.image || "?");
    const next = st.newest_tag || cl.to_tag || "?";
    const jump = cl.skipped_count > 1 ? `skips ${cl.skipped_count} releases` : (st.risk_reason || "");

    let summary = "";
    if (cl.summary && (cl.summary.bullets || cl.summary.breaking)) {
      const lis = []
        .concat((cl.summary.breaking || []).map((t) => `<li class="sl-warn">${esc(t)}</li>`))
        .concat((cl.summary.bullets || []).map((t) => `<li>${esc(t)}</li>`))
        .join("");
      summary = `<div class="sl-sec"><h4>⚡ Summary <span class="sl-ai">AI · ${esc(cl.summary.model || "Ollama")}</span></h4><ul>${lis}</ul></div>`;
    }

    let raw = "";
    if (cl.raw) {
      raw = `<div class="sl-sec"><h4>Changelog (raw)</h4><div class="sl-raw">${
        esc(cl.raw).replace(/^(#{1,3}.*)$/gm, '<span class="h">$1</span>')
      }</div></div>`;
    } else if (!summary) {
      raw = `<div class="sl-sec"><div style="color:#6f6f6f">No changelog source resolved for this image.</div></div>`;
    }

    const src = cl.source ? `Source: ${esc(cl.source)}` : "Source: —";
    const link = cl.url
      ? `<a class="sl-link" href="${esc(cl.url)}" target="_blank" rel="noopener">Open ↗</a>`
      : "";

    return `
      <div class="sl-bh">
        <span class="sl-ver">${esc(cur)} → <b>${esc(next)}</b></span>
        <span class="sl-pill sl-${rc}"><span class="sl-dot"></span>${esc(kindLabel(st))}</span>
        <span class="sl-jump">${esc(jump)}</span>
        <span class="sl-x" title="close">✕</span>
      </div>
      ${summary}${raw}
      <div class="sl-bf"><span>${src}</span>${link}</div>`;
  }

  function openFor(anchor, st) {
    close();
    const b = el("div", "sl-bubble", bubbleHTML(st));
    document.body.appendChild(b);
    const r = anchor.getBoundingClientRect();
    const w = b.offsetWidth || 560;
    const maxLeft = window.scrollX + document.documentElement.clientWidth - w - 12;
    let left = Math.min(window.scrollX + r.left, maxLeft);
    left = Math.max(window.scrollX + 8, left);
    b.style.left = left + "px";
    b.style.top = window.scrollY + r.bottom + 8 + "px";
    b.querySelector(".sl-x").addEventListener("click", (e) => { e.stopPropagation(); close(); });
    open = b;
  }

  // ──────────────────────────────────────────────────────── inject
  function tagRows() {
    let n = 0;
    for (const tr of findRows()) {
      const st = byName[norm(rowName(tr))];
      if (!st || !hasUpdate(st)) continue;
      const cell = findUpdateCell(tr);
      if (!cell || cell.getAttribute(MARK)) continue;
      const chip = el("a", "sl-chip", `${LOG_ICON}<span>Changelog</span><span class="sl-amp sl-${riskClass(st)}"></span>`);
      chip.href = "#";
      chip.title = `ShipLog: ${kindLabel(st)} — click for the changelog`;
      chip.addEventListener("click", (e) => { e.preventDefault(); e.stopPropagation(); openFor(chip, st); });
      cell.appendChild(chip);
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
