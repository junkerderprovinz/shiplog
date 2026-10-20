/**
 * Renders the CA icon (icon.png) from logo.svg: the bare logo on a transparent square.
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

const logo = readFileSync(join(__dir, "logo.svg"), "utf8");
const png = new Resvg(logo, { fitTo: { mode: "width", value: 512 } }).render().asPng();
writeFileSync(join(__dir, "icon.png"), png);
console.log("wrote icon.png");
