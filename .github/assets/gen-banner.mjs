/**
 * Generates the ShipLog banners (white 1600x500):
 *
 *   shiplog-banner.svg/.png       light, logo + "ShipLog" + claim   (README, light)
 *   shiplog-banner-dark.svg/.png  dark,  logo + "ShipLog" + claim   (README, GitHub dark)
 *   shiplog-banner-logo.svg/.png  light, logo only, NO text         (support thread)
 *
 * The text-free "-banner-logo" variant is ALWAYS generated alongside the README
 * banner: the Unraid support thread wants a banner completely without text (house
 * rule). It uses the house-standard "-banner-logo" name shared across all repos.
 *
 * No recolour hacks: each theme embeds the matching logo variant verbatim —
 *   light bg -> shiplog-dunkel.svg (dark ring, reads on white)
 *   dark  bg -> shiplog-hell.svg   (white ring, reads on dark)
 * The gold anchor is the constant core in both.
 *
 * Text is converted to SVG paths (opentype.js) so the SVG needs NO font and renders
 * identically with resvg or a browser. Bree Serif (name) + Lato (claim), the shared
 * brand fonts across the Bree-Serif repos (BombVault, featherdrop, ShipLog).
 *
 * Deps (global): opentype.js, @resvg/resvg-js. Fonts are fetched to the OS temp dir.
 * Run: node .github/assets/gen-banner.mjs
 */
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const groot = execSync("npm root -g").toString().trim();
const opentype = require(`${groot}/opentype.js`);
const { Resvg } = require(`${groot}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));

// ---- content + styling -----------------------------------------------------
const NAME = "ShipLog";
const CLAIM = "Read the changelog before you set sail.";
const W = 1600, H = 500;
const LH = 420, LW = 420;     // logo is ~square; embedLogo reads each master's own viewBox (960 or 1000)
const nameSize = 168, claimSize = 44, gap = 56, lineGap = 20;

// Each theme embeds the logo variant that reads on its background (no recolour).
const THEMES = [
  { suffix: "", bg: "#ffffff", name: "#242626", claim: "#5a5d5e", logo: "shiplog-dunkel.svg" },
  { suffix: "-dark", bg: "#0d1117", name: "#e6edf3", claim: "#9aa4ad", logo: "shiplog-hell.svg" },
];
// ---------------------------------------------------------------------------

const fontPath = join(tmpdir(), "ShipLog-BreeSerif-Regular.ttf");
if (!existsSync(fontPath)) {
  const url = "https://github.com/google/fonts/raw/main/ofl/breeserif/BreeSerif-Regular.ttf";
  const res = await fetch(url);
  if (!res.ok) throw new Error(`font fetch ${res.status}`);
  writeFileSync(fontPath, Buffer.from(await res.arrayBuffer()));
}
const font = opentype.parse(readFileSync(fontPath));

// The claim is set in Lato (humanist sans paired with Bree Serif).
const claimFontPath = join(tmpdir(), "ShipLog-Lato-Regular.ttf");
if (!existsSync(claimFontPath)) {
  const r = await fetch("https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf");
  if (!r.ok) throw new Error(`claim font fetch ${r.status}`);
  writeFileSync(claimFontPath, Buffer.from(await r.arrayBuffer()));
}
const claimFont = opentype.parse(readFileSync(claimFontPath));

// Text layout (logo + name + claim, group centred horizontally).
const nameW = font.getAdvanceWidth(NAME, nameSize);
const claimW = claimFont.getAdvanceWidth(CLAIM, claimSize);
const groupW = LW + gap + Math.max(nameW, claimW);
const startX = (W - groupW) / 2;
const LX = startX, LY = (H - LH) / 2;
const textX = startX + LW + gap;

const sc = (s) => s / font.unitsPerEm;
const nameAsc = font.ascender * sc(nameSize);
const nameDesc = -font.descender * sc(nameSize);
const claimAsc = claimFont.ascender * (claimSize / claimFont.unitsPerEm);
const blockH = nameAsc + nameDesc + lineGap + claimAsc;
const nameBaseline = H / 2 - blockH / 2 + nameAsc;
const claimBaseline = nameBaseline + nameDesc + lineGap + claimAsc;

const namePath = font.getPath(NAME, textX, nameBaseline, nameSize).toPathData(2);
const claimPath = claimFont.getPath(CLAIM, textX, claimBaseline, claimSize).toPathData(2);

// Embed a logo variant verbatim at (x,y,w,h): drop the XML decl, reposition its <svg>.
// viewBox-agnostic — reads the file's own viewBox and preserves it (handles 960/1000/…).
function embedLogo(logoFile, x, y, w, h) {
  const raw = readFileSync(join(__dir, logoFile), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
  const vb = (raw.match(/viewBox="([^"]+)"/) || [, "0 0 1000 1000"])[1];
  return raw.replace(
    /<svg\b[^>]*>/,
    `<svg x="${x.toFixed(1)}" y="${y.toFixed(1)}" width="${w}" height="${h}" viewBox="${vb}" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">`,
  );
}

function emit(name, svg, bg) {
  writeFileSync(join(__dir, `${name}.svg`), svg);
  const png = new Resvg(svg, { background: bg, fitTo: { mode: "width", value: W } }).render().asPng();
  writeFileSync(join(__dir, `${name}.png`), png);
  console.log(`wrote ${name}.svg + .png`);
}

// README banner (both themes): logo (left) + name + claim.
for (const t of THEMES) {
  const full = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <rect width="${W}" height="${H}" fill="${t.bg}"/>
  ${embedLogo(t.logo, LX, LY, LW, LH)}
  <path d="${namePath}" fill="${t.name}"/>
  <path d="${claimPath}" fill="${t.claim}"/>
</svg>
`;
  emit(`shiplog-banner${t.suffix}`, full, t.bg);
}

// Support-thread banner: logo only, NO text — ALWAYS generated (house rule).
// Light surface (the Unraid forum) -> the dunkel logo centred on white.
const logoLX = (W - LW) / 2, logoLY = (H - LH) / 2;
const lt = THEMES[0];
const logoOnly = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <rect width="${W}" height="${H}" fill="${lt.bg}"/>
  ${embedLogo(lt.logo, logoLX, logoLY, LW, LH)}
</svg>
`;
emit("shiplog-banner-logo", logoOnly, lt.bg);
