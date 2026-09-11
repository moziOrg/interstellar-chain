package interstellar

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestValidateInitialSupplyForMainnet(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	genesis := GenesisState{}
	bankState := banktypes.DefaultGenesisState()
	bankState.Supply = bankState.Supply.Add(sdk.NewCoin(BaseDenom, InitialCirculationAtto))
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(bankState)
	genesis["genutil"] = json.RawMessage(`{"gen_txs":[]}`)

	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err != nil {
		t.Fatalf("expected valid mainnet initial supply, got %v", err)
	}

	bankState.Supply = bankState.Supply.Add(sdk.NewCoin(BaseDenom, AttoPerHUGE))
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(bankState)
	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err == nil {
		t.Fatal("expected invalid mainnet supply to fail")
	}
}

func TestValidateInitialSupplyRequiresMainnetGenesisValidatorLocks(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	genesis := GenesisState{}
	bankState := banktypes.DefaultGenesisState()
	bankState.Supply = bankState.Supply.Add(sdk.NewCoin(BaseDenom, InitialCirculationAtto))
	bankState.Balances = []banktypes.Balance{{
		Address: DeadAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(BaseDenom, ValidatorCreationLock)),
	}}
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(bankState)
	genesis["genutil"] = json.RawMessage(`{"gen_txs":[{"body":{"messages":[{"@type":"/cosmos.staking.v1beta1.MsgCreateValidator"}]}}]}`)

	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err != nil {
		t.Fatalf("expected matching mainnet validator lock, got %v", err)
	}

	bankState.Balances[0].Coins = sdk.NewCoins(sdk.NewCoin(BaseDenom, ValidatorCreationLock.SubRaw(1)))
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(bankState)
	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err == nil {
		t.Fatal("expected missing mainnet validator lock to fail")
	}
}

func TestValidateInitialSupplyRejectsTooManyMainnetGenesisValidators(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	genesis := GenesisState{}
	bankState := banktypes.DefaultGenesisState()
	bankState.Supply = bankState.Supply.Add(sdk.NewCoin(BaseDenom, InitialCirculationAtto))
	bankState.Balances = []banktypes.Balance{{
		Address: DeadAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(BaseDenom, ValidatorCreationLock.MulRaw(int64(DefaultMaxValidators+1)))),
	}}
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(bankState)

	genTx := json.RawMessage(`{"body":{"messages":[{"@type":"/cosmos.staking.v1beta1.MsgCreateValidator"}]}}`)
	genTxs := make([]json.RawMessage, DefaultMaxValidators+1)
	for i := range genTxs {
		genTxs[i] = genTx
	}
	genutilState, err := json.Marshal(struct {
		GenTxs []json.RawMessage `json:"gen_txs"`
	}{GenTxs: genTxs})
	if err != nil {
		t.Fatal(err)
	}
	genesis["genutil"] = genutilState

	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err == nil {
		t.Fatal("expected too many mainnet genesis validators to fail")
	}
}
