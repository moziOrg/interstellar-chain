package interstellar

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const DeadAddressHex = "0x000000000000000000000000000000000000dEaD"

var (
	MinimumValidatorSelfDelegation = AttoPerHUGE.MulRaw(500_000)
	ValidatorCreationLock          = AttoPerHUGE.MulRaw(1_000)
)

func DeadAddress() sdk.AccAddress {
	return sdk.AccAddress(common.HexToAddress(DeadAddressHex).Bytes())
}

type validatorAdmissionBankKeeper interface {
	SendCoins(context.Context, sdk.AccAddress, sdk.AccAddress, sdk.Coins) error
}

// ValidatorAdmissionAnteHandler requires a 500,000 HUGE self-delegation and
// atomically locks 1,000 HUGE in DEAD_ADDRESS for every validator creation.
// The automatic transfer preserves the standard staking create-validator CLI.
func ValidatorAdmissionAnteHandler(bankKeeper validatorAdmissionBankKeeper, next sdk.AnteHandler) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		creators, err := validatorCreationRequirements(tx.GetMsgs())
		if err != nil {
			return ctx, err
		}
		// GenTxs are replayed while InitChain runs at height zero. Their required
		// locks are allocated directly in the immutable genesis state because the
		// SDK GenTx format permits only MsgCreateValidator. The self-delegation
		// requirement is still validated above.
		if ctx.BlockHeight() == 0 {
			return next(ctx, tx, simulate)
		}
		for creator, count := range creators {
			amount := ValidatorCreationLock.MulRaw(int64(count))
			creatorAddr, err := sdk.AccAddressFromBech32(creator)
			if err != nil {
				return ctx, fmt.Errorf("invalid validator creator address: %w", err)
			}
			if err := bankKeeper.SendCoins(ctx, creatorAddr, DeadAddress(), sdk.NewCoins(sdk.NewCoin(BaseDenom, amount))); err != nil {
				return ctx, fmt.Errorf("lock validator creation deposit: %w", err)
			}
		}
		return next(ctx, tx, simulate)
	}
}

func validatorCreationRequirements(msgs []sdk.Msg) (map[string]int, error) {
	creators := make(map[string]int)
	for _, msg := range msgs {
		create, ok := msg.(*stakingtypes.MsgCreateValidator)
		if !ok {
			continue
		}
		if create.Value.Denom != BaseDenom || create.Value.Amount.LT(MinimumValidatorSelfDelegation) {
			return nil, fmt.Errorf("validator creation requires at least %s%s self-delegation", MinimumValidatorSelfDelegation.String(), BaseDenom)
		}
		valAddr, err := sdk.ValAddressFromBech32(create.ValidatorAddress)
		if err != nil {
			return nil, fmt.Errorf("invalid validator address: %w", err)
		}
		creators[sdk.AccAddress(valAddr).String()]++
	}
	return creators, nil
}
