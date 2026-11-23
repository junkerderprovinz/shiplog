package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const words = "../../ui/src/locales"

var plural = regexp.MustCompile(`\.(zero|one|two|few|many|other)$`)
var param = regexp.MustCompile(`\{(\w+)\}`)

func flat(t *testing.T, code string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(words, code+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("%s: %v", code, err)
	}
	out := map[string]string{}
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, v := range m {
			switch v := v.(type) {
			case string:
				out[prefix+k] = v
			case map[string]any:
				walk(prefix+k+".", v)
			}
		}
	}
	walk("", tree)
	return out
}

func params(s string) []string {
	var out []string
	for _, m := range param.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	slices.Sort(out)
	return out
}

func TestThePageSpeaksEveryLanguageOfThePlugin(t *testing.T) {
	plugin, err := filepath.Glob("../../plugin/src/shiplog/usr/local/emhttp/plugins/shiplog/lang/*.json")
	if err != nil || len(plugin) == 0 {
		t.Fatalf("plugin languages: %v", err)
	}
	page, _ := filepath.Glob(filepath.Join(words, "*.json"))
	name := func(p string) string { return strings.TrimSuffix(filepath.Base(p), ".json") }
	var want, got []string
	for _, p := range plugin {
		want = append(want, name(p))
	}
	for _, p := range page {
		got = append(got, name(p))
	}
	if !slices.Equal(want, got) {
		t.Fatalf("page languages %v, plugin languages %v", got, want)
	}
}

func TestEveryLanguageHasEnglishsWordsAndBlanks(t *testing.T) {
	en := flat(t, "en")
	page, _ := filepath.Glob(filepath.Join(words, "*.json"))
	for _, p := range page {
		code := strings.TrimSuffix(filepath.Base(p), ".json")
		w := flat(t, code)
		for k, v := range en {
			if plural.MatchString(k) {
				continue
			}
			got, ok := w[k]
			if !ok {
				t.Errorf("%s lacks %s", code, k)
				continue
			}
			if !slices.Equal(params(got), params(v)) {
				t.Errorf("%s %s keeps %v, English %v", code, k, params(got), params(v))
			}
		}
		for k, v := range w {
			if regexp.MustCompile(`TrickWork|CannonCommander|config\.yaml`).MatchString(v) {
				t.Errorf("%s %s names another app: %q", code, k, v)
			}
		}
	}
}
