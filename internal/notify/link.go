package notify

import "github.com/junkerderprovinz/shiplog/internal/model"

// updateTarget names the version an update delivers, or "" when all that is known
// is a new image behind the running tag. The changelog's ToTag is the version the
// engine could vouch for. Its newest entry only supplies the spelling, because a
// rolling tag's recent releases say nothing about the image it serves.
func updateTarget(st model.UpdateStatus) string {
	to := st.NewestTag
	if cl := st.Changelog; cl != nil && cl.ToTag != "" {
		to = cl.ToTag
		if len(cl.Entries) > 0 && model.SameVersion(cl.Entries[0].Tag, to) {
			to = cl.Entries[0].Tag
		}
	}
	if model.SameVersion(to, st.Container.Tag) {
		return ""
	}
	return to
}

// updateLink returns the changelog file or the changelog's repo root, else the
// container's OCI source repo root, so every notifier links to the same place.
func updateLink(st model.UpdateStatus) string {
	link := ""
	switch {
	case st.Changelog != nil && st.Changelog.Provider == "file":
		link = st.Changelog.URL
	case st.Changelog != nil:
		link = repoRoot(st.Changelog.URL)
	}
	if link == "" {
		link = repoRoot(st.Container.Source)
	}
	return link
}
