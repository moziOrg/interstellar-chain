package interstellar

import (
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

	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err != nil {
		t.Fatalf("expected valid mainnet initial supply, got %v", err)
	}

	bankState.Supply = bankState.Supply.Add(sdk.NewCoin(BaseDenom, AttoPerHUGE))
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(bankState)
	if err := ValidateInitialSupply(cdc, MainnetChainID, genesis); err == nil {
		t.Fatal("expected invalid mainnet supply to fail")
	}
}
