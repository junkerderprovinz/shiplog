// Renders the README screenshots in the style of the house's store pictures: a capture on the dark
// relief ground with the logo pressed into it, a heading and a line of text beside it. A page sits
// in a browser window, a window of ShipLog's own stands on its own. 1920x1000 each.
//
// The pictures come from captures/, taken by capture.mjs at twice the pixel density. Everything
// here is rendered at twice the size and scaled down, which keeps the type crisp through the tilt.
//
// The heading is set in Bree Serif Light and the line below in Bree Light. Both are licensed and
// never committed, so HOUSE_FONTS (default %LOCALAPPDATA%\halleluja-fonts) must hold them as TTF.
// The browser bar keeps Lato, fetched once from Google Fonts.
//
// Deps (global): playwright-core with its Chromium installed.
// Run: node .github/assets/screenshots/render.mjs [capture...]   (no capture renders them all)

import { execSync } from "node:child_process";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { chromium } = require(`${execSync("npm root -g").toString().trim()}/playwright-core`);

const here = dirname(fileURLToPath(import.meta.url));

const SHOTS = [
  {
    file: "shiplog-1.png",
    capture: "changelog.png",
    caption: "Know what an <em>update brings</em>",
    sub: "One click on a container in Unraid's Docker tab opens what changed",
  },
  {
    file: "shiplog-2.png",
    capture: "docker.png",
    url: "nas.local/Docker",
    tab: "Tower/Docker",
    caption: "Right where you <em>click update</em>",
    sub: "A changelog chip on every container, with the risk and the known vulnerabilities beside it",
  },
  {
    file: "shiplog-3.png",
    capture: "vulns.png",
    caption: "See the holes <em>it closes</em>",
    sub: "The known critical and high vulnerabilities of the running image, and which ones the newest fixes",
  },
  {
    file: "shiplog-4.png",
    capture: "status.png",
    url: "nas.local:8484",
    tab: "ShipLog",
    caption: "Every container, <em>graded</em>",
    sub: "The status page shows the risk, the known vulnerabilities and where the notes come from",
  },
  {
    file: "shiplog-5.png",
    capture: "picker.png",
    caption: "Messages <em>where you read</em>",
    sub: "Unraid's own notifications, a chat, a push service, mail or a webhook, each picked from a tile",
  },
];

const CHOSEN = process.argv.slice(2);
const W = 1920;
const H = 1000;
// Where the capture stands: right of the text, with room for the tilt.
const AREA = { left: 620, right: 1860, top: 90, bottom: 930 };

