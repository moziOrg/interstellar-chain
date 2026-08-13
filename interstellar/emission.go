package interstellar

import (
	"context"
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

type supplyKeeper interface {
	GetSupply(context.Context, string) sdk.Coin
}

const (
	EmissionYears                     = 30
	InitialCirculation                = 17_000_000
	MaximumSupply                     = 170_000_000
	SecondsPerYear                    = 31_557_600
	TargetBlockSeconds                = 3
	GovernanceMinDepositHUGE          = 10_000
	GovernanceExpeditedMinDepositHUGE = 50_000

	// CompoundEmissionInitialAnnualHUGE is the first post-genesis annual
	// issuance. Subsequent annual budgets follow a 2.6545203947165086% compound
	// growth curve for 30 years, then permanently stop.
	CompoundEmissionInitialAnnualHUGE = 3_400_000

	// CompoundEmissionGrowthNumerator / CompoundEmissionGrowthDenominator is
	// 1.026545203947165086. This rate makes the 30-year curve total 153M HUGE;
	// it is encoded as an integer ratio so consensus never depends on floating
	// point arithmetic.
	CompoundEmissionGrowthNumerator   int64 = 1_026_545_203_947_165_086
	CompoundEmissionGrowthDenominator int64 = 1_000_000_000_000_000_000
)

var (
	AttoPerHUGE                       = math.NewInt(1_000_000_000_000_000_000)
	MaxSupplyAtto                     = AttoPerHUGE.MulRaw(MaximumSupply)
	InitialCirculationAtto            = AttoPerHUGE.MulRaw(InitialCirculation)
	EmissionReserveAtto               = MaxSupplyAtto.Sub(InitialCirculationAtto)
	GovernanceMinDepositAtto          = AttoPerHUGE.MulRaw(GovernanceMinDepositHUGE)
	GovernanceExpeditedMinDepositAtto = AttoPerHUGE.MulRaw(GovernanceExpeditedMinDepositHUGE)
)

func BlocksPerYear() uint64 {
	return SecondsPerYear / TargetBlockSeconds
}

var compoundAnnualEmissionSchedule = buildCompoundAnnualEmissionSchedule()

// buildCompoundAnnualEmissionSchedule derives the finite issuance schedule
// once at process start. The final annual amount absorbs only the accumulated
// atto-HUGE rounding remainder, so the reserve is exhausted exactly.
func buildCompoundAnnualEmissionSchedule() []math.Int {
	amounts := make([]math.Int, EmissionYears)
	annual := AttoPerHUGE.MulRaw(CompoundEmissionInitialAnnualHUGE)
	total := math.ZeroInt()

	for year := 0; year < EmissionYears-1; year++ {
		amounts[year] = annual
		total = total.Add(annual)
		annual = annual.MulRaw(CompoundEmissionGrowthNumerator).QuoRaw(CompoundEmissionGrowthDenominator)
	}
	amounts[EmissionYears-1] = EmissionReserveAtto.Sub(total)
	return amounts
}

// AnnualEmissionSchedule returns the 30-year compound Interstellar issuance curve.
// Year one emits 3,400,000 HUGE; year 30 emits approximately 7,268,473 HUGE.
// A copy is returned so callers cannot alter the process-wide consensus table.
func AnnualEmissionSchedule() []math.Int {
	amounts := make([]math.Int, len(compoundAnnualEmissionSchedule))
	copy(amounts, compoundAnnualEmissionSchedule)
	return amounts
}

// EmissionTargetAtYear returns the cumulative issuance at the end of a
// completed annual epoch.
func EmissionTargetAtYear(years uint64) math.Int {
	if years == 0 {
		return math.ZeroInt()
	}
	if years >= EmissionYears {
		return EmissionReserveAtto
	}

	total := math.ZeroInt()
	for year := uint64(0); year < years; year++ {
		total = total.Add(compoundAnnualEmissionSchedule[year])
	}
	return total
}

// EmissionTargetAtBlock returns the cumulative scheduled issuance after the
// supplied number of blocks. It avoids per-block rounding loss by deriving the
// block reward from two adjacent cumulative targets.
func EmissionTargetAtBlock(blocks uint64) math.Int {
	blocksPerYear := BlocksPerYear()
	completedYears := blocks / blocksPerYear
	if completedYears >= EmissionYears {
		return EmissionReserveAtto
	}

	target := EmissionTargetAtYear(completedYears)
	blocksIntoYear := blocks % blocksPerYear
	if blocksIntoYear == 0 {
		return target
	}
	annual := EmissionTargetAtYear(completedYears + 1).Sub(target)
	partial := annual.MulRaw(int64(blocksIntoYear)).QuoRaw(int64(blocksPerYear))
	return target.Add(partial)
}

func EmissionForBlock(height int64) math.Int {
	if height <= 0 {
		return math.ZeroInt()
	}
	current := EmissionTargetAtBlock(uint64(height))
	previous := EmissionTargetAtBlock(uint64(height - 1))
	return current.Sub(previous)
}

// InterstellarMintFn mints the finite 30-year schedule into the fee collector. The
// standard distribution module then allocates those block rewards according to
// validator commission and delegation shares.
func NewInterstellarMintFn(bankKeeper supplyKeeper) mintkeeper.MintFn {
	return func(ctx sdk.Context, keeper *mintkeeper.Keeper) error {
		params, err := keeper.Params.Get(ctx)
		if err != nil {
			return err
		}
		if params.MintDenom != BaseDenom {
			return fmt.Errorf("invalid Interstellar mint denomination %q", params.MintDenom)
		}

		minter, err := keeper.Minter.Get(ctx)
		if err != nil {
			return err
		}

		annual := math.ZeroInt()
		year := uint64(ctx.BlockHeight()-1) / BlocksPerYear()
		if year < EmissionYears {
			annual = AnnualEmissionSchedule()[year]
		}
		minter.Inflation = math.LegacyZeroDec()
		minter.AnnualProvisions = math.LegacyNewDecFromInt(annual)
		if err := keeper.Minter.Set(ctx, minter); err != nil {
			return err
		}

		minted := EmissionForBlock(ctx.BlockHeight())
		currentSupply := bankKeeper.GetSupply(ctx, BaseDenom).Amount
		if currentSupply.GTE(MaxSupplyAtto) {
			minted = math.ZeroInt()
		} else if currentSupply.Add(minted).GT(MaxSupplyAtto) {
			minted = MaxSupplyAtto.Sub(currentSupply)
		}
		if minted.IsZero() {
			return nil
		}

		coins := sdk.NewCoins(sdk.NewCoin(BaseDenom, minted))
		if err := keeper.MintCoins(ctx, coins); err != nil {
			return err
		}
		if err := keeper.AddCollectedFees(ctx, coins); err != nil {
			return err
		}
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			minttypes.EventTypeMint,
			sdk.NewAttribute(minttypes.AttributeKeyInflation, minter.Inflation.String()),
			sdk.NewAttribute(minttypes.AttributeKeyAnnualProvisions, minter.AnnualProvisions.String()),
			sdk.NewAttribute(sdk.AttributeKeyAmount, minted.String()+BaseDenom),
		))
		return nil
	}
}
