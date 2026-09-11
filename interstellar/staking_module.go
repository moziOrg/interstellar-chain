package interstellar

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
	staking "github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// validatorAdmissionStakingModule keeps the native staking module's API and
// migrations, while registering the Interstellar admission wrapper as the
// MsgServer for CreateValidator.
type validatorAdmissionStakingModule struct {
	staking.AppModule
	keeper    *stakingkeeper.Keeper
	admission ValidatorAdmissionService
}

func newValidatorAdmissionStakingModule(
	cdc codec.Codec,
	keeper *stakingkeeper.Keeper,
	accountKeeper stakingtypes.AccountKeeper,
	bankKeeper stakingtypes.BankKeeper,
	admission ValidatorAdmissionService,
) validatorAdmissionStakingModule {
	return validatorAdmissionStakingModule{
		AppModule: staking.NewAppModule(cdc, keeper, accountKeeper, bankKeeper, nil),
		keeper:    keeper,
		admission: admission,
	}
}

func (am validatorAdmissionStakingModule) RegisterServices(cfg module.Configurator) {
	stakingtypes.RegisterMsgServer(cfg.MsgServer(), NewValidatorAdmissionMsgServer(stakingkeeper.NewMsgServerImpl(am.keeper), am.admission))
	stakingtypes.RegisterQueryServer(cfg.QueryServer(), stakingkeeper.NewQuerier(am.keeper))

	migrator := stakingkeeper.NewMigrator(am.keeper, nil)
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 1, migrator.Migrate1to2); err != nil {
		panic(fmt.Sprintf("register staking migration 1 to 2: %v", err))
	}
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 2, migrator.Migrate2to3); err != nil {
		panic(fmt.Sprintf("register staking migration 2 to 3: %v", err))
	}
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 3, migrator.Migrate3to4); err != nil {
		panic(fmt.Sprintf("register staking migration 3 to 4: %v", err))
	}
	if err := cfg.RegisterMigration(stakingtypes.ModuleName, 4, migrator.Migrate4to5); err != nil {
		panic(fmt.Sprintf("register staking migration 4 to 5: %v", err))
	}
}
