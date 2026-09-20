package notify

import "github.com/junkerderprovinz/shiplog/internal/model"

// updateLink returns the changelog's repo root, else the container's OCI source
// repo root, so every notifier links to the same place.
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
