// Takes the raw pictures for the README screenshots: the Docker tab with ShipLog's chips, the
// changelog window, the channel window on the settings page and the engine's status page.
//
// The containers are a sandbox, never the server's own. In a Docker-in-Docker container, older
// releases of public images run under a moving tag such as latest, so the engine sees a real
// update, and a standalone engine checks them and serves its status page on SANDBOX_URL. The
// Unraid server only lends its Docker tab and settings page: each sandbox container has an alpine
// stand-in there, named sl-shot-<name> and run with --network none, and every answer of ShipLog's
// proxy is swapped for the sandbox engine's. The server is only read. Every request other than a
// GET is dropped after the login, and the rows of the server's own containers are taken out
// before a picture.
//
// With SHOT_FIXTURE=<file>, the Docker tab and the changelog window show the made-up containers of
// that file instead, and only the status page needs the sandbox. Their rows take the form Unraid
// sends its list in, so the server needs no stand-ins either.
//
// ShipLog's own text is always English. Unraid prints its text in the server's language, so its
// display language has to be English while the pictures are taken, or SHOT_TEXT names a JSON file
// that maps each of its words on the page to the English one.
//
// SHOT_DENY takes comma-separated words, such as the server's name, that must not show on a page.
// Hits are printed and the picture is still taken, so it can be checked.
//
// Deps (global): playwright-core with its Chromium installed.
// Run: UNRAID_URL=https://<server> UNRAID_PW=<root password> SANDBOX_URL=http://<engine>:8484 \
//      node .github/assets/screenshots/capture.mjs [shot...]
//  or: UNRAID_URL=... UNRAID_PW=... SHOT_FIXTURE=.github/assets/screenshots/fixture.json \
//      node .github/assets/screenshots/capture.mjs docker changelog

import { execSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { chromium } = require(`${execSync("npm root -g").toString().trim()}/playwright-core`);

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, "captures");
const BASE = (process.env.UNRAID_URL || "").replace(/\/+$/, "");
const SANDBOX = (process.env.SANDBOX_URL || "").replace(/\/+$/, "");
const PASSWORD = process.env.UNRAID_PW;
const FIXTURE = process.env.SHOT_FIXTURE ? JSON.parse(readFileSync(process.env.SHOT_FIXTURE, "utf8")) : null;
const TEXT = process.env.SHOT_TEXT ? JSON.parse(readFileSync(process.env.SHOT_TEXT, "utf8")) : {};
const CHOSEN = process.argv.slice(2);
if (!BASE || !PASSWORD) throw new Error("set UNRAID_URL and UNRAID_PW");
if (!SANDBOX && (!FIXTURE || !CHOSEN.length || CHOSEN.includes("status"))) throw new Error("set SANDBOX_URL, or SHOT_FIXTURE without the status shot");

const LANG = join(here, "../../../plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/lang/en.json");
const I18N = Object.fromEntries(Object.entries(JSON.parse(readFileSync(LANG, "utf8"))).filter(([k]) => k.startsWith("d_")));

const PREFIX = "sl-shot-";
const VIEW = { width: 1440, height: 820 };
const PROXY = /\/plugins\/shiplog\/server\/status\.php(\?|$)/;

// The channels the settings page shows in place of the server's own.
const CHANNELS = [
  { id: "unraid", kind: "unraid", enabled: true, on: "always" },
  { id: "discord-1", kind: "discord", name: "Homelab", enabled: true, on: "always", discord: { url: "********" } },
  { id: "ntfy-1", kind: "ntfy", name: "Phone", enabled: true, on: "failure", ntfy: { url: "https://ntfy.sh/homelab-updates", token: "********" } },
  { id: "smtp-1", kind: "smtp", name: "Mail", enabled: false, on: "failure",
    smtp: { host: "mail.example.org", tls: "starttls", port: 587, username: "unraid", password: "********", from: "unraid@example.org", to: "me@example.org" } },
];

const still = "*, *::before, *::after { transition: none !important; animation: none !important; caret-color: transparent !important; }";

// The sandbox engine's answer to a proxy request, its containers renamed to their stand-ins.
async function sandbox(url) {
  const q = new URL(url).searchParams;
  const path = q.get("path");
  if (path === "notify") return { channels: CHANNELS };
  if (path === "notify-hub") return { hub: false };
  if (FIXTURE) return path === "guards" ? FIXTURE.guards : FIXTURE.status;
  const res = await fetch(`${SANDBOX}${path === "guards" ? "/api/guards" : "/api/containers"}`);
  const data = await res.json();
  if (path) return data;
  for (const st of data) st.container.name = PREFIX + st.container.name;
  return data;
}

