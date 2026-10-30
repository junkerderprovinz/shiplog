// Package plugin holds checks on the Unraid plugin's page sources.
package plugin

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var pluginDir = filepath.Join("src", "shiplog", "usr", "local", "emhttp", "plugins", "shiplog")

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(pluginDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestWindowFootButtonsAreFilled keeps Close and every other answer in a
// window's foot filled (GlimStone rules 3 and 15), since without a ground of
// its own a button there reads as a word.
func TestWindowFootButtonsAreFilled(t *testing.T) {
	page := read(t, "shiplog.page")
	feet := regexp.MustCompile(`<div class="sl-win-act">(.*?)</div>`).FindAllStringSubmatch(page, -1)
	if len(feet) == 0 {
		t.Fatal("no window foot found in shiplog.page")
	}
	button := regexp.MustCompile(`<(button|a)\b[^>]*>`)
	for _, foot := range feet {
		for _, tag := range button.FindAllString(foot[1], -1) {
			if !strings.Contains(tag, "sl-win-btn") {
				t.Errorf("window foot button without sl-win-btn: %s", tag)
			}
		}
	}

	css := regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(read(t, filepath.Join("styles", "settings.css")), "")
	rule := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	ground := regexp.MustCompile(`(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)`)
	clear := regexp.MustCompile(`^(transparent|none)\b`)
	filled := false
	for _, m := range rule.FindAllStringSubmatch(css, -1) {
		if !strings.Contains(m[1], "sl-win-btn") {
			continue
		}
		for _, d := range ground.FindAllStringSubmatch(m[2], -1) {
			v := strings.TrimSpace(d[1])
			if clear.MatchString(v) {
				t.Errorf("%s clears the window button's fill", strings.TrimSpace(m[1]))
			} else if strings.Contains(m[1], "cc-popups-on") && !strings.Contains(m[1], ":hover") {
				filled = true
			}
		}
	}
	if !filled {
		t.Error("settings.css gives sl-win-btn no fill of its own under TowerTorch")
	}
}
