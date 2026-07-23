package notify

import "github.com/junkerderprovinz/shiplog/internal/model"

// updateLink returns the best repo link for an update: the changelog's repo root
// when known, else the container's OCI source repo root. "" when neither is set.
// Shared by every notifier so their links match.
func updateLink(st model.UpdateStatus) string {
	link := ""
	if st.Changelog != nil {
		link = repoRoot(st.Changelog.URL)
	}
	if link == "" {
		link = repoRoot(st.Container.Source)
	}
	return link
}
