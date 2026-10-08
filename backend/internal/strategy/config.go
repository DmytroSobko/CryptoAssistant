package strategy

import "fmt"

// ValidateConfig verifies that every persisted strategy setting can be used by
// the deterministic engine without creating impossible entry or exit rules.
func ValidateConfig(config Config) error {
	strategyID := config.ResolvedStrategyID()
	if !validStrategyID(strategyID) {
		return fmt.Errorf("strategy ID must be %s or %s", StrategyRecoveryBreakout, StrategyDipAccumulation)
	}
	if err := validateSharedConfig(config); err != nil {
		return err
	}
	if strategyID == StrategyDipAccumulation {
		if err := validateDipAccumulationConfig(config); err != nil {
			return err
		}
	}
	if config.ATHEntryOverrideEnabled && !validPositivePercentage(config.ATHEntryThresholdPct) {
		return fmt.Errorf("ATH entry override threshold must be greater than 0 and at most 100")
	}
	return nil
}

// validateSharedConfig is deliberately the original Recovery Breakout
// validation. Keeping it separate guards the MVP configuration semantics as
// more strategy-specific fields are introduced.
func validateSharedConfig(config Config) error {
	if isNaNOrInf(config.RecoveryEntryMinPeakDiscountPct) || config.RecoveryEntryMinPeakDiscountPct < 0 || config.RecoveryEntryMinPeakDiscountPct >= 100 {
		return fmt.Errorf("recovery Entry 1 minimum peak discount must be at least 0 and below 100")
	}
	if !validPositivePercentage(config.PullbackMinPct) {
		return fmt.Errorf("pullback minimum must be greater than 0 and at most 100")
	}
	if config.PivotLeft < 1 || config.PivotRight < 1 {
		return fmt.Errorf("pivot windows must both be at least 1")
	}
	if !finiteNonNegativePercentage(config.BreakoutBufferPct) {
		return fmt.Errorf("breakout buffer must be between 0 and 100")
	}
	if config.TrendMode != TrendModeStrict && config.TrendMode != TrendModeRecovery && config.TrendMode != TrendModeOff {
		return fmt.Errorf("trend mode must be STRICT, RECOVERY, or OFF")
	}

	entries := []struct {
		name  string
		value float64
	}{
		{"entry 1 percentage", config.Entry1Pct},
		{"entry 2 percentage", config.Entry2Pct},
		{"entry 3 percentage", config.Entry3Pct},
	}
	var entryTotal float64
	for _, entry := range entries {
		if !validPositivePercentage(entry.value) {
			return fmt.Errorf("%s must be greater than 0 and at most 100", entry.name)
		}
		entryTotal += entry.value
	}
	if entryTotal > 100 {
		return fmt.Errorf("entry percentages cannot total more than 100")
	}

	if !validPositivePercentage(config.ProfitTrigger1Pct) {
		return fmt.Errorf("profit trigger must be greater than 0 and at most 100")
	}
	if !validPositivePercentage(config.ProfitTakePct) {
		return fmt.Errorf("profit take percentage must be greater than 0 and at most 100")
	}

	drawdowns := []struct {
		name      string
		threshold float64
		sellPct   float64
	}{
		{"drawdown 1", config.Drawdown1Pct, config.Drawdown1SellPct},
		{"drawdown 2", config.Drawdown2Pct, config.Drawdown2SellPct},
		{"drawdown 3", config.Drawdown3Pct, config.Drawdown3SellPct},
	}
	for _, drawdown := range drawdowns {
		if !validNegativePercentage(drawdown.threshold) {
			return fmt.Errorf("%s threshold must be below 0 and no less than -100", drawdown.name)
		}
		if !validPositivePercentage(drawdown.sellPct) {
			return fmt.Errorf("%s sell percentage must be greater than 0 and at most 100", drawdown.name)
		}
	}
	if !(config.Drawdown1Pct > config.Drawdown2Pct && config.Drawdown2Pct > config.Drawdown3Pct) {
		return fmt.Errorf("drawdown thresholds must become progressively deeper")
	}
	return nil
}

func validateDipAccumulationConfig(config Config) error {
	if !validPositivePercentage(config.Entry2DipFromFirstPct) {
		return fmt.Errorf("entry 2 dip from first entry must be greater than 0 and at most 100")
	}
	if !validPositivePercentage(config.Entry3DipFromFirstPct) {
		return fmt.Errorf("entry 3 dip from first entry must be greater than 0 and at most 100")
	}
	if config.Entry3DipFromFirstPct <= config.Entry2DipFromFirstPct {
		return fmt.Errorf("entry 3 dip from first entry must be greater than entry 2 dip")
	}
	if isNaNOrInf(config.EstimatedSellFeeBps) || config.EstimatedSellFeeBps < 0 || config.EstimatedSellFeeBps >= 10000 {
		return fmt.Errorf("estimated sell fee must be at least 0 and below 10000 bps")
	}
	if !config.BreakEvenExitFloorEnabled {
		return fmt.Errorf("dip accumulation requires the break-even exit floor")
	}
	return nil
}
