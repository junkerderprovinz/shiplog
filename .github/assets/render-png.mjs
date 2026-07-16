/* Render an SVG to a transparent PNG at a given square-ish size, viewBox-agnostic.
 * Usage: node render-png.mjs <in.svg> <out.png> <width>
 * Uses global @resvg/resvg-js (same resolution as gen-banner.mjs). */
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(import.meta.url);
const groot = execSync("npm root -g").toString().trim();
const { Resvg } = require(`${groot}/@resvg/resvg-js`);
const [, , inp, outp, wArg] = process.argv;
const svg = readFileSync(inp, "utf8");
const width = parseInt(wArg || "512", 10);
const png = new Resvg(svg, { fitTo: { mode: "width", value: width }, background: "rgba(0,0,0,0)" }).render().asPng();
writeFileSync(outp, png);
console.log(`rendered ${inp} -> ${outp} @ ${width}px (${png.length} bytes)`);
