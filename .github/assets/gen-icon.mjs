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

// Put a white background rect behind the logo (dark ring + gold anchor).
const logo = readFileSync(join(__dir, "logo.svg"), "utf8");
const iconSvg = logo.replace(
  /(<svg\b[^>]*viewBox="0 0 1000 1000"[^>]*>)/,
  `$1<rect width="1000" height="1000" fill="#ffffff"/>`,
);

writeFileSync(join(__dir, "icon.svg"), iconSvg);
const png = new Resvg(iconSvg, { fitTo: { mode: "width", value: 512 } }).render().asPng();
writeFileSync(join(__dir, "icon.png"), png);
console.log("wrote icon.svg + icon.png (gold anchor + dark ring on white)");
