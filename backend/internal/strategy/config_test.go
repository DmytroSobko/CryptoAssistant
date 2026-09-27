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
		{"zero pivot", func() Config { cfg := valid; cfg.PivotLeft = 0; return cfg }(), false},
		{"unknown trend", func() Config { cfg := valid; cfg.TrendMode = "MAYBE"; return cfg }(), false},
		{"over allocated entries", func() Config { cfg := valid; cfg.Entry3Pct = 40; return cfg }(), false},
		{"reversed profit triggers", func() Config { cfg := valid; cfg.ProfitTrigger2Pct = 5; return cfg }(), false},
		{"shallow second drawdown", func() Config { cfg := valid; cfg.Drawdown2Pct = -5; return cfg }(), false},
		{"non-finite", func() Config { cfg := valid; cfg.ProfitTakePct = math.NaN(); return cfg }(), false},
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
