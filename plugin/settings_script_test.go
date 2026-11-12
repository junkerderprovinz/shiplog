package plugin

import (
	"regexp"
	"strings"
	"testing"
)

// TestNoVariableHidesAFunctionOfTheSettingsPage keeps the names in the page
// script's outer function apart. A var there is hoisted over a function of the
// same name, so once the page has loaded every call to that function throws:
// a save then never answers and the fortress card never shows the join.
func TestNoVariableHidesAFunctionOfTheSettingsPage(t *testing.T) {
	src := read(t, "scripts/settings.js")
	funcs := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^  function ([A-Za-z_$][\w$]*)\(`).FindAllStringSubmatch(src, -1) {
		funcs[m[1]] = true
	}
	if !funcs["note"] {
		t.Fatal("settings.js has no function note, so this check reads the wrong file")
	}
	declarator := regexp.MustCompile(`(?:^var\s+|,\s*)([A-Za-z_$][\w$]*)\s*=[^=]`)
	for _, line := range strings.Split(src, "\n") {
		if !strings.HasPrefix(line, "  var ") {
			continue
		}
		for _, m := range declarator.FindAllStringSubmatch(strings.TrimSpace(line), -1) {
			if funcs[m[1]] {
				t.Errorf("var %s hides function %s: %s", m[1], m[1], strings.TrimSpace(line))
			}
		}
	}
}
