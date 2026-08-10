/**
 * Generates the CA / app icon from logo.svg (the shiplog-dunkel variant):
 *   icon.svg / icon.png : the gold-anchor-in-dark-ring logo on a WHITE background,
 *   so the light tile stands out on Unraid's dark CA page and the dark ring + gold
 *   anchor read on the white tile.
 *
 * Run: node .github/assets/gen-icon.mjs
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const { Resvg } = require(`${execSync("npm root -g").toString().trim()}/@resvg/resvg-js`);
const __dir = dirname(fileURLToPath(import.meta.url));

// Put a white background rect behind the logo (dark ring + gold anchor), then
// flood-fill the border-connected white back to transparent. The ring is a
// hollow path (no separate inner-disk fill), so a flat rect alone would leave
// the 4 corners OUTSIDE the ring opaque white too (square-looking tile). The
// flood fill stops at the ring's dark border, so only the disk it encloses
// stays white (jdp, 2026-08-10: "nur innerhalb des schwarzen Rahmens einen
// weißen Hintergrund").
// viewBox-agnostic: size the rect from the logo's own viewBox (handles 960/1000/…).
const logo = readFileSync(join(__dir, "logo.svg"), "utf8");
const vb = (logo.match(/viewBox="0 0 ([\d.]+) ([\d.]+)"/) || [, "1000", "1000"]);
const iconSvg = logo.replace(
  /(<svg\b[^>]*>)/,
  `$1<rect width="${vb[1]}" height="${vb[2]}" fill="#ffffff"/>`,
);

writeFileSync(join(__dir, "icon.svg"), iconSvg);
const png = new Resvg(iconSvg, { fitTo: { mode: "width", value: 512 } }).render().asPng();
const iconPngPath = join(__dir, "icon.png");
writeFileSync(iconPngPath, png);
execSync(`python3 "${join(__dir, "flood-transparent.py")}" "${iconPngPath}"`);
console.log("wrote icon.svg + icon.png (gold anchor + dark ring, white only inside the ring)");
