package interstellar

import "testing"

func TestCompoundAnnualEmissionScheduleIsFiniteAndExact(t *testing.T) {
	schedule := AnnualEmissionSchedule()
	if len(schedule) != EmissionYears {
		t.Fatalf("expected %d years, got %d", EmissionYears, len(schedule))
	}
	total := EmissionTargetAtBlock(uint64(EmissionYears) * BlocksPerYear())
	if !total.Equal(EmissionReserveAtto) {
		t.Fatalf("expected reserve %s, got %s", EmissionReserveAtto, total)
	}
	if !EmissionTargetAtBlock(uint64(EmissionYears)*BlocksPerYear() + 1).Equal(EmissionReserveAtto) {
		t.Fatal("emission must stop after year 30")
	}
	firstYear := AttoPerHUGE.MulRaw(CompoundEmissionInitialAnnualHUGE)
	if !schedule[0].Equal(firstYear) {
		t.Fatalf("expected year one issuance %s, got %s", firstYear, schedule[0])
	}
	if !schedule[EmissionYears-1].GT(AttoPerHUGE.MulRaw(7_250_000)) || !schedule[EmissionYears-1].LT(AttoPerHUGE.MulRaw(7_300_000)) {
		t.Fatalf("expected year thirty issuance near 7.25M HUGE, got %s", schedule[EmissionYears-1])
	}
	for year := 1; year < EmissionYears-1; year++ {
		expected := schedule[year-1].MulRaw(CompoundEmissionGrowthNumerator).QuoRaw(CompoundEmissionGrowthDenominator)
		if !schedule[year].Equal(expected) {
			t.Fatalf("year %d does not follow the compound growth ratio", year+1)
		}
		if !schedule[year].GT(schedule[year-1]) {
			t.Fatalf("year %d issuance must exceed year %d", year+1, year)
		}
	}
	if !schedule[EmissionYears-1].GT(schedule[EmissionYears-2]) {
		t.Fatal("year thirty issuance must exceed year twenty-nine")
	}
}

func TestEmissionForBlockMatchesCumulativeTarget(t *testing.T) {
	height := int64(BlocksPerYear() + 123)
	want := EmissionTargetAtBlock(uint64(height)).Sub(EmissionTargetAtBlock(uint64(height - 1)))
	if !EmissionForBlock(height).Equal(want) {
		t.Fatalf("unexpected block emission at height %d", height)
	}
}
