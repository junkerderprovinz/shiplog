package changelog

import (
	"context"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Fallback is the terminal provider in a Chain: it always handles, producing a
// bare version-delta changelog so the engine never returns nothing for an
// update. It carries no release entries and an empty Raw body.
type Fallback struct{}

// Get always returns (changelog, true). The URL is a best-effort compare link
// when the container source is a github.com or gitlab.com repo, otherwise empty.
func (Fallback) Get(_ context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	return &model.Changelog{
		FromTag:  fromTag,
		ToTag:    toTag,
		Raw:      "",
		Provider: "fallback",
		Source:   "version delta only",
		URL:      compareURL(c.Source, fromTag, toTag),
	}, true
}

// compareURL builds a host-appropriate compare link from a github/gitlab source
// URL. It returns "" when the source is empty or an unrecognized host.
func compareURL(source, fromTag, toTag string) string {
	if owner, repo, ok := parseGitHubRepo(source); ok {
		return "https://github.com/" + owner + "/" + repo + "/compare/" + fromTag + "..." + toTag
	}
	if owner, repo, ok := parseGitLabRepo(source); ok {
		return "https://gitlab.com/" + owner + "/" + repo + "/-/compare/" + fromTag + "..." + toTag
	}
	return ""
}

// parseGitLabRepo extracts owner/repo from a gitlab.com URL, tolerating a
// trailing ".git" or "/". It returns false for empty or non-gitlab sources.
func parseGitLabRepo(source string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(source)
	if !strings.Contains(s, "gitlab.com") {
		return "", "", false
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "git@gitlab.com:")
	s = strings.TrimPrefix(s, "gitlab.com/")
	s = strings.TrimPrefix(s, "gitlab.com:")
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")

	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
