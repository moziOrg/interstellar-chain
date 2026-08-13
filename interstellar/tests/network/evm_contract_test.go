//go:build test

package network

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/crypto/hd"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/moziOrg/interstellar-chain/interstellar"
)

// TestInterstellarJSONRPCDeployContract validates the public EVM path: derive a
// funded devnet account, submit a signed EIP-155 transaction over JSON-RPC,
// wait for inclusion, and verify the deployed runtime code.
func TestInterstellarJSONRPCDeployContract(t *testing.T) {
	cfg := DefaultConfig(interstellar.DevnetChainID)
	cfg.NumValidators = 1
	// Keep a spendable native balance after the validator's 2M HUGE self-bond.
	cfg.StakingTokens = interstellar.MinimumValidatorSelfDelegation.Add(interstellar.AttoPerHUGE.MulRaw(10_000))

	network, err := New(t, t.TempDir(), cfg)
	require.NoError(t, err)
	defer network.Cleanup()
	require.Equal(t, interstellar.DevnetEVMChainID, evmtypes.GetChainConfig().ChainId)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, network.WaitForNextBlock())

	client := network.Validators[0].JSONRPCClient
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	require.Equal(t, new(big.Int).SetUint64(interstellar.DevnetEVMChainID), chainID)

	mnemonic := readValidatorMnemonic(t, network.BaseDir)
	privateKey, err := deriveEthereumKey(mnemonic)
	require.NoError(t, err)

	from := crypto.PubkeyToAddress(privateKey.PublicKey)
	require.Equal(t, common.BytesToAddress(network.Validators[0].Address), from)
	balance, err := client.BalanceAt(ctx, from, nil)
	require.NoError(t, err)
	require.True(t, balance.Sign() > 0, "test validator account must be funded")

	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Init code returns a runtime contract that returns uint256(42) for any call.
	initCode := common.FromHex("0x600a600c600039600a6000f3602a60005260206000f3")
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      100_000,
		Data:     initCode,
	})
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privateKey)
	require.NoError(t, err)
	require.NoError(t, client.SendTransaction(ctx, signedTx))

	receipt, err := bind.WaitMined(ctx, client, signedTx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status)

	code, err := client.CodeAt(ctx, receipt.ContractAddress, nil)
	require.NoError(t, err)
	require.Equal(t, "0x602a60005260206000f3", "0x"+fmt.Sprintf("%x", code))
}

func readValidatorMnemonic(t *testing.T, baseDir string) string {
	t.Helper()
	path := filepath.Join(baseDir, "node0", "interstellarcli", "key_seed.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var payload struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal(data, &payload))
	require.NotEmpty(t, payload.Secret)
	return payload.Secret
}

func deriveEthereumKey(mnemonic string) (*ecdsa.PrivateKey, error) {
	keyBytes, err := hd.EthSecp256k1.Derive()(
		mnemonic,
		keyring.DefaultBIP39Passphrase,
		sdk.GetConfig().GetFullBIP44Path(),
	)
	if err != nil {
		return nil, err
	}
	return crypto.ToECDSA(keyBytes)
}
