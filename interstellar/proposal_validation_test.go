//go:build test

package interstellar

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"cosmossdk.io/log/v2"
	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	srvflags "github.com/cosmos/evm/server/flags"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"
)

func TestProcessProposalValidatesAnte(t *testing.T) {
	for _, scenario := range []string{"valid", "signature", "nonce", "fee", "self-delegation"} {
		t.Run(scenario, func(t *testing.T) {
			evmtypes.NewEVMConfigurator().ResetTestConfig()
			app := NewApp(log.NewNopLogger(), dbm.NewMemDB(), true, sims.AppOptionsMap{
				flags.FlagHome: t.TempDir(), srvflags.EVMChainID: DevnetEVMChainID,
			}, baseapp.SetChainID(DevnetChainID))
			t.Cleanup(func() { _ = app.Close() })
			key, err := ethsecp256k1.GenerateKey()
			require.NoError(t, err)
			addr := sdk.AccAddress(key.PubKey().Address())
			gen := app.DefaultGenesis()
			auth := authtypes.DefaultGenesisState()
			auth.Accounts, err = authtypes.PackAccounts([]authtypes.GenesisAccount{authtypes.NewBaseAccount(addr, key.PubKey(), 0, 0)})
			require.NoError(t, err)
			gen[authtypes.ModuleName] = app.AppCodec().MustMarshalJSON(auth)
			var bank banktypes.GenesisState
			app.AppCodec().MustUnmarshalJSON(gen[banktypes.ModuleName], &bank)
			bank.Balances = []banktypes.Balance{{Address: addr.String(), Coins: sdk.NewCoins(sdk.NewCoin(BaseDenom, AttoPerHUGE.MulRaw(1_000_000)))}}
			validator, err := stakingtypes.NewValidator(sdk.ValAddress(addr).String(), ed25519.GenPrivKey().PubKey(), stakingtypes.NewDescription("genesis", "", "", "", ""))
			require.NoError(t, err)
			validator.Status = stakingtypes.Bonded
			validator.Tokens = MinimumValidatorSelfDelegation
			validator.DelegatorShares = math.LegacyNewDecFromInt(validator.Tokens)
			validator.MinSelfDelegation = MinimumValidatorSelfDelegation
			var staking stakingtypes.GenesisState
			app.AppCodec().MustUnmarshalJSON(gen[stakingtypes.ModuleName], &staking)
			staking.Validators = []stakingtypes.Validator{validator}
			staking.Delegations = []stakingtypes.Delegation{stakingtypes.NewDelegation(addr.String(), validator.OperatorAddress, validator.DelegatorShares)}
			gen[stakingtypes.ModuleName] = app.AppCodec().MustMarshalJSON(&staking)
			bank.Balances = append(bank.Balances, banktypes.Balance{Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(), Coins: sdk.NewCoins(sdk.NewCoin(BaseDenom, validator.Tokens))})
			gen[banktypes.ModuleName] = app.AppCodec().MustMarshalJSON(&bank)
			bz, err := json.Marshal(gen)
			require.NoError(t, err)
			params := NewConsensusParams().ToProto()
			_, err = app.InitChain(&abci.RequestInitChain{ChainId: DevnetChainID, AppStateBytes: bz, ConsensusParams: &params})
			require.NoError(t, err)
			builder := app.TxConfig().NewTxBuilder()
			var msg sdk.Msg = &banktypes.MsgSend{FromAddress: addr.String(), ToAddress: sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String(), Amount: sdk.NewCoins(sdk.NewInt64Coin(BaseDenom, 1))}
			if scenario == "self-delegation" {
				msg, err = stakingtypes.NewMsgCreateValidator(sdk.ValAddress(addr).String(), ed25519.GenPrivKey().PubKey(), sdk.NewCoin(BaseDenom, MinimumValidatorSelfDelegation.SubRaw(1)), stakingtypes.NewDescription("test", "", "", "", ""), stakingtypes.NewCommissionRates(math.LegacyZeroDec(), math.LegacyOneDec(), math.LegacyOneDec()), MinimumValidatorSelfDelegation.SubRaw(1))
				require.NoError(t, err)
			}
			require.NoError(t, builder.SetMsgs(msg))
			builder.SetGasLimit(500_000)
			builder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(BaseDenom, math.NewInt(500_000_000_000_000))))
			if scenario == "fee" {
				builder.SetFeeAmount(sdk.Coins{})
			}
			sequence := uint64(0)
			if scenario == "nonce" {
				sequence = 7
			}
			mode := signing.SignMode_SIGN_MODE_DIRECT
			require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: key.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: sequence}))
			sig, err := clienttx.SignWithPrivKey(context.Background(), mode, authsigning.SignerData{ChainID: DevnetChainID, AccountNumber: 0, Sequence: sequence}, builder, key, app.TxConfig(), sequence)
			require.NoError(t, err)
			if scenario == "signature" {
				sig.Data.(*signing.SingleSignatureData).Signature[0] ^= 1
			}
			require.NoError(t, builder.SetSignatures(sig))
			bz, err = app.TxConfig().TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			res, err := app.ProcessProposal(&abci.RequestProcessProposal{Height: 1, Time: time.Unix(1, 0), Txs: [][]byte{bz}})
			require.NoError(t, err)
			if scenario == "valid" {
				require.Equal(t, abci.ResponseProcessProposal_ACCEPT, res.Status)
				// A new round must start from committed state, not from the
				// previous proposal's speculative fee deduction/sequence update.
				again, err := app.ProcessProposal(&abci.RequestProcessProposal{Height: 1, Time: time.Unix(2, 0), Txs: [][]byte{bz}})
				require.NoError(t, err)
				require.Equal(t, abci.ResponseProcessProposal_ACCEPT, again.Status)
			} else {
				require.Equal(t, abci.ResponseProcessProposal_REJECT, res.Status)
			}
		})
	}
}
