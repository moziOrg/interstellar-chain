package interstellar

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"
)

type admissionBankKeeperMock struct {
	from, to sdk.AccAddress
	coins    sdk.Coins
	calls    int
}

func (m *admissionBankKeeperMock) SendCoins(_ context.Context, from, to sdk.AccAddress, coins sdk.Coins) error {
	m.from, m.to, m.coins = from, to, coins
	m.calls++
	return nil
}

type admissionMsgServerMock struct {
	stakingtypes.MsgServer
	calls int
}

func (m *admissionMsgServerMock) CreateValidator(context.Context, *stakingtypes.MsgCreateValidator) (*stakingtypes.MsgCreateValidatorResponse, error) {
	m.calls++
	return &stakingtypes.MsgCreateValidatorResponse{}, nil
}

func admissionCreateValidator(amount, minSelf math.Int) (*stakingtypes.MsgCreateValidator, sdk.AccAddress) {
	creator := sdk.AccAddress(make([]byte, 20))
	creator[19] = 1
	return &stakingtypes.MsgCreateValidator{
		ValidatorAddress:  sdk.ValAddress(creator).String(),
		Value:             sdk.NewCoin(BaseDenom, amount),
		MinSelfDelegation: minSelf,
	}, creator
}

func TestValidatorAdmissionValidatesProjectMinimums(t *testing.T) {
	create, creator := admissionCreateValidator(MinimumValidatorSelfDelegation, MinimumValidatorSelfDelegation)
	got, err := validateValidatorCreation(create)
	require.NoError(t, err)
	require.Equal(t, creator, got)

	create, _ = admissionCreateValidator(MinimumValidatorSelfDelegation.SubRaw(1), MinimumValidatorSelfDelegation)
	_, err = validateValidatorCreation(create)
	require.Error(t, err)

	create, _ = admissionCreateValidator(MinimumValidatorSelfDelegation, MinimumValidatorSelfDelegation.SubRaw(1))
	_, err = validateValidatorCreation(create)
	require.Error(t, err)
}

func TestValidatorAdmissionLocksOnlyAfterGenesis(t *testing.T) {
	create, creator := admissionCreateValidator(MinimumValidatorSelfDelegation, MinimumValidatorSelfDelegation)
	bank := &admissionBankKeeperMock{}
	service := NewValidatorAdmissionService(bank)

	require.NoError(t, service.AdmitCreateValidator(sdk.Context{}.WithBlockHeight(0), create))
	require.Zero(t, bank.calls)

	require.NoError(t, service.AdmitCreateValidator(sdk.Context{}.WithBlockHeight(1), create))
	require.Equal(t, 1, bank.calls)
	require.Equal(t, creator, bank.from)
	require.Equal(t, DeadAddress(), bank.to)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin(BaseDenom, ValidatorCreationLock)), bank.coins)
	require.Equal(t, AttoPerHUGE.MulRaw(5_000), ValidatorCreationLock)
}

func TestValidatorAdmissionMsgServerAppliesPolicyBeforeNativeStaking(t *testing.T) {
	create, _ := admissionCreateValidator(MinimumValidatorSelfDelegation, MinimumValidatorSelfDelegation)
	bank := &admissionBankKeeperMock{}
	native := &admissionMsgServerMock{}
	server := NewValidatorAdmissionMsgServer(native, NewValidatorAdmissionService(bank))

	_, err := server.CreateValidator(sdk.WrapSDKContext(sdk.Context{}.WithBlockHeight(1)), create)
	require.NoError(t, err)
	require.Equal(t, 1, bank.calls)
	require.Equal(t, 1, native.calls)

	low, _ := admissionCreateValidator(MinimumValidatorSelfDelegation.SubRaw(1), MinimumValidatorSelfDelegation)
	_, err = server.CreateValidator(sdk.WrapSDKContext(sdk.Context{}.WithBlockHeight(1)), low)
	require.Error(t, err)
	require.Equal(t, 1, bank.calls)
	require.Equal(t, 1, native.calls)
}

func TestValidatorAdmissionValidationAnteHandlerChecksAuthzMessages(t *testing.T) {
	create, creator := admissionCreateValidator(MinimumValidatorSelfDelegation.SubRaw(1), MinimumValidatorSelfDelegation)
	ante := ValidatorAdmissionValidationAnteHandler(func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	})
	exec := authz.NewMsgExec(creator, []sdk.Msg{create})
	tx := mockTx{msgs: []sdk.Msg{&exec}}
	_, err := ante(sdk.Context{}, tx, false)
	require.Error(t, err)
}

type mockTx struct{ msgs []sdk.Msg }

func (tx mockTx) GetMsgs() []sdk.Msg                 { return tx.msgs }
func (mockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
