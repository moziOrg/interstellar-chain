package interstellar

import (
	"encoding/json"
	"fmt"

	cmttypes "github.com/cometbft/cometbft/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GenesisState of the blockchain is represented here as a map of raw json
// messages key'd by an identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

// NewConsensusParams returns the Interstellar launch consensus parameters.
func NewConsensusParams() *cmttypes.ConsensusParams {
	params := cmttypes.DefaultConsensusParams()
	params.Block.MaxGas = DefaultBlockGasLimit
	return params
}

// NewEVMGenesisState returns the default genesis state for the EVM module.
//
// The denomination and precompiles are protocol parameters. They must be
// identical on every node from genesis onward.
func NewEVMGenesisState() *evmtypes.GenesisState {
	evmGenState := evmtypes.DefaultGenesisState()
	evmGenState.Params.EvmDenom = BaseDenom
	evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
	evmGenState.Preinstalls = evmtypes.DefaultPreinstalls

	return evmGenState
}

// NewErc20GenesisState returns the default genesis state for the ERC20 module.
//
// Interstellar does not inherit the reference chain's native token pair or contract
// address. A wrapped HUGE deployment is a separately audited launch artifact.
func NewErc20GenesisState() *erc20types.GenesisState {
	return erc20types.DefaultGenesisState()
}

// NewMintGenesisState returns the default genesis state for the mint module.
//
// The reference application's inflation schedule is disabled. Block issuance
// is supplied exclusively by the capped Interstellar mint function.
func NewMintGenesisState() *minttypes.GenesisState {
	mintGenState := minttypes.DefaultGenesisState()

	mintGenState.Params.MintDenom = BaseDenom
	mintGenState.Params.InflationRateChange = math.LegacyZeroDec()
	mintGenState.Params.InflationMax = math.LegacyZeroDec()
	mintGenState.Params.InflationMin = math.LegacyZeroDec()
	mintGenState.Params.BlocksPerYear = BlocksPerYear()
	mintGenState.Params.MaxSupply = MaxSupplyAtto
	mintGenState.Minter.Inflation = math.LegacyZeroDec()
	mintGenState.Minter.AnnualProvisions = math.LegacyZeroDec()
	return mintGenState
}

// NewGovGenesisState sets economically meaningful proposal deposits. Voting
// and deposit windows retain the SDK's reviewed defaults and can later be
// changed by governance itself.
func NewGovGenesisState() *govv1.GenesisState {
	govGenState := govv1.DefaultGenesisState()
	govGenState.Params.MinDeposit = sdk.NewCoins(sdk.NewCoin(BaseDenom, GovernanceMinDepositAtto))
	govGenState.Params.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(BaseDenom, GovernanceExpeditedMinDepositAtto))
	return govGenState
}

// NewFeeMarketGenesisState returns the default genesis state for the feemarket module.
//
// Base fee is enabled so standard EIP-1559 clients can use fee history and
// dynamic-fee transactions. A non-zero minimum prevents empty blocks from
// decaying the base fee to zero. Launch values remain genesis-governed parameters.
func NewFeeMarketGenesisState() *feemarkettypes.GenesisState {
	feeMarketGenState := feemarkettypes.DefaultGenesisState()
	feeMarketGenState.Params.NoBaseFee = false
	feeMarketGenState.Params.MinGasPrice = math.LegacyNewDec(DefaultMinGasPriceWei)

	return feeMarketGenState
}

// NativeDenomMetadata provides the canonical bank metadata for HUGE.
func NativeDenomMetadata() banktypes.Metadata {
	return banktypes.Metadata{
		Description: "The native token of the Interstellar network.",
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: BaseDenom, Exponent: 0},
			{Denom: DisplayDenom, Exponent: DenomExponent},
		},
		Base:    BaseDenom,
		Display: DisplayDenom,
		Name:    "Interstellar",
		Symbol:  DenomSymbol,
	}
}

// ApplyGenesisDefaults applies the Interstellar protocol defaults to an existing
// module genesis map. It is used both by the running application and by the
// init command before accounts and gentxs are added.
func ApplyGenesisDefaults(cdc codec.Codec, genesis GenesisState) GenesisState {
	var bankGenState banktypes.GenesisState
	cdc.MustUnmarshalJSON(genesis[banktypes.ModuleName], &bankGenState)
	if !containsNativeDenomMetadata(bankGenState.DenomMetadata) {
		bankGenState.DenomMetadata = append(bankGenState.DenomMetadata, NativeDenomMetadata())
	}
	genesis[banktypes.ModuleName] = cdc.MustMarshalJSON(&bankGenState)

	var stakingGenState stakingtypes.GenesisState
	cdc.MustUnmarshalJSON(genesis[stakingtypes.ModuleName], &stakingGenState)
	stakingGenState.Params.BondDenom = BaseDenom
	stakingGenState.Params.MaxValidators = 5
	genesis[stakingtypes.ModuleName] = cdc.MustMarshalJSON(&stakingGenState)

	var distrGenState distrtypes.GenesisState
	cdc.MustUnmarshalJSON(genesis[distrtypes.ModuleName], &distrGenState)
	distrGenState.Params.CommunityTax = math.LegacyZeroDec()
	genesis[distrtypes.ModuleName] = cdc.MustMarshalJSON(&distrGenState)

	genesis[minttypes.ModuleName] = cdc.MustMarshalJSON(NewMintGenesisState())
	genesis[govtypes.ModuleName] = cdc.MustMarshalJSON(NewGovGenesisState())
	genesis[evmtypes.ModuleName] = cdc.MustMarshalJSON(NewEVMGenesisState())
	genesis[erc20types.ModuleName] = cdc.MustMarshalJSON(NewErc20GenesisState())
	genesis[feemarkettypes.ModuleName] = cdc.MustMarshalJSON(NewFeeMarketGenesisState())

	return genesis
}

// ValidateInitialSupply enforces the fixed launch allocation on mainnet. Test
// and development networks intentionally retain flexible allocations so local
// and public testnets can fund ephemeral validators and test accounts.
func ValidateInitialSupply(cdc codec.Codec, chainID string, genesis GenesisState) error {
	if chainID != MainnetChainID {
		return nil
	}

	var bankGenState banktypes.GenesisState
	if err := cdc.UnmarshalJSON(genesis[banktypes.ModuleName], &bankGenState); err != nil {
		return fmt.Errorf("decode bank genesis: %w", err)
	}
	actual := bankGenState.Supply.AmountOf(BaseDenom)
	if !actual.Equal(InitialCirculationAtto) {
		return fmt.Errorf(
			"mainnet genesis must allocate exactly %s%s (17,000,000 HUGE), got %s%s",
			InitialCirculationAtto, BaseDenom, actual, BaseDenom,
		)
	}
	return nil
}

func containsNativeDenomMetadata(metadata []banktypes.Metadata) bool {
	for _, item := range metadata {
		if item.Base == BaseDenom {
			return true
		}
	}
	return false
}
