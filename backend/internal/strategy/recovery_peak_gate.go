package strategy

import "fmt"

// recoveryEntryPeakGate checks the signal close without consuming a breakout.
// ATH override entries are evaluated separately before the normal strategy.
func recoveryEntryPeakGate(result Result, config Config, state *PersistedState) (Result, bool) {
	if config.RecoveryEntryMinPeakDiscountPct == 0 {
		return Result{}, false
	}
	state.CurrentState = StateEntryPending
	if !finitePositive(config.ATHReferencePeak) {
		return waitingResult(result, *state, config,
			"Recovery Entry 1 is waiting for a known prior historical peak.",
			"Wait for historical peak data to check the minimum entry discount."), true
	}
	threshold := config.ATHReferencePeak * (1 - config.RecoveryEntryMinPeakDiscountPct/100)
	if result.Price <= threshold {
		return Result{}, false
	}
	return waitingResult(result, *state, config,
		fmt.Sprintf("Recovery Entry 1 withheld: completed close %.2f exceeds %.2f, the required %.2f%% discount from the prior historical peak %.2f.", result.Price, threshold, config.RecoveryEntryMinPeakDiscountPct, config.ATHReferencePeak),
		fmt.Sprintf("Wait for a completed close at or below %.2f with recovery and trend conditions still confirmed.", threshold)), true
}