// The fixture's containers as Unraid's DockerContainers.php sends them: the rows, the script that
// fills its docker[] array and the busy flag, split by NUL.
function dockerList() {
  const rows = [], pushes = [];
  const empty = "<td style='white-space:nowrap'><span class='docker_readmore'></span></td>";
  FIXTURE.status.forEach(({ container: { name } }, i) => {
    const id = `5e1f00c0ffe${i}`;
    rows.push(`<tr class='sortable'><td class='ct-name' style='width:220px;padding:8px'><i class='fa fa-arrows-v mover orange-text'></i>` +
      `<span class='outer'><span id='${id}' class='hand'><img src='${FIXTURE.icons[name]}' class='img'></span>` +
      `<span class='inner'><span class='appname '>${name}</span><br><i id='load-${id}' class='fa fa-play started green-text'></i>` +
      `<span class='state'>started</span></span></span></td>` +
      `<td class='updatecolumn'><div><span><i class='fa fa-docker fa-fw'></i> 3rd Party</span></div></td>` +
      `<td style='white-space:nowrap'><span class='docker_readmore'>none</span></td>${empty.repeat(4)}<td class='advanced'></td>` +
      `<td><i class='fa fa-docker fa-fw'></i> 3rd Party</td>` +
      `<td><div style='white-space:nowrap'>Uptime: 12 minutes<div style='margin-top:4px'>Created: 12 minutes ago</div></div></td></tr>`);
    pushes.push(`docker.push({name:'${name}',id:'${id}',state:1,pause:0,update:3});`);
  });
  return [rows.join(""), pushes.join(""), "0"].join("\0");
}

async function login(page) {
  await page.goto(`${BASE}/login`, { timeout: 120000 });
  await page.fill("input[name=username]", "root");
  await page.fill("input[name=password]", PASSWORD);
  await Promise.all([
    page.waitForURL((u) => !/login/.test(String(u)), { waitUntil: "commit", timeout: 60000 }),
    page.click("button[type=submit], input[type=submit]"),
  ]);
}

// Runs in the page: keeps the stand-ins' rows in the order given and drops the prefix from
// everything a reader sees.
function curate({ prefix, order, text }) {
  const list = document.querySelector("#docker_list");
  if (list) {
    const nameOf = (tr) => (tr.querySelector("td.ct-name .appname")?.textContent || "").trim().replace(prefix, "");
    for (const tr of [...list.children]) if (!order.includes(nameOf(tr))) tr.remove();
    const rank = (tr) => order.indexOf(nameOf(tr));
    [...list.children].sort((a, b) => rank(a) - rank(b)).forEach((tr) => list.appendChild(tr));
  }
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
  for (let n = walker.currentNode; n; n = walker.nextNode()) {
    if (n.nodeType === Node.TEXT_NODE) {
      if (n.nodeValue.includes(prefix)) n.nodeValue = n.nodeValue.split(prefix).join("");
      const t = n.nodeValue.trim();
      if (text[t]) n.nodeValue = n.nodeValue.replace(t, text[t]);
      continue;
    }
    if (n.matches("input[type=button]") && text[n.value]) n.value = text[n.value];
    for (const at of ["title", "aria-label", "data-tip", "alt"]) {
      const v = n.getAttribute(at);
      if (v && v.includes(prefix)) n.setAttribute(at, v.split(prefix).join(""));
    }
  }
}

// Runs in the page: every visible text with a denied word or a private address.
function leaks(deny) {
  const hits = [];
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const box = n.parentElement?.getBoundingClientRect();
    if (!box || !box.width || !box.height || box.bottom < 0 || box.top > innerHeight) continue;
    const t = n.nodeValue;
    for (const w of deny) if (t.toLowerCase().includes(w.toLowerCase())) hits.push(`${w}: ${t.trim().slice(0, 80)}`);
    if (/\b(10|172|192)\.\d{1,3}\.\d{1,3}\.\d{1,3}\b/.test(t)) hits.push(`address: ${t.trim().slice(0, 80)}`);
  }
  return [...new Set(hits)];
}

const ORDER = ["radarr", "uptime-kuma", "portainer", "traefik", "homepage", "prowlarr"];
const DENY = [PREFIX, ...(process.env.SHOT_DENY || "").split(",").map((w) => w.trim()).filter(Boolean)];

mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
try {
  const ctx = await browser.newContext({ ignoreHTTPSErrors: true, viewport: VIEW, deviceScaleFactor: 2, locale: "en-US", colorScheme: "dark" });
  await ctx.addInitScript((i18n) => Object.defineProperty(window, "shiplogI18n", { get: () => i18n, set() {} }), I18N);
  const page = await ctx.newPage();
  await page.route(/./, (r) => (r.request().method() === "GET" || /\/login/.test(r.request().url()) ? r.continue() : r.abort()));
  await login(page);
  await page.unrouteAll({ behavior: "ignoreErrors" });
  await page.route(/./, async (route) => {
    const req = route.request();
    const url = req.url();
    // Another skin of the Docker tab belongs in its own README.
    if (/\/plugins\/towertorch\//.test(url)) return route.abort();
    if (req.method() !== "GET") return route.abort();
    if (PROXY.test(url)) return route.fulfill({ contentType: "application/json", body: JSON.stringify(await sandbox(url)) });
    if (FIXTURE && /\/DockerContainers\.php(\?|$)/.test(url)) return route.fulfill({ contentType: "text/html", body: dockerList() });
    return route.continue();
  });

  const shot = async (name, target) => {
    await page.addStyleTag({ content: still });
    await page.evaluate(curate, { prefix: PREFIX, order: ORDER, text: TEXT });
    await page.waitForTimeout(300);
    for (const h of await page.evaluate(leaks, DENY)) console.log(`  ${name}: ${h}`);
    await target.screenshot({ path: join(out, `${name}.png`) });
    console.log(`wrote ${name}.png`);
  };

  if (!CHOSEN.length || CHOSEN.includes("docker") || CHOSEN.includes("changelog")) {
    await page.goto(`${BASE}/Docker`, { waitUntil: "load", timeout: 60000 });
    await page.waitForSelector(`#docker_list .sl-chip`, { timeout: 60000 });
    await page.waitForTimeout(3000);
    // Unraid redraws the list on a timer, which would bring the other rows back.
    await page.evaluate(() => {
      const last = setTimeout(() => {}, 0);
      for (let i = 0; i <= last; i++) { clearTimeout(i); clearInterval(i); }
    });
    await page.mouse.move(VIEW.width / 2, 2);
    await page.evaluate(curate, { prefix: PREFIX, order: ORDER, text: TEXT });
    await page.waitForTimeout(1000);
    if (!CHOSEN.length || CHOSEN.includes("docker")) {
      // The stand-ins have no ports, addresses or volumes, so those columns would only stand empty.
      const empty = await page.addStyleTag({ content: "#docker_containers tr > :nth-child(n+4):nth-child(-n+7) { display: none; }" });
      await page.setViewportSize({ width: 1100, height: VIEW.height });
      await page.waitForTimeout(500);
      // From the tab's title to the buttons under the list, below the server's header.
      const top = await page.locator("#docker_containers").evaluate((t) => t.getBoundingClientRect().top - 80);
      const bottom = await page.locator("#docker_containers").evaluate((t) => t.getBoundingClientRect().bottom + 90);
      await shot("docker", { screenshot: (o) => page.screenshot({ ...o, clip: { x: 0, y: top, width: 1100, height: bottom - top } }) });
      await empty.evaluate((el) => el.remove());
      await page.setViewportSize(VIEW);
    }
    if (!CHOSEN.length || CHOSEN.includes("changelog")) {
      const row = page.locator("#docker_list > tr").filter({ has: page.locator(".appname", { hasText: /^\s*traefik\s*$/ }) });
      await row.locator(".sl-chip").first().click();
      await page.waitForSelector(".sl-bubble", { timeout: 30000 });
      await page.waitForTimeout(1500);
      await page.mouse.move(2, VIEW.height - 2);
      await shot("changelog", page.locator(".sl-bubble"));
      await page.evaluate(() => {
        const d = document.querySelector(".sl-bubble details.sl-vulns");
        d.open = true;
        let box = d.parentElement;
        while (box && box.scrollHeight <= box.clientHeight) box = box.parentElement;
        box.scrollTop += d.getBoundingClientRect().top - box.getBoundingClientRect().top - 90;
      });
      await page.waitForTimeout(500);
      await shot("vulns", page.locator(".sl-bubble"));
      await page.keyboard.press("Escape");
    }
  }

  if (!CHOSEN.length || CHOSEN.includes("channels")) {
    await page.goto(`${BASE}/Settings/shiplog`, { waitUntil: "load", timeout: 60000 });
    await page.click("#sl-tab-notify");
    await page.waitForSelector("#sl-notify .glim-nc-switch", { timeout: 30000 });
    await page.waitForTimeout(1000);
    await page.locator("#sl-notify .glim-nc-add").first().click();
    await page.waitForSelector(".glim-nc-tile", { timeout: 15000 });
    await page.waitForTimeout(800);
    await page.mouse.move(2, VIEW.height - 2);
    // Only the window, on a transparent ground, so the badge above its edge stays whole.
    await page.addStyleTag({ content: `html, body { background: transparent !important; }
      body * { visibility: hidden !important; } .glim-nc-win, .glim-nc-win * { visibility: visible !important; }` });
    const box = await page.locator(".glim-nc-win[role=dialog]").boundingBox();
    await shot("picker", { screenshot: (o) => page.screenshot({ ...o, omitBackground: true,
      clip: { x: box.x - 4, y: box.y - 28, width: box.width + 8, height: box.height + 32 } }) });
  }

  if (!CHOSEN.length || CHOSEN.includes("status")) {
    const status = await ctx.newPage();
    await status.setViewportSize({ width: VIEW.width, height: 700 });
    await status.goto(`${SANDBOX}/`, { waitUntil: "load" });
    await status.waitForTimeout(2000);
    await status.addStyleTag({ content: still });
    await status.screenshot({ path: join(out, "status.png") });
    console.log("wrote status.png");
  }
} finally {
  await browser.close();
}
