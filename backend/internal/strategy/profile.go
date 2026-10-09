package strategy

import "fmt"

type ATHEntryOverrideSettings struct {
	Enabled      bool    `json:"enabled"`
	ThresholdPct float64 `json:"thresholdPct"`
	SourceFile   string  `json:"sourceFile"`
	PeakCount    int     `json:"peakCount"`
}

// ResolvedStrategyID maps configurations created before strategy selection to
// the unchanged MVP ruleset. Callers that persist settings should use an
// explicit ID; this method exists so old saved JSON remains valid.
func (config Config) ResolvedStrategyID() StrategyID {
	if config.StrategyID == "" {
		return StrategyRecoveryBreakout
	}
	return config.StrategyID
}

func validStrategyID(id StrategyID) bool {
	switch id {
	case StrategyRecoveryBreakout, StrategyDipAccumulation:
		return true
	default:
		return false
	}
}

// AssetStrategySettings is the per-asset profile shape that storage and the
// API will persist in a later phase. Defining and validating it here keeps
// strategy choice entirely inside the pure domain layer.
type AssetStrategySettings struct {
	SelectedStrategyID StrategyID               `json:"selectedStrategyId"`
	RecoveryBreakout   Config                   `json:"recoveryBreakout"`
	DipAccumulation    Config                   `json:"dipAccumulation"`
	ATHEntryOverride   ATHEntryOverrideSettings `json:"athEntryOverride"`
}

// ActiveConfig returns a copy with the profile's selected ID applied. It
// avoids trusting an ID embedded in an inactive configuration payload.
func (settings AssetStrategySettings) ActiveConfig() (Config, error) {
	normalized, err := settings.Normalized()
	if err != nil {
		return Config{}, err
	}

	var config Config
	switch normalized.SelectedStrategyID {
	case StrategyRecoveryBreakout:
		config = normalized.RecoveryBreakout
	case StrategyDipAccumulation:
		config = normalized.DipAccumulation
	}
	return config, nil
}

// Validate verifies both saved strategy configurations and the selected ID.
// It is intentionally separate from ActiveConfig so settings can be checked
// before they are written to persistence in a later phase.
func (settings AssetStrategySettings) Validate() error {
	_, err := settings.Normalized()
	return err
}

// Normalized makes the selected strategy and each nested configuration
// explicit. This is the one representation storage and API callers persist,
// while still accepting old Recovery Breakout JSON that had no strategy ID.
func (settings AssetStrategySettings) Normalized() (AssetStrategySettings, error) {
	selected := settings.SelectedStrategyID
	if selected == "" {
		selected = StrategyRecoveryBreakout
	}
	if !validStrategyID(selected) {
		return AssetStrategySettings{}, fmt.Errorf("unknown strategy ID %q", selected)
	}

	recovery := settings.RecoveryBreakout
	recovery.StrategyID = StrategyRecoveryBreakout
	if err := ValidateConfig(recovery); err != nil {
		return AssetStrategySettings{}, fmt.Errorf("validate recovery breakout settings: %w", err)
	}
	dip := settings.DipAccumulation
	dip.StrategyID = StrategyDipAccumulation
	if err := ValidateConfig(dip); err != nil {
		return AssetStrategySettings{}, fmt.Errorf("validate dip accumulation settings: %w", err)
	}
	override := settings.ATHEntryOverride
	if override.ThresholdPct == 0 {
		override.ThresholdPct = 60
	}
	if !validPositivePercentage(override.ThresholdPct) {
		return AssetStrategySettings{}, fmt.Errorf("ATH entry override threshold must be greater than 0 and at most 100")
	}
	if override.PeakCount == 0 {
		override.PeakCount = 3
	}
	if override.PeakCount < 1 || override.PeakCount > 20 {
		return AssetStrategySettings{}, fmt.Errorf("historical peak count must be between 1 and 20")
	}
	return AssetStrategySettings{SelectedStrategyID: selected, RecoveryBreakout: recovery, DipAccumulation: dip, ATHEntryOverride: override}, nil
}
