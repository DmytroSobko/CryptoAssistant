package strategy

import (
	"strings"
	"testing"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

func TestNormalFirstEntryRequiresPeakDiscountAtSignalTime(t *testing.T) {
	for _, id := range []StrategyID{StrategyRecoveryBreakout, StrategyDipAccumulation} {
		t.Run(string(id), func(t *testing.T) {
			config := dipAccumulationConfig()
			config.StrategyID = id
			config.RecoveryEntryMinPeakDiscountPct = 20
			config.ATHReferencePeak = 120
			position := portfolio.Asset{Symbol: "BTC"}
			_, correction := Evaluate(candleFixture([]float64{100, 120, 100, 90}), position, config, PersistedState{})
			// The earlier correction was eligible, but recovery at 97 is above
			// the peak's 20% discount ceiling of 96.
			blocked, pending := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 97}), position, config, correction)
			if blocked.Action != ActionWait || blocked.State != StateEntryPending || pending.EntryStep != 0 || !pending.LastBreakoutHighAt.IsZero() {
				t.Fatalf("gate failed or consumed the breakout: result=%+v state=%+v", blocked, pending)
			}
			allowed, next := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96}), position, config, pending)
			if allowed.Action != ActionBuy40 || next.EntryStep != 1 {
				t.Fatalf("price at the ceiling must pass with recovery: result=%+v state=%+v", allowed, next)
			}
			config.ATHReferencePeak = 0
			missing, _ := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96}), position, config, correction)
			if missing.Action != ActionWait || !strings.Contains(missing.Reason, "known prior historical peak") {
				t.Fatalf("missing peak must block enabled gate: %+v", missing)
			}
			config.RecoveryEntryMinPeakDiscountPct = 0
			legacy, _ := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 97}), position, config, correction)
			if legacy.Action != ActionBuy40 {
				t.Fatalf("disabled gate changed legacy entry: %+v", legacy)
			}
		})
	}
}

func TestPeakDiscountExampleAndOverrideIndependence(t *testing.T) {
	config := dipAccumulationConfig()
	config.RecoveryEntryMinPeakDiscountPct = 20
	config.ATHReferencePeak = 70000
	state := PersistedState{}
	if _, blocked := recoveryEntryPeakGate(Result{Price: 56000}, config, &state); blocked {
		t.Fatal("56000 must meet a 20% discount from 70000")
	}
	if _, blocked := recoveryEntryPeakGate(Result{Price: 56001}, config, &state); !blocked {
		t.Fatal("56001 must exceed the recovery ceiling")
	}
	// The gate limits normal recovery entries, not the independent override.
	config.RecoveryEntryMinPeakDiscountPct = 50
	config.ATHEntryOverrideEnabled = true
	config.ATHEntryThresholdPct = 60
	entry, next := Evaluate(candleFixture([]float64{42000}), portfolio.Asset{Symbol: "BTC"}, config, PersistedState{})
	if entry.Action != ActionBuy40 || next.EntryStep != 1 || !strings.Contains(entry.Reason, "ATH entry override") {
		t.Fatalf("override behavior changed: result=%+v state=%+v", entry, next)
	}
}
