package strategy

import "testing"

func TestConfigResolvedStrategyIDDefaultsToRecoveryBreakout(t *testing.T) {
	if got := (Config{}).ResolvedStrategyID(); got != StrategyRecoveryBreakout {
		t.Fatalf("resolved strategy ID = %q; want %q", got, StrategyRecoveryBreakout)
	}
}

func TestAssetStrategySettingsActiveConfigUsesSelectedStrategy(t *testing.T) {
	recovery := engineConfig(TrendModeStrict)
	dip := engineConfig(TrendModeRecovery)
	dip.Entry2DipFromFirstPct = 10
	dip.Entry3DipFromFirstPct = 20
	dip.EstimatedSellFeeBps = 10
	dip.BreakEvenExitFloorEnabled = true

	settings := AssetStrategySettings{
		SelectedStrategyID: StrategyDipAccumulation,
		RecoveryBreakout:   recovery,
		DipAccumulation:    dip,
	}
	if err := settings.Validate(); err != nil {
		t.Fatalf("validate settings: %v", err)
	}
	active, err := settings.ActiveConfig()
	if err != nil {
		t.Fatalf("active config: %v", err)
	}
	if active.StrategyID != StrategyDipAccumulation || active.TrendMode != TrendModeRecovery {
		t.Fatalf("active config=%+v; want dip accumulation settings", active)
	}
	normalized, err := settings.Normalized()
	if err != nil || normalized.ATHEntryOverride.ThresholdPct != 60 || normalized.ATHEntryOverride.Enabled {
		t.Fatalf("ATH override default=%+v err=%v; want disabled at 60%%", normalized.ATHEntryOverride, err)
	}
}

func TestAssetStrategySettingsRejectsUnknownSelection(t *testing.T) {
	settings := AssetStrategySettings{SelectedStrategyID: "UNKNOWN"}
	if _, err := settings.ActiveConfig(); err == nil {
		t.Fatal("expected unknown selected strategy to be rejected")
	}
}
