package strategy

import (
	"math"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	valid := engineConfig(TrendModeStrict)
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{"valid", valid, true},
		{"explicit recovery breakout", func() Config { cfg := valid; cfg.StrategyID = StrategyRecoveryBreakout; return cfg }(), true},
		{"unknown strategy", func() Config { cfg := valid; cfg.StrategyID = "UNKNOWN"; return cfg }(), false},
		{"zero pivot", func() Config { cfg := valid; cfg.PivotLeft = 0; return cfg }(), false},
		{"unknown trend", func() Config { cfg := valid; cfg.TrendMode = "MAYBE"; return cfg }(), false},
		{"over allocated entries", func() Config { cfg := valid; cfg.Entry3Pct = 40; return cfg }(), false},
		{"reversed profit triggers", func() Config { cfg := valid; cfg.ProfitTrigger2Pct = 5; return cfg }(), false},
		{"shallow second drawdown", func() Config { cfg := valid; cfg.Drawdown2Pct = -5; return cfg }(), false},
		{"non-finite", func() Config { cfg := valid; cfg.ProfitTakePct = math.NaN(); return cfg }(), false},
		{"enabled ATH override missing threshold", func() Config { cfg := valid; cfg.ATHEntryOverrideEnabled = true; return cfg }(), false},
		{"enabled ATH override valid threshold", func() Config {
			cfg := valid
			cfg.ATHEntryOverrideEnabled = true
			cfg.ATHEntryThresholdPct = 60
			return cfg
		}(), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateConfig(test.config)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateConfig() error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestValidateDipAccumulationConfig(t *testing.T) {
	valid := engineConfig(TrendModeStrict)
	valid.StrategyID = StrategyDipAccumulation
	valid.Entry2DipFromFirstPct = 10
	valid.Entry3DipFromFirstPct = 20
	valid.EstimatedSellFeeBps = 10
	valid.BreakEvenExitFloorEnabled = true

	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{"valid", valid, true},
		{"second dip missing", func() Config { cfg := valid; cfg.Entry2DipFromFirstPct = 0; return cfg }(), false},
		{"third dip not deeper", func() Config { cfg := valid; cfg.Entry3DipFromFirstPct = 10; return cfg }(), false},
		{"negative sell fee", func() Config { cfg := valid; cfg.EstimatedSellFeeBps = -1; return cfg }(), false},
		{"floor disabled", func() Config { cfg := valid; cfg.BreakEvenExitFloorEnabled = false; return cfg }(), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateConfig(test.config)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateConfig() error = %v, valid = %v", err, test.valid)
			}
		})
	}
}
