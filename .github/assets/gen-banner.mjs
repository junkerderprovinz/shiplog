/**
 * Generates the ShipLog README banner:
 *   shiplog-banner.svg / .png : white 1600x500; the logo (embedded verbatim from
 *   logo.svg) on the left, "ShipLog" in Bree Serif + a claim to the right. Text
 *   is converted to SVG paths (opentype.js) so the SVG needs NO font and renders
 *   identically with resvg or a browser. Same brand font as BombVault.
 *
 * Deps (global): opentype.js, @resvg/resvg-js. The Bree Serif (OFL) font is
 * fetched at runtime to the OS temp dir — NOT committed.
 *
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
const LH = 420, LW = 420;     // logo is square (viewBox 0 0 1000 1000)
const nameSize = 168, claimSize = 44, gap = 56, lineGap = 20;

// Two themes so the banner adapts in GitHub (light/dark via <picture>): the logo
// recolours (its two greys → light) and the text + background flip.
const THEMES = [
  { suffix: "", bg: "#ffffff", name: "#242626", claim: "#5a5d5e", recolor: [] },
  {
    // Dark mode: the anchor body goes light so it reads on dark, but the shading
    // stays dark (cls-2) for depth — not light like the body.
    suffix: "-dark", bg: "#0d1117", name: "#e6edf3", claim: "#9aa4ad",
    recolor: [["#353738", "#e6edf3"], ["#242626", "#3a3e42"]],
  },
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

// The claim is set in Lato (humanist sans paired with Bree Serif) — the shared
// claim font across all Bree-Serif repos (BombVault, featherdrop, ShipLog).
const claimFontPath = join(tmpdir(), "ShipLog-Lato-Regular.ttf");
if (!existsSync(claimFontPath)) {
  const r = await fetch("https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf");
  if (!r.ok) throw new Error(`claim font fetch ${r.status}`);
  writeFileSync(claimFontPath, Buffer.from(await r.arrayBuffer()));
}
const claimFont = opentype.parse(readFileSync(claimFontPath));

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

// Embed the logo verbatim: drop the XML decl, position its root <svg>.
const logoRaw = readFileSync(join(__dir, "logo.svg"), "utf8")
  .replace(/<\?xml[^>]*\?>\s*/, "")
  .replace(
    /<svg\b[^>]*viewBox="0 0 1000 1000"[^>]*>/,
    `<svg x="${LX.toFixed(1)}" y="${LY.toFixed(1)}" width="${LW}" height="${LH}" viewBox="0 0 1000 1000" xmlns="http://www.w3.org/2000/svg">`,
  );

for (const t of THEMES) {
  let logo = logoRaw;
  for (const [from, to] of t.recolor) logo = logo.split(from).join(to);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <rect width="${W}" height="${H}" fill="${t.bg}"/>
  ${logo}
  <path d="${namePath}" fill="${t.name}"/>
  <path d="${claimPath}" fill="${t.claim}"/>
</svg>
`;
  writeFileSync(join(__dir, `shiplog-banner${t.suffix}.svg`), svg);
  const png = new Resvg(svg, { background: t.bg, fitTo: { mode: "width", value: W } }).render().asPng();
  writeFileSync(join(__dir, `shiplog-banner${t.suffix}.png`), png);
  console.log(`wrote shiplog-banner${t.suffix}.svg + .png`);
}
