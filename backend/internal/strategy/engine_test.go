package strategy

import (
	"testing"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

func TestEvaluateFixtureDrivenEntryTransitions(t *testing.T) {
	config := engineConfig(TrendModeOff)

	t.Run("cash waits until correction", func(t *testing.T) {
		result, state := Evaluate(candleFixture([]float64{100, 120, 100, 110}), portfolio.Asset{Symbol: "BTC"}, config, PersistedState{})
		if result.Action != ActionWait || result.State != StateCash || state.CurrentState != StateCash {
			t.Fatalf("result=%+v state=%+v; want CASH/WAIT", result, state)
		}
	})

	t.Run("correction then recovery then entry", func(t *testing.T) {
		correctionCandles := candleFixture([]float64{100, 120, 100, 90})
		result, state := Evaluate(correctionCandles, portfolio.Asset{Symbol: "BTC"}, config, PersistedState{})
		if result.Action != ActionWait || result.State != StateCorrection || state.LocalHigh != 120 {
			t.Fatalf("correction result=%+v state=%+v", result, state)
		}

		recoveryCandles := candleFixture([]float64{100, 120, 100, 80, 90, 85, 90})
		result, state = Evaluate(recoveryCandles, portfolio.Asset{Symbol: "BTC"}, config, state)
		if result.Action != ActionWait || result.State != StateRecoveryPending || state.LocalLow != 80 || state.HigherLow != 85 {
			t.Fatalf("recovery result=%+v state=%+v", result, state)
		}

		breakoutCandles := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96})
		result, state = Evaluate(breakoutCandles, portfolio.Asset{Symbol: "BTC"}, config, state)
		if result.Action != ActionBuy40 || result.ActionPct != 40 || result.State != StatePartialPosition || state.EntryStep != 1 {
			t.Fatalf("entry result=%+v state=%+v", result, state)
		}
	})

	t.Run("strict trend holds a valid breakout in entry pending", func(t *testing.T) {
		strictConfig := engineConfig(TrendModeStrict)
		_, prior := Evaluate(candleFixture([]float64{100, 120, 100, 90}), portfolio.Asset{Symbol: "BTC"}, strictConfig, PersistedState{})
		result, state := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96}), portfolio.Asset{Symbol: "BTC"}, strictConfig, prior)
		if result.Action != ActionWait || result.State != StateEntryPending || state.EntryStep != 0 {
			t.Fatalf("result=%+v state=%+v; want ENTRY_PENDING without SMA200 history", result, state)
		}
	})
}

func TestEvaluateStagesEntriesOnlyOnNewBreakoutStructures(t *testing.T) {
	config := engineConfig(TrendModeOff)
	candles := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96})
	position := portfolio.Asset{Symbol: "BTC"}

	_, prior := Evaluate(candleFixture([]float64{100, 120, 100, 90}), position, config, PersistedState{})
	first, state := Evaluate(candles, position, config, prior)
	if first.Action != ActionBuy40 || state.EntryStep != 1 {
		t.Fatalf("first entry=%+v state=%+v", first, state)
	}
	duplicate, duplicateState := Evaluate(candles, position, config, state)
	if duplicate.Action != ActionHold || duplicateState.EntryStep != 1 {
		t.Fatalf("duplicate breakout result=%+v state=%+v; must not issue another buy", duplicate, duplicateState)
	}

	secondStructure := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96, 110, 92, 105, 98, 112})
	second, state := Evaluate(secondStructure, position, config, duplicateState)
	if second.Action != ActionBuy30 || second.ActionPct != 30 || state.EntryStep != 2 {
		t.Fatalf("second entry=%+v state=%+v", second, state)
	}

	thirdStructure := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96, 110, 92, 105, 98, 112, 100, 110, 105, 115})
	third, state := Evaluate(thirdStructure, position, config, state)
	if third.Action != ActionBuy30Final || third.ActionPct != 30 || third.State != StateFullPosition || state.EntryStep != 3 || third.PositionPct != 100 {
		t.Fatalf("third entry=%+v state=%+v", third, state)
	}
}

func TestEvaluateUsesConfiguredEntryPercentage(t *testing.T) {
	config := engineConfig(TrendModeOff)
	config.Entry1Pct = 45
	position := portfolio.Asset{Symbol: "BTC"}
	_, prior := Evaluate(candleFixture([]float64{100, 120, 100, 90}), position, config, PersistedState{})
	result, state := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96}), position, config, prior)
	if result.Action != ActionBuy || result.ActionPct != 45 || result.PositionPct != 45 || state.EntryStep != 1 {
		t.Fatalf("configured entry was not preserved: result=%+v state=%+v", result, state)
	}
}

