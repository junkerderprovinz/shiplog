// ==UserScript==
// @name         ShipLog — Docker-tab bubble (feasibility spike)
// @namespace    https://github.com/junkerderprovinz/shiplog
// @version      0.1.0
// @description  P2.0 spike: inject a risk badge + click-to-open changelog bubble next to every container in Unraid's native Docker tab. Proves the DOM hook on the real box and harvests the exact selectors before the full .plg plugin is built.
// @match        http*://*/Docker
// @match        http*://*/Dashboard
// @run-at       document-idle
// @grant        none
// ==/UserScript==
//
// HOW TO RUN (no install needed):
//   1. Open Unraid → the DOCKER tab.
//   2. Open the browser DevTools console (F12 → "Console").
//   3. Paste this whole file, press Enter.
//   => A risk badge appears next to each container; click it for the bubble.
//      A toast (bottom-right) reports how many rows were tagged.
//
// WHAT TO SEND BACK if it does NOT tag your containers:
//   copy the "[ShipLog spike]" diagnostics block the console prints and paste
//   it back — it dumps the Docker table structure so the exact selectors for
//   your Unraid version can be locked in for the real plugin.
//
// This spike uses DEMO data. Real per-container data arrives in the plugin via a
// server-side PHP proxy to the engine (same-origin → no CORS). Optionally set
// ENGINE_URL below to try the live engine now (only works if the engine sends
// CORS headers; otherwise it silently falls back to demo).

