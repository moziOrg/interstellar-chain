package interstellar

import (
	"context"

	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// UpgradeNameV2 is the first named production upgrade path. A governance
// software-upgrade proposal with this name requires validators to install a
// binary containing this handler before the scheduled height.
const UpgradeNameV2 = "interstellar-v2"

// RegisterUpgradeHandlers registers every upgrade supported by this binary.
// Future releases must add a new, immutable plan name and any required store
// upgrades in the same change set; changing an existing plan handler is a
// consensus-breaking operation.
func (app *EVMD) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(UpgradeNameV2, func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
	})
}