async function cached(file, url) {
  const path = join(tmpdir(), `shiplog-${file}`);
  if (!existsSync(path)) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${file}: fetch ${res.status}`);
    writeFileSync(path, Buffer.from(await res.arrayBuffer()));
  }
  return readFileSync(path);
}

const dataUrl = (buf, type) => `data:${type};base64,${buf.toString("base64")}`;
// The house typeface is licensed, so it is read from a local folder and never committed.
const fonts = process.env.HOUSE_FONTS ?? join(process.env.LOCALAPPDATA ?? tmpdir(), "halleluja-fonts");
const font = (file) => dataUrl(readFileSync(join(fonts, file)), "font/ttf");
const lato = dataUrl(await cached("Lato-Regular.ttf", "https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf"), "font/ttf");
const logo = dataUrl(readFileSync(join(here, "..", "logo.svg")), "image/svg+xml");

const pngSize = (buf) => ({ width: buf.readUInt32BE(16) / 2, height: buf.readUInt32BE(20) / 2 });

const STYLE = `
@font-face { font-family: "Bree Serif"; src: url(${font("bree-serif-light.ttf")}); font-weight: 300; }
@font-face { font-family: Bree; src: url(${font("bree-light.ttf")}); font-weight: 300; }
@font-face { font-family: Lato; src: url(${lato}); }
* { box-sizing: border-box; margin: 0; }
body { position: relative; width: ${W}px; height: ${H}px; overflow: hidden; font-family: Bree, sans-serif; font-weight: 300; background: #0c0c0b; }
.wall { position: absolute; inset: 0; background: radial-gradient(55% 60% at 62% 35%, #26231d, #0f0e0c 72%); }
.relief { position: absolute; width: 56%; left: -12%; top: 4%; transform: rotate(-10deg); opacity: .32;
  filter: grayscale(1) brightness(.36) contrast(1.2) drop-shadow(-2px -2px 0 rgba(255,255,255,.16)) drop-shadow(12px 18px 26px rgba(0,0,0,.85)); }
.vignette { position: absolute; inset: 0; box-shadow: inset 0 0 200px rgba(0,0,0,.6); }
.copy { position: absolute; left: 84px; top: 0; bottom: 0; width: 470px; display: flex; flex-direction: column; justify-content: center; gap: 28px; }
.copy img { width: 132px; }
h1 { font: 300 64px/1.12 "Bree Serif", serif; color: #f4f4f4; }
h1 em { font-style: normal; color: #FCC419; }
.sub { font-size: 28px; line-height: 1.35; color: #9d9481; }
.stage { position: absolute; perspective: 2400px; }
.floor { position: absolute; left: 12%; right: 12%; bottom: -4%; height: 8%; border-radius: 50%; background: rgba(0,0,0,.8); filter: blur(30px); }
.win { position: relative; transform: rotateY(-8deg) rotateX(2deg); border-radius: 14px; overflow: hidden; background: #161616;
  box-shadow: 0 0 0 1px #3c3c3c, 0 2px 4px rgba(0,0,0,.35), 0 18px 36px rgba(0,0,0,.45), 0 52px 100px rgba(0,0,0,.55); }
.win > img { display: block; width: 100%; }
.own { position: relative; transform: rotateY(-8deg) rotateX(2deg); }
.own > img { display: block; width: 100%; border-radius: 14px; box-shadow: 0 0 0 1px #3c3c3c, 0 18px 36px rgba(0,0,0,.45), 0 52px 100px rgba(0,0,0,.55); }
.own.cut > img { border-radius: 0; box-shadow: none; filter: drop-shadow(0 18px 30px rgba(0,0,0,.5)) drop-shadow(0 40px 60px rgba(0,0,0,.45)); }
.bar { position: relative; display: flex; align-items: center; color: #d6d6d6; font: 400 17px/1 Lato, sans-serif; }
.tabs { height: 42px; padding: 7px 0 0 12px; background: #1c1c1c; align-items: flex-end; }
.tab { display: flex; align-items: center; gap: 10px; height: 35px; padding: 0 70px 0 14px; border-radius: 10px 10px 0 0; background: #2c2c2c; font-size: 15px; }
.tab img { width: 20px; height: 20px; object-fit: contain; }
.address { height: 46px; gap: 14px; padding: 0 14px; background: #2c2c2c; border-bottom: 1px solid #111; }
.nav { width: 10px; height: 10px; border-left: 2px solid #9a9a9a; border-bottom: 2px solid #9a9a9a; transform: rotate(45deg); margin: 0 4px; }
.nav.fwd { transform: rotate(-135deg); border-color: #5a5a5a; }
.url { flex: 1; height: 32px; border-radius: 16px; background: #1a1a1a; padding-left: 18px; display: flex; align-items: center; font-size: 15px; color: #c9c9c9; }
.glare { position: absolute; inset: 0; pointer-events: none; border-radius: 14px;
  background: linear-gradient(118deg, rgba(255,255,255,.09) 0%, rgba(255,255,255,.03) 28%, rgba(255,255,255,0) 42%); }
`;

// The capture's box: as large as the area allows, centred in it.
function place(shot, size) {
  const chrome = shot.url ? 88 : 0;
  const scale = Math.min((AREA.right - AREA.left) / size.width, (AREA.bottom - AREA.top - chrome) / size.height);
  const width = Math.round(size.width * scale);
  const height = Math.round(size.height * scale) + chrome;
  return { width, left: AREA.left + Math.round((AREA.right - AREA.left - width) / 2), top: AREA.top + Math.round((AREA.bottom - AREA.top - height) / 2) };
}

function page(shot) {
  const buf = readFileSync(join(here, "captures", shot.capture));
  const box = place(shot, pngSize(buf));
  const img = `<img src="${dataUrl(buf, "image/png")}">`;
  const body = shot.url
    ? `<div class="win">
  <div class="bar tabs"><div class="tab">${shot.tab === "ShipLog" ? `<img src="${logo}">` : ""}<span>${shot.tab}</span></div></div>
  <div class="bar address"><i class="nav"></i><i class="nav fwd"></i><div class="url">${shot.url}</div></div>
  ${img}<div class="glare"></div></div>`
    : `<div class="own${shot.capture === "picker.png" ? " cut" : ""}">${img}</div>`;
  return `<!doctype html><html><head><meta charset="utf-8"><style>${STYLE}</style></head><body>
<div class="wall"></div><img class="relief" src="${logo}"><div class="vignette"></div>
<div class="copy"><img src="${logo}"><h1>${shot.caption}</h1><p class="sub">${shot.sub}</p></div>
<div class="stage" style="left:${box.left}px;top:${box.top}px;width:${box.width}px"><div class="floor"></div>${body}</div>
</body></html>`;
}

const browser = await chromium.launch();
try {
  for (const shot of SHOTS.filter((s) => !CHOSEN.length || CHOSEN.includes(s.capture.replace(".png", "")))) {
    const big = await browser.newPage({ viewport: { width: W, height: H }, deviceScaleFactor: 2 });
    await big.setContent(page(shot));
    await big.evaluate(() => Promise.all(['300 1em "Bree Serif"', "300 1em Bree", "1em Lato"].map((f) => document.fonts.load(f))));
    await big.evaluate(() => Promise.all([...document.images].map((img) => img.decode())));
    const png = await big.screenshot();
    await big.close();
    const small = await browser.newPage({ viewport: { width: W, height: H } });
    await small.setContent(`<body style="margin:0"><img src="${dataUrl(png, "image/png")}" style="display:block;width:${W}px;height:${H}px">`);
    await small.locator("img").evaluate((img) => img.decode());
    await small.screenshot({ path: join(here, shot.file) });
    await small.close();
    console.log(`${shot.file} written`);
  }
} finally {
  await browser.close();
}
