package interstellar

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestValidatorCreationRequiresMinimumSelfDelegation(t *testing.T) {
	creator := sdk.AccAddress(make([]byte, 20))
	creator[19] = 1
	create := &stakingtypes.MsgCreateValidator{
		ValidatorAddress:  sdk.ValAddress(creator).String(),
		Value:             sdk.NewCoin(BaseDenom, MinimumValidatorSelfDelegation),
		MinSelfDelegation: MinimumValidatorSelfDelegation,
	}

	creators, err := validatorCreationRequirements([]sdk.Msg{create})
	if err != nil {
		t.Fatalf("expected valid validator admission, got %v", err)
	}
	if creators[creator.String()] != 1 {
		t.Fatal("expected one validator lock requirement")
	}
}

func TestValidatorCreationRejectsSelfDelegationBelowMinimum(t *testing.T) {
	creator := sdk.AccAddress(make([]byte, 20))
	creator[19] = 1
	create := &stakingtypes.MsgCreateValidator{
		ValidatorAddress:  sdk.ValAddress(creator).String(),
		Value:             sdk.NewCoin(BaseDenom, MinimumValidatorSelfDelegation.SubRaw(1)),
		MinSelfDelegation: MinimumValidatorSelfDelegation,
	}

	if _, err := validatorCreationRequirements([]sdk.Msg{create}); err == nil {
		t.Fatal("expected validator creation below the minimum to fail")
	}
}

func TestValidatorCreationRejectsLowPersistentSelfDelegation(t *testing.T) {
	creator := sdk.AccAddress(make([]byte, 20))
	creator[19] = 1
	create := &stakingtypes.MsgCreateValidator{
		ValidatorAddress:  sdk.ValAddress(creator).String(),
		Value:             sdk.NewCoin(BaseDenom, MinimumValidatorSelfDelegation),
		MinSelfDelegation: MinimumValidatorSelfDelegation.SubRaw(1),
	}

	if _, err := validatorCreationRequirements([]sdk.Msg{create}); err == nil {
		t.Fatal("expected low min_self_delegation to fail")
	}
}

func TestValidatorCreationRequirementsUnpacksAuthzExec(t *testing.T) {
	creator := sdk.AccAddress(make([]byte, 20))
	creator[19] = 1
	create := &stakingtypes.MsgCreateValidator{
		ValidatorAddress:  sdk.ValAddress(creator).String(),
		Value:             sdk.NewCoin(BaseDenom, MinimumValidatorSelfDelegation),
		MinSelfDelegation: MinimumValidatorSelfDelegation,
	}
	exec := authz.NewMsgExec(creator, []sdk.Msg{create})

	creators, err := validatorCreationRequirements([]sdk.Msg{&exec})
	if err != nil {
		t.Fatalf("expected authz-wrapped validator creation to be checked, got %v", err)
	}
	if creators[creator.String()] != 1 {
		t.Fatal("expected authz-wrapped validator creation to require one lock")
	}
}
