package interstellar

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const DeadAddressHex = "0x000000000000000000000000000000000000dEaD"

var (
	MinimumValidatorSelfDelegation = AttoPerHUGE.MulRaw(500_000)
	ValidatorCreationLock          = AttoPerHUGE.MulRaw(5_000)
)

func DeadAddress() sdk.AccAddress {
	return sdk.AccAddress(common.HexToAddress(DeadAddressHex).Bytes())
}

type validatorAdmissionBankKeeper interface {
	SendCoins(context.Context, sdk.AccAddress, sdk.AccAddress, sdk.Coins) error
}

// ValidatorAdmissionService applies Interstellar's validator-creation policy
// before the native staking MsgServer mutates staking state. It is shared by
// the SDK message route and the EVM staking precompile route.
type ValidatorAdmissionService struct {
	bankKeeper validatorAdmissionBankKeeper
}

func NewValidatorAdmissionService(bankKeeper validatorAdmissionBankKeeper) ValidatorAdmissionService {
	return ValidatorAdmissionService{bankKeeper: bankKeeper}
}

// AdmitCreateValidator validates the protocol minimums and locks the creation
// deposit. It runs in the same message-execution cache as CreateValidator, so
// a subsequent native staking error reverts the lock transfer as well.
func (s ValidatorAdmissionService) AdmitCreateValidator(ctx sdk.Context, create *stakingtypes.MsgCreateValidator) error {
	creator, err := validateValidatorCreation(create)
	if err != nil {
		return err
	}
	// GenTxs run at height zero. Their lock allocation is part of the immutable
	// genesis distribution, while the self-delegation requirements still apply.
	if ctx.BlockHeight() == 0 {
		return nil
	}
	if err := s.bankKeeper.SendCoins(ctx, creator, DeadAddress(), sdk.NewCoins(sdk.NewCoin(BaseDenom, ValidatorCreationLock))); err != nil {
		return fmt.Errorf("lock validator creation deposit: %w", err)
	}
	return nil
}

func validateValidatorCreation(create *stakingtypes.MsgCreateValidator) (sdk.AccAddress, error) {
	if create.Value.Denom != BaseDenom || create.Value.Amount.LT(MinimumValidatorSelfDelegation) {
		return nil, fmt.Errorf("validator creation requires at least %s%s self-delegation", MinimumValidatorSelfDelegation.String(), BaseDenom)
	}
	if create.MinSelfDelegation.LT(MinimumValidatorSelfDelegation) {
		return nil, fmt.Errorf("validator creation requires min_self_delegation of at least %s%s", MinimumValidatorSelfDelegation.String(), BaseDenom)
	}
	valAddr, err := sdk.ValAddressFromBech32(create.ValidatorAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid validator address: %w", err)
	}
	return sdk.AccAddress(valAddr), nil
}

// ValidatorAdmissionValidationAnteHandler is deliberately read-only. It keeps
// invalid SDK (including authz-wrapped) create-validator transactions out of
// Prepare/ProcessProposal, while the MsgServer wrapper remains the
// authoritative shared state transition for both SDK and EVM routes.
func ValidatorAdmissionValidationAnteHandler(next sdk.AnteHandler) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		if err := validateValidatorCreationMessages(tx.GetMsgs()); err != nil {
			return ctx, err
		}
		return next(ctx, tx, simulate)
	}
}

func validateValidatorCreationMessages(msgs []sdk.Msg) error {
	for _, msg := range msgs {
		if exec, ok := msg.(*authz.MsgExec); ok {
			wrapped, err := exec.GetMessages()
			if err != nil {
				return fmt.Errorf("unpack authz messages: %w", err)
			}
			if err := validateValidatorCreationMessages(wrapped); err != nil {
				return err
			}
			continue
		}
		if create, ok := msg.(*stakingtypes.MsgCreateValidator); ok {
			if _, err := validateValidatorCreation(create); err != nil {
				return err
			}
		}
	}
	return nil
}

type validatorAdmissionMsgServer struct {
	stakingtypes.MsgServer
	admission ValidatorAdmissionService
}

func NewValidatorAdmissionMsgServer(next stakingtypes.MsgServer, admission ValidatorAdmissionService) stakingtypes.MsgServer {
	return validatorAdmissionMsgServer{MsgServer: next, admission: admission}
}

func (s validatorAdmissionMsgServer) CreateValidator(goCtx context.Context, create *stakingtypes.MsgCreateValidator) (*stakingtypes.MsgCreateValidatorResponse, error) {
	if err := s.admission.AdmitCreateValidator(sdk.UnwrapSDKContext(goCtx), create); err != nil {
		return nil, err
	}
	return s.MsgServer.CreateValidator(goCtx, create)
}
