package fortress

import kitfortress "github.com/junkerderprovinz/parleyport/kit/fortress"

// houseNames are how the house's apps write their names, for a member that
// joined through the kit's fortress command, which announces the app by its
// ID alone.
var houseNames = map[string]string{
	"arrowloop": "ArrowLoop", "bombvault": "BombVault", "cannoncommander": "CannonCommander",
	"castleparcel": "CastleParcel", "firesquire": "FireSquire", "knightloader": "KnightLoader",
	"shiplog": "ShipLog", "tollroll": "TollRoll", "towertorch": "TowerTorch",
}

// AppName is the name the settings page shows for a member's app.
func AppName(a kitfortress.App) string {
	if a.Name != "" && a.Name != a.ID {
		return a.Name
	}
	if n, ok := houseNames[a.ID]; ok {
		return n
	}
	return a.ID
}
