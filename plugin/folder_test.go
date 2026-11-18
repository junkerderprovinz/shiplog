package plugin

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// folderRun loads docker.js in node with a small Docker tab of its own: rows
// are tagged, then rebuilt the way a folder plugin does it, keeping the mark
// on the update cell but not the chip, and the observer runs again. It
// prints, as JSON, the chips in each cell after every pass.
const folderRun = `
class El {
  constructor(tag) { this.tagName = tag.toUpperCase(); this.children = []; this.parentNode = null; this.attrs = {}; this.text = ""; this.style = {}; this.dataset = {}; this.listeners = 0; }
  get classList() { const el = this; return {
    contains: (c) => (el.attrs.class || "").split(/\s+/).includes(c),
    add: (c) => { if (!this.contains(c)) el.attrs.class = ((el.attrs.class || "") + " " + c).trim(); },
    remove: (c) => { el.attrs.class = (el.attrs.class || "").split(/\s+/).filter((x) => x !== c).join(" "); },
  }; }
  get className() { return this.attrs.class || ""; }
  set className(v) { this.attrs.class = v; }
  get id() { return this.attrs.id || ""; }
  set innerHTML(v) { this.html = v; }
  get innerHTML() { return this.html || ""; }
  get textContent() { return this.text + this.children.map((c) => c.textContent).join(""); }
  set textContent(v) { this.text = v; this.children = []; }
  getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(c) { c.parentNode = this; this.children.push(c); return c; }
  insertBefore(c) { return this.appendChild(c); }
  remove() { if (this.parentNode) this.parentNode.children = this.parentNode.children.filter((x) => x !== this); }
  addEventListener() { this.listeners++; }
  closest() { return null; }
  get firstChild() { return this.children[0] || null; }
  descendants() { return this.children.flatMap((c) => [c, ...c.descendants()]); }
  matches(sel) {
    const m = /^([a-z]*)((?:[.#][\w-]+)*)((?::not\(\.[\w-]+\))*)$/.exec(sel);
    if (!m) return false;
    if (m[1] && this.tagName !== m[1].toUpperCase()) return false;
    for (const p of m[2].match(/[.#][\w-]+/g) || []) {
      if (p[0] === "." && !this.classList.contains(p.slice(1))) return false;
      if (p[0] === "#" && this.id !== p.slice(1)) return false;
    }
    for (const n of m[3].match(/\.[\w-]+/g) || []) if (this.classList.contains(n.slice(1))) return false;
    return true;
  }
  querySelectorAll(sels) {
    const out = [];
    for (const raw of sels.split(",")) {
      const sel = raw.trim();
      if (sel.startsWith(":scope >")) {
        const parts = sel.slice(8).trim().split(/\s+/);
        if (parts.length === 1) out.push(...this.children.filter((c) => c.matches(parts[0])));
        continue;
      }
      const parts = sel.split(/\s+/).filter((p) => p !== ">");
      let set = [this];
      for (const p of parts) set = set.flatMap((e) => e.descendants().filter((d) => d.matches(p)));
      out.push(...set);
    }
    return [...new Set(out)];
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
}
const body = new El("body");
const list = body.appendChild(new El("tbody"));
list.setAttribute("id", "docker_list");
function row(name, cls) {
  const tr = new El("tr"); tr.setAttribute("class", cls);
  const n = tr.appendChild(new El("td")); n.setAttribute("class", "ct-name");
  const a = n.appendChild(new El("span")); a.setAttribute("class", "appname"); a.text = name;
  const u = tr.appendChild(new El("td")); u.setAttribute("class", "updatecolumn"); u.text = "up-to-date";
  return tr;
}
for (const name of ["immich", "planka"]) list.appendChild(row(name, "sortable"));
let observed = null;
globalThis.MutationObserver = class { constructor(cb) { observed = cb; } observe() {} disconnect() {} };
globalThis.document = { body, documentElement: new El("html"), createElement: (t) => new El(t),
  querySelector: (s) => body.querySelector(s), querySelectorAll: (s) => body.querySelectorAll(s),
  getElementById: () => null, addEventListener() {} };
globalThis.getComputedStyle = () => ({ backgroundColor: "rgb(0,0,0)", getPropertyValue: () => "" });
globalThis.window = globalThis;
window.shiplogGlyphs = { Warning: "<svg></svg>", Sync: "<svg></svg>", ExternalLink: "<svg></svg>", Close: "<svg></svg>" };
window.docker = [];
const statuses = [{ container: { name: "immich" }, kind: "none" }, { container: { name: "planka" }, kind: "none" }];
globalThis.fetch = (url) => Promise.resolve({ ok: true, json: () => Promise.resolve(String(url).includes("guards") ? { guards: [] } : statuses) });
const chips = () => list.children.map((tr) => tr.querySelector("td.updatecolumn").children.filter((c) => c.classList.contains("sl-chiprow")).length);
const pass = () => new Promise((r) => setTimeout(r, 120)).then(() => { observed([{ target: list }]); });
(async () => {
  require(process.argv[1]);
  await new Promise((r) => setTimeout(r, 50));
  const out = { tagged: chips() };
  // The folder plugin rebuilds a member's row and keeps the mark on its cell.
  const rebuilt = row("planka", "folder-element");
  rebuilt.querySelector("td.updatecolumn").setAttribute("data-shiplog", "1");
  list.children[1].remove();
  list.appendChild(rebuilt);
  await pass();
  out.rebuilt = chips();
  await pass();
  out.again = chips();
  console.log(JSON.stringify(out));
})();
`

func TestAFolderMemberGetsItsChipBackOnce(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	script, err := filepath.Abs(filepath.Join(pluginDir, "scripts", "docker.js"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, "-e", folderRun, script).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	// docker.js logs its own line before the result.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var got struct{ Tagged, Rebuilt, Again []int }
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for name, chips := range map[string][]int{"first pass": got.Tagged, "after the rebuild": got.Rebuilt, "next pass": got.Again} {
		if len(chips) != 2 || chips[0] != 1 || chips[1] != 1 {
			t.Errorf("%s: chips per row = %v, want one each", name, chips)
		}
	}
}
