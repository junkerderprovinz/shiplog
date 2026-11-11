package updates

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/shiplog/internal/autoupdate"
	"github.com/junkerderprovinz/shiplog/internal/model"
)

func askManaged(t *testing.T, m *Managed) (bool, []string) {
	t.Helper()
	raw, err := m.Action().Run(context.Background(), fortress.Caller{ID: "m1", App: "bombvault", Device: "Tower"}, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var a struct {
		AutoUpdate *bool    `json:"autoUpdate"`
		Containers []string `json:"containers"`
	}
	if err := json.Unmarshal(raw, &a); err != nil || a.AutoUpdate == nil || a.Containers == nil {
		t.Fatalf("answer %s, %v; want autoUpdate and a list", raw, err)
	}
	return *a.AutoUpdate, a.Containers
}

func TestTheManagedActionIsUpdatesManaged(t *testing.T) {
	if got := (&Managed{}).Action().Name; got != "updates.managed" {
		t.Fatalf("action = %q", got)
	}
}

func TestManagedNamesTheContainersTheAutoUpdateLooksAfter(t *testing.T) {
	pinned := status("pinned", model.KindNone)
	pinned.Container.PinnedDigest = "sha256:a"
	local := status("local", model.KindNone)
	local.Container.IsLocal = true
	blocked := status("blocked", model.KindPatch)
	blocked.Changelog = &model.Changelog{Raw: "a BREAKING change"}
	store := fakeStore{
		status("calm", model.KindNone),
		status("patch", model.KindPatch),
		status("major", model.KindMajor),
		status("Plex", model.KindPatch),
		status("dozzle", model.KindNone),
		pinned, local, blocked,
	}
	m := &Managed{Store: store, On: true, Policy: autoupdate.Policy{
		Level: autoupdate.LevelMinor, ExcludeContainers: []string{"plex", "Dozzle"}, ExcludeWords: []string{"breaking"},
	}}
	on, names := askManaged(t, m)
	if !on || len(names) != 2 || names[0] != "calm" || names[1] != "patch" {
		t.Fatalf("answer = %v %v, want calm and patch", on, names)
	}
}

func TestManagedNamesNothingWhileTheAutoUpdateIsOff(t *testing.T) {
	m := &Managed{Store: fakeStore{status("calm", model.KindNone)}, Policy: autoupdate.Policy{Level: autoupdate.LevelMajor}}
	if on, names := askManaged(t, m); on || len(names) != 0 {
		t.Fatalf("answer = %v %v, want off and none", on, names)
	}
}
