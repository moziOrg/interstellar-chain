package interstellar

import (
	"testing"

	"cosmossdk.io/math"
)

func TestNewConsensusParams(t *testing.T) {
	params := NewConsensusParams()
	if params.Block.MaxGas != DefaultBlockGasLimit {
		t.Fatalf("expected block gas limit %d, got %d", DefaultBlockGasLimit, params.Block.MaxGas)
	}
}

func TestNewFeeMarketGenesisStateHasGasPriceFloor(t *testing.T) {
	state := NewFeeMarketGenesisState()
	if state.Params.NoBaseFee {
		t.Fatal("expected EIP-1559 base fee to be enabled")
	}

	want := math.LegacyNewDec(DefaultMinGasPriceWei)
	if !state.Params.MinGasPrice.Equal(want) {
		t.Fatalf("expected minimum gas price %s, got %s", want, state.Params.MinGasPrice)
	}
}
