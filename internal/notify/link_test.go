package notify

import (
	"testing"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

func TestUpdateLink(t *testing.T) {
	tests := []struct {
		name string
		st   model.UpdateStatus
		want string
	}{
		{"changelog file links the file itself",
			model.UpdateStatus{Changelog: &model.Changelog{Provider: "file", URL: "https://raw.githubusercontent.com/o/r/main/CHANGELOG.md"}},
			"https://raw.githubusercontent.com/o/r/main/CHANGELOG.md"},
		{"releases link to the repo root",
			model.UpdateStatus{Changelog: &model.Changelog{Provider: "github", URL: "https://github.com/o/r/releases"}},
			"https://github.com/o/r"},
		{"no changelog falls back to the source label",
			model.UpdateStatus{Container: model.Container{Source: "https://github.com/o/r.git"}},
			"https://github.com/o/r"},
	}
	for _, tt := range tests {
		if got := updateLink(tt.st); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
