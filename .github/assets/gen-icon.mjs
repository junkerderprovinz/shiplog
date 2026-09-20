/**
 * Generates the CA and app icon (icon.svg, icon.png) from logo.svg, with white
 * inside the dark ring so the icon stands out on Unraid's dark CA page.
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

// The ring is a hollow path, so a white rect alone would fill the corners
// outside it too. The flood fill afterwards clears everything up to the ring.
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
console.log("wrote icon.svg and icon.png (white only inside the ring)");
