package strategy

import "fmt"

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
	SelectedStrategyID StrategyID `json:"selectedStrategyId"`
	RecoveryBreakout   Config     `json:"recoveryBreakout"`
	DipAccumulation    Config     `json:"dipAccumulation"`
}

// ActiveConfig returns a copy with the profile's selected ID applied. It
// avoids trusting an ID embedded in an inactive configuration payload.
func (settings AssetStrategySettings) ActiveConfig() (Config, error) {
	selected := settings.SelectedStrategyID
	if selected == "" {
		selected = StrategyRecoveryBreakout
	}
	if !validStrategyID(selected) {
		return Config{}, fmt.Errorf("unknown strategy ID %q", selected)
	}

	var config Config
	switch selected {
	case StrategyRecoveryBreakout:
		config = settings.RecoveryBreakout
	case StrategyDipAccumulation:
		config = settings.DipAccumulation
	}
	config.StrategyID = selected
	if err := ValidateConfig(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// Validate verifies both saved strategy configurations and the selected ID.
// It is intentionally separate from ActiveConfig so settings can be checked
// before they are written to persistence in a later phase.
func (settings AssetStrategySettings) Validate() error {
	selected := settings.SelectedStrategyID
	if selected == "" {
		selected = StrategyRecoveryBreakout
	}
	if !validStrategyID(selected) {
		return fmt.Errorf("unknown strategy ID %q", selected)
	}

	recovery := settings.RecoveryBreakout
	recovery.StrategyID = StrategyRecoveryBreakout
	if err := ValidateConfig(recovery); err != nil {
		return fmt.Errorf("validate recovery breakout settings: %w", err)
	}
	dip := settings.DipAccumulation
	dip.StrategyID = StrategyDipAccumulation
	if err := ValidateConfig(dip); err != nil {
		return fmt.Errorf("validate dip accumulation settings: %w", err)
	}
	return nil
}
