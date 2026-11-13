package plugin

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

// adapterRun loads fortress.js in node with a page and an engine of its own
// and prints, as JSON, what the adapter hands the card for each engine state.
const adapterRun = `
const states = JSON.parse(process.argv[2]);
let answer = null, adapter = null;
globalThis.window = {
  SL_LANG: "en", SLT: { s_fortress_no_backup: "Until then, auto-updates run without a backup." },
  GlimFortressCard: { mount: (el, a) => { adapter = a; } },
};
globalThis.document = { getElementById: () => ({ hidden: false }) };
globalThis.fetch = () => Promise.resolve({ ok: true, json: () => Promise.resolve(answer) });
require(process.argv[1]);
(async () => {
  const out = [];
  for (const s of states) {
    answer = s;
    const steps = {};
    steps.load = (await adapter.load()).hints;
    steps.join = (await adapter.join(["legal"])).hints;
    steps.leave = (await adapter.leave()).hints;
    steps.remove = (await adapter.remove("5f3a")).hints;
    out.push(steps);
  }
  console.log(JSON.stringify(out));
})();
`

func TestTheCardWarnsOfUpdatesWithoutABackupWhileShipLogIsOut(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	states := []map[string]any{
		{"joined": false, "removed": true},
		{"joined": true, "phraseChanged": true},
		{"joined": true},
		{"joined": false},
	}
	raw, err := json.Marshal(states)
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join(pluginDir, "scripts", "fortress.js"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, "-e", adapterRun, script, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	type hint struct{ Text, Tone string }
	var got []map[string][]hint
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	warn := []hint{{Text: "Until then, auto-updates run without a backup.", Tone: "warn"}}
	for i, steps := range got {
		want := []hint{}
		if i < 2 {
			want = warn
		}
		for _, step := range []string{"load", "join", "leave", "remove"} {
			hints, ok := steps[step]
			if !ok || hints == nil || len(hints) != len(want) || (len(want) == 1 && hints[0] != want[0]) {
				t.Errorf("%v, %s: hints = %+v, want %+v", states[i], step, hints, want)
			}
		}
	}
}

// refusalRun loads the card and fortress.js in node, has the engine turn a
// join away with the code in argv[3] and prints the card's sentence for the
// adapter's refusal beside its joinLocked.
const refusalRun = `
globalThis.window = globalThis;
window.SL_LANG = "en";
window.SLT = {};
globalThis.document = { getElementById: () => ({ hidden: false }) };
require(process.argv[1]);
const card = window.GlimFortressCard;
let adapter = null;
window.GlimFortressCard = { mount: (el, a) => { adapter = a; } };
globalThis.fetch = () => Promise.resolve({ ok: false, json: () => Promise.resolve({ error: process.argv[3] }) });
require(process.argv[2]);
const t = card.text("en");
adapter.join(["legal"]).then(
  () => console.log(JSON.stringify({ said: "joined", locked: t.joinLocked })),
  (e) => console.log(JSON.stringify({ said: card.joinRefusal(t, e.code), locked: t.joinLocked }))
);
`

func TestAJoinTheFortressTurnsAwayShowsJoinLocked(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	var scripts []string
	for _, name := range []string{"fortress-card.js", "fortress.js"} {
		p, err := filepath.Abs(filepath.Join(pluginDir, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		scripts = append(scripts, p)
	}
	out, err := exec.Command(node, "-e", refusalRun, scripts[0], scripts[1], "closedUntilNewPhrase").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	var got struct{ Said, Locked string }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got.Locked == "" || got.Said != got.Locked {
		t.Errorf("the card says %q, want joinLocked %q", got.Said, got.Locked)
	}
}
