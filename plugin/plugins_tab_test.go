package plugin

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pluginsRun loads plugins.js in node with a small Plugins tab: each row has
// Unraid's remove checkbox and a version cell. The version cell of one row is
// rewritten the way Unraid does once its own check answers, and the observer
// runs again. It prints the chips per row, their dots, and the window of each
// plugin as JSON.
const pluginsRun = `
class El {
  constructor(tag) { this.tagName = tag.toUpperCase(); this.children = []; this.parentNode = null; this.attrs = {}; this.html = ""; this.style = {}; }
  get classList() { const el = this; return {
    contains: (c) => (el.attrs.class || "").split(/\s+/).includes(c),
    add: (c) => { el.attrs.class = ((el.attrs.class || "") + " " + c).trim(); },
  }; }
  get className() { return this.attrs.class || ""; }
  set className(v) { this.attrs.class = v; }
  set innerHTML(v) { this.html = v; this.children = []; }
  get innerHTML() { return this.html; }
  getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  appendChild(c) { c.parentNode = this; this.children.push(c); return c; }
  remove() { if (this.parentNode) this.parentNode.children = this.parentNode.children.filter((x) => x !== this); }
  addEventListener() {}
  descendants() { return this.children.flatMap((c) => [c, ...c.descendants()]); }
  matches(sel) {
    const m = /^([a-z]*)((?:\.[\w-]+)*)$/.exec(sel);
    if (!m) return false;
    if (m[1] && this.tagName !== m[1].toUpperCase()) return false;
    return (m[2].match(/\.[\w-]+/g) || []).every((c) => this.classList.contains(c.slice(1)));
  }
  querySelectorAll(sel) {
    if (sel.startsWith(":scope >")) return this.children.filter((c) => c.matches(sel.slice(8).trim()));
    return this.descendants().filter((d) => d.matches(sel));
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
}
const body = new El("body");
const list = body.appendChild(new El("tbody"));
function row(file, version) {
  const tr = list.appendChild(new El("tr"));
  for (let i = 0; i < 6; i++) tr.appendChild(new El("td"));
  tr.children[3].html = version;
  const box = tr.children[5].appendChild(new El("input"));
  box.setAttribute("class", "remove"); box.setAttribute("data", file);
  return tr;
}
row("a.plg", "1"); row("b.plg", "1"); row("c.plg", "1"); row("d.plg", "1"); row("other.plg", "1");
let observed = null;
globalThis.MutationObserver = class { constructor(cb) { observed = cb; } observe() {} };
globalThis.document = { body, createElement: (t) => new El(t),
  querySelector: (s) => body.querySelector(s), querySelectorAll: (s) => body.querySelectorAll(s), addEventListener() {} };
globalThis.getComputedStyle = () => ({ backgroundColor: "rgb(0,0,0)" });
globalThis.window = globalThis;
window.shiplogGlyphs = { Sync: "<svg></svg>", ExternalLink: "<svg></svg>", Close: "<svg></svg>", Warning: "<svg></svg>" };
const plugins = [
  { file: "a.plg", name: "a", installed: "2026.01.01", latest: "2026.03.01", state: "available", support: "https://forums.unraid.net/a", changes: "###2026.03.01\n- Fixed <script>x</script>" },
  { file: "b.plg", name: "b", installed: "2026.01.01", latest: "2026.01.01", state: "current", changes: "" },
  { file: "c.plg", name: "c", installed: "1.0", latest: "2.0", state: "incompatible", min: "7.3.0" },
  { file: "d.plg", name: "d", installed: "1.0", state: "unchecked" },
];
globalThis.fetch = (url) => Promise.resolve({ ok: String(url).includes("path=plugins"), json: () => Promise.resolve(plugins) });
const chips = () => list.children.map((tr) => tr.children[3].querySelectorAll(":scope > .sl-chiprow").length);
const dots = () => list.children.map((tr) => { const r = tr.children[3].querySelector(".sl-chiprow"); const c = r && r.children[0]; const m = c && /sl-amp sl-(\w+)/.exec(c.html); return m ? m[1] : ""; });
(async () => {
  require(process.argv[1]);
  await new Promise((r) => setTimeout(r, 50));
  const out = { tagged: chips(), dots: dots() };
  // Unraid rewrites the version cell when its check for the row answers.
  list.children[0].children[3].innerHTML = "2026.01.01<br><span class='red-text'>2026.03.01</span>";
  observed([{ target: list }]);
  await new Promise((r) => setTimeout(r, 120));
  out.rewritten = chips();
  observed([{ target: list }]);
  await new Promise((r) => setTimeout(r, 120));
  out.again = chips();
  out.windows = plugins.map((p) => window.shiplogPlugins.windowHTML(p));
  console.log(JSON.stringify(out));
})();
`