(function () {
  "use strict";

  // ──────────────────────────────────────────────────────────── config
  const CONFIG = {
    ENGINE_URL: "", // e.g. "http://192.168.20.51:8484" — leave "" for demo data
    BADGE_ATTR: "data-shiplog", // marker so we never double-tag a row
  };

  const TAG = "[ShipLog spike]";

  // ──────────────────────────────────────────────────────────── styles
  const CSS = `
  .sl-badge{display:inline-flex;align-items:center;gap:5px;padding:1px 8px;margin:0 6px 0 0;
    border-radius:999px;font:600 11px/1.4 "Segoe UI",system-ui,sans-serif;letter-spacing:.3px;
    border:1px solid;background:transparent;cursor:pointer;vertical-align:middle;white-space:nowrap}
  .sl-badge .sl-dot{width:7px;height:7px;border-radius:50%}
  .sl-low {color:#3fae6a;border-color:#2f6b46}  .sl-low  .sl-dot{background:#3fae6a}
  .sl-mid {color:#d6a243;border-color:#7a6028}  .sl-mid  .sl-dot{background:#d6a243}
  .sl-high{color:#d9534f;border-color:#7a3331}  .sl-high .sl-dot{background:#d9534f}
  .sl-grey{color:#9a9a9a;border-color:#444}     .sl-grey .sl-dot{background:#8a8a8a}
  .sl-ok  {color:#6f6f6f;border-color:#333}      .sl-ok   .sl-dot{background:#525252}

  .sl-bubble{position:absolute;z-index:99999;width:560px;max-width:92vw;
    background:#1d1d1d;color:#e6e6e6;border:1px solid #525252;border-radius:12px;
    box-shadow:0 14px 40px rgba(0,0,0,.6);font:14px/1.5 "Segoe UI",system-ui,sans-serif}
  .sl-bubble .sl-bh{display:flex;align-items:center;gap:10px;flex-wrap:wrap;
    padding:12px 16px;border-bottom:1px solid #2a2a2a}
  .sl-bubble .sl-ver{font-family:Consolas,monospace;font-size:13px;color:#9a9a9a}
  .sl-bubble .sl-ver b{color:#e6e6e6;font-weight:600}
  .sl-bubble .sl-jump{color:#6f6f6f;font-size:12px}
  .sl-bubble .sl-x{margin-left:auto;cursor:pointer;color:#9a9a9a;border:1px solid #333;
    border-radius:7px;width:26px;height:26px;display:flex;align-items:center;justify-content:center}
  .sl-bubble .sl-sec{padding:12px 16px;border-bottom:1px solid #2a2a2a}
  .sl-bubble h4{font:600 11px/1.4 "Segoe UI",sans-serif;text-transform:uppercase;letter-spacing:.7px;
    color:#6f6f6f;margin:0 0 8px;display:flex;align-items:center;gap:8px}
  .sl-bubble .sl-ai{font:400 10px/1 "Segoe UI",sans-serif;border:1px solid #333;border-radius:4px;
    padding:2px 6px;color:#6f6f6f;text-transform:none;letter-spacing:0}
  .sl-bubble ul{list-style:none;margin:0;padding:0}
  .sl-bubble li{padding:3px 0 3px 18px;position:relative}
  .sl-bubble li:before{content:"–";position:absolute;left:2px;color:#6f6f6f}
  .sl-bubble li.sl-warn:before{content:"⚠";color:#d6a243;font-size:12px}
  .sl-bubble .sl-raw{font-family:Consolas,monospace;font-size:12px;color:#9a9a9a;background:#161616;
    border:1px solid #2a2a2a;border-radius:8px;padding:10px 12px;white-space:pre-wrap;overflow:auto;max-height:160px}
  .sl-bubble .sl-raw .h{color:#e6e6e6}
  .sl-bubble .sl-bf{display:flex;align-items:center;gap:10px;flex-wrap:wrap;
    padding:11px 16px;color:#6f6f6f;font-size:12px}
  .sl-bubble .sl-link{margin-left:auto;color:#9a9a9a;border:1px solid #333;border-radius:7px;
    padding:5px 11px;text-decoration:none}

  .sl-toast{position:fixed;right:16px;bottom:16px;z-index:99999;background:#1d1d1d;color:#e6e6e6;
    border:1px solid #525252;border-radius:10px;padding:12px 16px;max-width:340px;
    font:13px/1.45 "Segoe UI",system-ui,sans-serif;box-shadow:0 10px 30px rgba(0,0,0,.6)}
  .sl-toast b{color:#fff}
  .sl-toast .sl-anchor{font-size:15px;margin-right:6px}
  `;

  // ─────────────────────────────────────────────────── demo payloads
  // Deterministic per name so each container keeps a stable, varied look.
  const DEMOS = [
    {
      risk: "low", label: "LOW · Patch", cur: "1.41.3", next: "1.41.9",
      jump: "patch bump", source: "OCI label → GitHub Releases",
      summary: [["Stability fixes for the transcoder and library scan", false]],
      raw: "## 1.41.9\n- fix: transcoder memory leak (#1841)\n- fix: library scan stall on large folders",
    },
    {
      risk: "mid", label: "MEDIUM · 2× minor", cur: "v1.122.0", next: "v1.124.2",
      jump: "skips 2 releases (v1.123.0, v1.124.0)", source: "OCI label → GitHub Releases",
      summary: [
        ["New facial-recognition model — first start re-indexes the library", false],
        ["Breaking: requires PostgreSQL ≥ 15 with pgvecto-rs ≥ 0.3", true],
        ["Fixes: video-transcode leak, HEIC thumbnails, upload duplicates", false],
      ],
      raw: "## v1.124.2\n- fix(server): video transcode memory leak (#13412)\n## v1.124.0\n- feat(ml): new facial recognition model — requires re-index (#13201)\n- breaking(server): requires PostgreSQL >= 15 (#13176)",
    },
    {
      risk: "high", label: "HIGH · Major", cur: "29.0.4", next: "30.0.1",
      jump: "major version bump", source: "linuxserver.io changelog",
      summary: [
        ["Major release — review the upgrade notes before applying", true],
        ["Dropped support for PHP 8.1; container now ships PHP 8.3", false],
      ],
      raw: "## 30.0.0\n- breaking: minimum PHP 8.2, image ships 8.3\n- feat: new files UI\n## 30.0.1\n- fix: occ upgrade on large instances",
    },
    {
      risk: "grey", label: "UNKNOWN", cur: ":latest", next: "new digest",
      jump: "no semver — digest changed", source: "no changelog source resolved",
      summary: [["No changelog source could be resolved for this image", false]],
      raw: "(no changelog found — image sets no OCI source label and matches no known convention)",
    },
  ];

  // ──────────────────────────────────────────────────────── helpers
  function hash(s) {
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
    return Math.abs(h);
  }
  function demoFor(name) {
    return DEMOS[hash(name) % DEMOS.length];
  }
  function el(tag, cls, html) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (html != null) n.innerHTML = html;
    return n;
  }
  function esc(s) {
    return String(s).replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
  }

  // Find container rows in the Docker table. Returns {rows, selector} or null.
  // Tries known Unraid selectors, then falls back to a structural heuristic.
  function findRows() {
    const candidates = [
      "table#docker_list tbody tr",
      "table.tablesorter tbody tr",
      "div.tabs table tbody tr",
      "table tbody tr",
    ];
    for (const sel of candidates) {
      const all = Array.from(document.querySelectorAll(sel));
      // A real container row has an icon image and some text (skips header/spacer rows).
      const rows = all.filter((tr) => tr.querySelector("img") && tr.textContent.trim().length > 1);
      if (rows.length >= 1) return { rows, selector: sel };
    }
    return null;
  }

  // Extract a display name + the cell that holds the icon (where the badge goes).
  function rowInfo(tr) {
    const img = tr.querySelector("img");
    const cell = img ? img.closest("td") || tr : tr;
    // name: first non-empty anchor text, else the icon cell's text, else row text.
    let name = "";
    const a = cell.querySelector("a");
    if (a && a.textContent.trim()) name = a.textContent.trim();
    if (!name) name = (cell.textContent || tr.textContent).trim().split("\n")[0].trim();
    name = name.slice(0, 60) || "container";
    return { name, cell, img };
  }

  // ──────────────────────────────────────────────────────── bubble
  let openBubble = null;
  function closeBubble() {
    if (openBubble) { openBubble.remove(); openBubble = null; }
  }
  function openFor(badge, name, d) {
    closeBubble();
    const repoGuess = "github.com/example/" + name.toLowerCase().replace(/[^a-z0-9]+/g, "-");
    const lis = d.summary
      .map(([t, warn]) => `<li class="${warn ? "sl-warn" : ""}">${esc(t)}</li>`)
      .join("");
    const raw = esc(d.raw).replace(/^(##.*)$/gm, '<span class="h">$1</span>');
    const b = el("div", "sl-bubble");
    b.innerHTML = `
      <div class="sl-bh">
        <span class="sl-ver">${esc(d.cur)} → <b>${esc(d.next)}</b></span>
        <span class="sl-badge sl-${d.risk}"><span class="sl-dot"></span>${esc(d.label)}</span>
        <span class="sl-jump">${esc(d.jump)}</span>
        <span class="sl-x" title="close">✕</span>
      </div>
      <div class="sl-sec">
        <h4>⚡ Summary <span class="sl-ai">AI · Ollama (demo)</span></h4>
        <ul>${lis}</ul>
      </div>
      <div class="sl-sec">
        <h4>Changelog (raw excerpt)</h4>
        <div class="sl-raw">${raw}</div>
      </div>
      <div class="sl-bf">
        <span>Source: ${esc(d.source)}</span>
        <a class="sl-link" href="https://${repoGuess}" target="_blank" rel="noopener">Open on GitHub ↗</a>
      </div>`;
    document.body.appendChild(b);
    const r = badge.getBoundingClientRect();
    b.style.top = window.scrollY + r.bottom + 8 + "px";
    b.style.left = Math.max(8, window.scrollX + r.left) + "px";
    b.querySelector(".sl-x").addEventListener("click", (e) => { e.stopPropagation(); closeBubble(); });
    openBubble = b;
  }

  // ──────────────────────────────────────────────────────── inject
  function tagRows() {
    const found = findRows();
    if (!found) return null;
    let tagged = 0;
    for (const tr of found.rows) {
      if (tr.getAttribute(CONFIG.BADGE_ATTR)) continue;
      const { name, cell, img } = rowInfo(tr);
      const d = demoFor(name);
      const badge = el("span", `sl-badge sl-${d.risk}`,
        `<span class="sl-dot"></span>${esc(d.label.split(" ")[0])}`);
      badge.title = `ShipLog: ${d.label} — click for changelog`;
      badge.addEventListener("click", (e) => { e.preventDefault(); e.stopPropagation(); openFor(badge, name, d); });
      if (img && img.parentNode === cell) cell.insertBefore(badge, img);
      else cell.insertBefore(badge, cell.firstChild);
      tr.setAttribute(CONFIG.BADGE_ATTR, "1");
      tagged++;
    }
    return { tagged, total: found.rows.length, selector: found.selector };
  }

  function toast(html) {
    const t = el("div", "sl-toast", html);
    document.body.appendChild(t);
    setTimeout(() => t.remove(), 9000);
  }

  function diagnostics() {
    console.group(`${TAG} diagnostics — no container rows matched`);
    const tables = Array.from(document.querySelectorAll("table"));
    console.log(`tables on page: ${tables.length}`);
    tables.forEach((t, i) => {
      const body = t.tBodies[0];
      const rows = body ? body.rows.length : 0;
      console.log(`#${i}  id="${t.id}"  class="${t.className}"  bodyRows=${rows}`);
      if (body && body.rows[0]) {
        console.log(`     sample row HTML:`, body.rows[0].outerHTML.slice(0, 600));
      }
    });
    console.log("→ copy this whole block and paste it back so the real selectors can be locked in.");
    console.groupEnd();
  }

  // ──────────────────────────────────────────────────────── run
  function injectStyleOnce() {
    if (document.getElementById("sl-spike-style")) return;
    const s = el("style"); s.id = "sl-spike-style"; s.textContent = CSS;
    document.head.appendChild(s);
  }

  function run() {
    injectStyleOnce();
    const res = tagRows();
    if (!res || res.tagged === 0) {
      diagnostics();
      toast(`<span class="sl-anchor">⚓</span><b>ShipLog spike:</b> couldn't find container rows. Open the <b>Docker</b> tab, then check the console — paste the diagnostics back.`);
      return;
    }
    console.log(`${TAG} tagged ${res.tagged} container(s) via selector: ${res.selector}`);
    toast(`<span class="sl-anchor">⚓</span><b>ShipLog spike active.</b> Tagged <b>${res.tagged}</b> container(s). Click a badge for the bubble. <small style="color:#6f6f6f">(demo data — selector: <code>${res.selector}</code>)</small>`);
  }

  // close the bubble on outside click / Esc
  document.addEventListener("click", (e) => {
    if (openBubble && !openBubble.contains(e.target) && !e.target.classList.contains("sl-badge")
        && !e.target.closest(".sl-badge")) closeBubble();
  });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeBubble(); });

  // Unraid re-renders the Docker table periodically (auto-refresh) — re-tag new rows.
  const mo = new MutationObserver(() => { if (document.getElementById("sl-spike-style")) tagRows(); });
  try { mo.observe(document.body, { childList: true, subtree: true }); } catch (e) {}

  run();
})();
