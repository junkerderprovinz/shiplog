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

// Get always returns (changelog, true). When the container source is a
// github.com or gitlab.com repo, the URL links to that repo's RELEASES page, so
// even a bare version-delta row is a clickable jump to the real source. (A
// version-to-version compare link often 404s because the version tags aren't
// git refs.) Empty for an absent/unrecognized source.
func (Fallback) Get(_ context.Context, c model.Container, fromTag, toTag string) (*model.Changelog, bool) {
	return &model.Changelog{
		FromTag:  fromTag,
		ToTag:    toTag,
		Raw:      "",
		Provider: "fallback",
		Source:   "version delta only",
		URL:      sourceReleasesURL(c.Source),
	}, true
}

// sourceReleasesURL builds a host-appropriate releases link from a github/gitlab
// source URL. It returns "" when the source is empty or an unrecognized host.
func sourceReleasesURL(source string) string {
	if owner, repo, ok := parseGitHubRepo(source); ok {
		return "https://github.com/" + owner + "/" + repo + "/releases"
	}
	if owner, repo, ok := parseGitLabRepo(source); ok {
		return "https://gitlab.com/" + owner + "/" + repo + "/-/releases"
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