func TestEvaluateUsesCompletedCloseNotLatestCandleHigh(t *testing.T) {
	config := engineConfig(TrendModeOff)
	candles := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96})
	withHighSpike := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96})
	withHighSpike[len(withHighSpike)-1].High = 1000

	position := portfolio.Asset{Symbol: "BTC"}
	_, prior := Evaluate(candleFixture([]float64{100, 120, 100, 90}), position, config, PersistedState{})
	ordinary, _ := Evaluate(candles, position, config, prior)
	spike, _ := Evaluate(withHighSpike, position, config, prior)
	if ordinary.Action != ActionBuy40 || spike.Action != ordinary.Action || spike.ActionPct != ordinary.ActionPct {
		t.Fatalf("latest candle high changed a completed-close decision: ordinary=%+v spike=%+v", ordinary, spike)
	}
}

func TestEvaluateProfitAndDrawdownAreOneTimePerHighWaterCycle(t *testing.T) {
	config := engineConfig(TrendModeOff)
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}

	profit, state := Evaluate(candleFixture([]float64{100, 115}), position, config, PersistedState{EntryStep: 3})
	if profit.Action != ActionSellProfit || profit.ActionPct != 25 || profit.ProfitLossPct != 15 || !state.ProfitTaken || state.CurrentState != StateProfitProtection {
		t.Fatalf("profit result=%+v state=%+v", profit, state)
	}
	noRepeat, state := Evaluate(candleFixture([]float64{100, 115}), position, config, state)
	if noRepeat.Action != ActionHold || state.CurrentState != StateProfitProtection {
		t.Fatalf("profit action repeated: result=%+v state=%+v", noRepeat, state)
	}

	state = PersistedState{EntryStep: 3, PositionOpen: true, HighestPrice: 120, ProfitTaken: true}
	drawdown, state := Evaluate(candleFixture([]float64{100, 108}), position, config, state)
	if drawdown.Action != ActionSellDrawdown || drawdown.ActionPct != 20 || !state.Drawdown1Triggered || state.CurrentState != StateExiting {
		t.Fatalf("drawdown result=%+v state=%+v", drawdown, state)
	}
	noRepeat, state = Evaluate(candleFixture([]float64{100, 108}), position, config, state)
	if noRepeat.Action != ActionHold || !state.Drawdown1Triggered {
		t.Fatalf("drawdown action repeated: result=%+v state=%+v", noRepeat, state)
	}
	newHigh, state := Evaluate(candleFixture([]float64{100, 125}), position, config, state)
	if newHigh.Action != ActionHold || state.HighestPrice != 125 || state.Drawdown1Triggered {
		t.Fatalf("new high did not reset drawdown cycle: result=%+v state=%+v", newHigh, state)
	}
	drawdown, state = Evaluate(candleFixture([]float64{100, 112.5}), position, config, state)
	if drawdown.Action != ActionSellDrawdown || drawdown.ActionPct != 20 || !state.Drawdown1Triggered {
		t.Fatalf("reset drawdown cycle did not trigger: result=%+v state=%+v", drawdown, state)
	}
}

func TestEvaluateRequiresFreshStructureAfterPositionCloses(t *testing.T) {
	config := engineConfig(TrendModeOff)
	candles := candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96})
	prior := PersistedState{PositionOpen: true, EntryStep: 3, CurrentState: StateFullPosition}
	closed, state := Evaluate(candles, portfolio.Asset{Symbol: "BTC"}, config, prior)
	if closed.Action != ActionWait || closed.State != StateWaitingForReentry || state.EntryStep != 0 || state.ReentryAfter.IsZero() {
		t.Fatalf("closed position result=%+v state=%+v", closed, state)
	}
	reused, _ := Evaluate(candles, portfolio.Asset{Symbol: "BTC"}, config, state)
	if reused.Action != ActionWait || reused.State != StateWaitingForReentry {
		t.Fatalf("old breakout was reused after exit: %+v", reused)
	}
}

func engineConfig(mode string) Config {
	return Config{
		PullbackMinPct: 10, PivotLeft: 1, PivotRight: 1, TrendMode: mode,
		Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30,
		ProfitTrigger1Pct: 10, ProfitTrigger2Pct: 20, ProfitTakePct: 25,
		Drawdown1Pct: -10, Drawdown1SellPct: 20,
		Drawdown2Pct: -15, Drawdown2SellPct: 30,
		Drawdown3Pct: -20, Drawdown3SellPct: 70,
	}
}
