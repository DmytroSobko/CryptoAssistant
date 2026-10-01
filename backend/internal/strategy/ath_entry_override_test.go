package strategy

import (
	"testing"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

func TestATHEntryOverrideCreatesOneFirstEntryPerFreshCycle(t *testing.T) {
	config := engineConfig(TrendModeStrict)
	config.ATHEntryOverrideEnabled = true
	config.ATHEntryThresholdPct = 60
	config.ATHReferencePeak = 100
	candles := candleFixture([]float64{70, 60})

	result, state := Evaluate(candles, portfolio.Asset{Symbol: "BTC"}, config, PersistedState{Asset: "BTC", CurrentState: StateCash})
	if result.Action != ActionBuy40 || result.ActionPct != 40 || state.EntryStep != 1 || !state.PositionOpen || state.FirstEntryReferencePrice != 60 {
		t.Fatalf("override entry result=%+v state=%+v", result, state)
	}
	if result.Reason == "" || result.Price != 60 {
		t.Fatalf("override must explain its completed-close decision: %+v", result)
	}

	result, repeated := Evaluate(candles, portfolio.Asset{Symbol: "BTC", Quantity: 0.5}, config, state)
	if result.Action == ActionBuy40 || repeated.EntryStep != 1 {
		t.Fatalf("override repeated in an open cycle: result=%+v state=%+v", result, repeated)
	}
}

func TestATHEntryOverrideWaitsWhenCloseIsAboveThreshold(t *testing.T) {
	config := engineConfig(TrendModeStrict)
	config.ATHEntryOverrideEnabled = true
	config.ATHEntryThresholdPct = 60
	config.ATHReferencePeak = 100
	result, state, applied := athEntryOverride(candleFixture([]float64{61}), portfolio.Asset{Symbol: "BTC"}, config, PersistedState{Asset: "BTC", CurrentState: StateCash})
	if applied || result.Action != "" || state.CurrentState != "" {
		t.Fatalf("above threshold unexpectedly applied override: result=%+v state=%+v applied=%v", result, state, applied)
	}
}

func TestATHEntryOverrideAppliesToAFlatReentryCycle(t *testing.T) {
	config := engineConfig(TrendModeStrict)
	config.ATHEntryOverrideEnabled = true
	config.ATHEntryThresholdPct = 60
	config.ATHReferencePeak = 100
	result, state := Evaluate(candleFixture([]float64{60}), portfolio.Asset{Symbol: "BTC"}, config, PersistedState{Asset: "BTC", CurrentState: StateWaitingForReentry})
	if result.Action != ActionBuy40 || state.EntryStep != 1 || !state.PositionOpen {
		t.Fatalf("flat re-entry cycle did not receive ATH Entry 1: result=%+v state=%+v", result, state)
	}
}
