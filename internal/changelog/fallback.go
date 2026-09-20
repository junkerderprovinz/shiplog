package changelog

import (
	"context"
	"strings"

	"github.com/junkerderprovinz/shiplog/internal/model"
)

// Fallback is the last provider in a Chain. It always answers with a bare
// version delta, so an update never comes without a changelog.
type Fallback struct{}

// Get links to the source repo's releases page on GitHub or GitLab. A compare
// link would often 404, since image tags are rarely git refs.
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

func sourceReleasesURL(source string) string {
	if owner, repo, ok := parseGitHubRepo(source); ok {
		return "https://github.com/" + owner + "/" + repo + "/releases"
	}
	if owner, repo, ok := parseGitLabRepo(source); ok {
		return "https://gitlab.com/" + owner + "/" + repo + "/-/releases"
	}
	return ""
}

// parseGitLabRepo extracts owner/repo from a gitlab.com URL.
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
