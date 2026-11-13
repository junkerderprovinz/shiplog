package fortress

import (
	"testing"

	kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/member"
)

func TestAnAppThatJoinedByItsIDAloneShowsUnderItsHouseName(t *testing.T) {
	got := others([]kitfortress.Member{
		{ID: "a", Kind: member.KindInstance, App: kitfortress.App{ID: "firesquire"}, Device: "Bottich"},
		{ID: "b", Kind: member.KindInstance, App: kitfortress.App{ID: "firesquire", Name: "firesquire"}, Device: "Atlas"},
		{ID: "c", Kind: member.KindInstance, App: kitfortress.App{ID: "bombvault", Name: "BombVault Home"}, Device: "Atlas"},
		{ID: "d", Kind: member.KindInstance, App: kitfortress.App{ID: "somethingnew"}, Device: "Atlas"},
	})
	want := []string{"FireSquire", "FireSquire", "BombVault Home", "somethingnew"}
	for i, o := range got {
		if o.Name != want[i] {
			t.Errorf("member %s is named %q, want %q", o.ID, o.Name, want[i])
		}
	}
}