func TestEveryPluginGetsAChangelogChipThatSurvivesUnraidsRewrite(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	script, err := filepath.Abs(filepath.Join(pluginDir, "scripts", "plugins.js"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, "-e", pluginsRun, script).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var got struct {
		Tagged, Rewritten, Again []int
		Dots, Windows            []string
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	want := []int{1, 1, 1, 1, 0} // other.plg is not in the engine's list
	for name, chips := range map[string][]int{"first pass": got.Tagged, "after Unraid's rewrite": got.Rewritten, "next pass": got.Again} {
		if jsonInts(chips) != jsonInts(want) {
			t.Errorf("%s: chips per row = %v, want %v", name, chips, want)
		}
	}
	if strings.Join(got.Dots, ",") != "mid,ok,grey,grey," {
		t.Errorf("dots = %v, want an amber update, a green current and grey for incompatible and unchecked", got.Dots)
	}
	w := got.Windows
	if len(w) != 4 {
		t.Fatalf("windows = %v", w)
	}
	if !strings.Contains(w[0], "2026.01.01 → <b>2026.03.01</b>") || !strings.Contains(w[0], `class="sl-upd"`) ||
		!strings.Contains(w[0], `href="https://forums.unraid.net/a"`) || strings.Contains(w[0], "<script>") || !strings.Contains(w[0], "&lt;script&gt;") {
		t.Errorf("update window = %s", w[0])
	}
	if strings.Contains(w[1], "sl-upd") || !strings.Contains(w[1], "This plugin ships no release notes.") {
		t.Errorf("current window = %s", w[1])
	}
	if strings.Contains(w[2], "sl-upd") || !strings.Contains(w[2], "Version 2.0 needs another Unraid version (≥ 7.3.0)") {
		t.Errorf("incompatible window = %s", w[2])
	}
	if !strings.Contains(w[3], "Not checked yet") || !strings.Contains(w[3], "Check for Updates") {
		t.Errorf("unchecked window = %s", w[3])
	}
}

func jsonInts(v []int) string { b, _ := json.Marshal(v); return string(b) }

var plgKeys = []string{"plgUpdate", "plgUpdateHint", "plgIncompatible", "plgIncompatibleHint", "plgUnchecked", "plgUncheckedHint", "plgNoNotes", "plgSupport"}

// plugins.js falls back to the English of en.json, and every language names
// the chip's states, keeping the placeholders.
func TestEveryLanguageNamesThePluginChip(t *testing.T) {
	js := read(t, filepath.Join("scripts", "plugins.js"))
	page := read(t, "shiplog.Plugins.page")
	if !strings.Contains(page, `Menu="Plugins"`) || !strings.Contains(page, "scripts/plugins.js") || !strings.Contains(page, "styles/docker.css") {
		t.Error("shiplog.Plugins.page does not load plugins.js with the Docker tab's styles")
	}
	if !strings.Contains(read(t, filepath.Join("server", "status.php")), `'plugins' => '/api/plugins'`) {
		t.Error("status.php does not pass path=plugins to /api/plugins")
	}
	files, err := filepath.Glob(filepath.Join(pluginDir, "lang", "*.json"))
	if err != nil || len(files) < 26 {
		t.Fatalf("lang files = %d, %v", len(files), err)
	}
	for _, f := range files {
		var texts map[string]string
		if err := json.Unmarshal([]byte(read(t, filepath.Join("lang", filepath.Base(f)))), &texts); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		for _, k := range plgKeys {
			v := texts["d_"+k]
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: d_%s is missing", filepath.Base(f), k)
			}
			if k == "plgIncompatibleHint" && (!strings.Contains(v, "%v") || !strings.Contains(v, "%r")) {
				t.Errorf("%s: d_%s = %q lost a placeholder", filepath.Base(f), k, v)
			}
			if filepath.Base(f) == "en.json" {
				quoted, _ := json.Marshal(v)
				if !regexp.MustCompile(`\b` + k + `: ` + regexp.QuoteMeta(string(quoted))).MatchString(js) {
					t.Errorf("plugins.js falls back to another English %s than en.json's %s", k, quoted)
				}
			}
		}
	}
}
