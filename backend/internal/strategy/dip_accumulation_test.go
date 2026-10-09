package strategy

import (
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

func TestDipAccumulationUsesRecoveryForFirstEntryAndFixedReferenceForLaterEntries(t *testing.T) {
	config := dipAccumulationConfig()
	position := portfolio.Asset{Symbol: "BTC"}

	_, correctionState := Evaluate(candleFixture([]float64{100, 120, 100, 90}), position, config, PersistedState{})
	first, state := Evaluate(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 96}), position, config, correctionState)
	if first.Action != ActionBuy40 || state.EntryStep != 1 || state.FirstEntryReferencePrice != 96 || state.FirstEntryReferenceAt.IsZero() {
		t.Fatalf("first entry=%+v state=%+v; want Entry 1 with a 96 signal reference", first, state)
	}

	position = portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 96}
	noEarly, state := Evaluate(dipCloseFixture(90, state.FirstEntryReferenceAt.AddDate(0, 0, 1)), position, config, state)
	if noEarly.Action != ActionHold || state.EntryStep != 1 {
		t.Fatalf("early entry result=%+v state=%+v; Entry 2 must wait for 86.40", noEarly, state)
	}

	second, state := Evaluate(dipCloseFixture(86.4, state.FirstEntryReferenceAt.AddDate(0, 0, 1)), position, config, state)
	if second.Action != ActionBuy30 || second.ActionPct != 30 || state.EntryStep != 2 {
		t.Fatalf("second entry=%+v state=%+v; want Entry 2 at 10%% below 96", second, state)
	}

	third, state := Evaluate(dipCloseFixture(76.8, state.LastDipEntryAt.AddDate(0, 0, 1)), position, config, state)
	if third.Action != ActionBuy30Final || third.ActionPct != 30 || state.EntryStep != 3 || third.State != StateFullPosition {
		t.Fatalf("third entry=%+v state=%+v; want Entry 3 at 20%% below 96", third, state)
	}
}

func TestDipAccumulationGapThroughBothLevelsKeepsStageOrder(t *testing.T) {
	config := dipAccumulationConfig()
	state := PersistedState{Asset: "BTC", CurrentState: StatePartialPosition, EntryStep: 1, FirstEntryReferencePrice: 100}
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}

	second, state := Evaluate(candleFixture([]float64{100, 75}), position, config, state)
	if second.Action != ActionBuy30 || state.EntryStep != 2 {
		t.Fatalf("first gap action=%+v state=%+v; want Entry 2 before Entry 3", second, state)
	}
	sameCandle, state := Evaluate(candleFixture([]float64{100, 75}), position, config, state)
	if sameCandle.Action != ActionHold || state.EntryStep != 2 {
		t.Fatalf("same-candle action=%+v state=%+v; it must not create a second buy", sameCandle, state)
	}
	third, state := Evaluate(dipCloseFixture(75, state.LastDipEntryAt.AddDate(0, 0, 1)), position, config, state)
	if third.Action != ActionBuy30Final || state.EntryStep != 3 {
		t.Fatalf("second gap action=%+v state=%+v; want Entry 3 on a later completed candle", third, state)
	}
}

func TestDipAccumulationDoesNotInventReferenceForManualPosition(t *testing.T) {
	config := dipAccumulationConfig()
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}
	result, state := Evaluate(candleFixture([]float64{100, 90}), position, config, PersistedState{})
	if result.Action != ActionHold || state.EntryStep != 0 || state.FirstEntryReferencePrice != 0 {
		t.Fatalf("manual position result=%+v state=%+v; it must not invent dip entries", result, state)
	}
}

func TestDipAccumulationResetsReferenceAfterPositionCloses(t *testing.T) {
	config := dipAccumulationConfig()
	prior := PersistedState{Asset: "BTC", CurrentState: StateFullPosition, PositionOpen: true, EntryStep: 3, FirstEntryReferencePrice: 100, FirstEntryReferenceAt: candleFixture([]float64{100})[0].Timestamp}
	result, state := Evaluate(candleFixture([]float64{100, 90}), portfolio.Asset{Symbol: "BTC"}, config, prior)
	if result.Action != ActionWait || state.EntryStep != 0 || state.FirstEntryReferencePrice != 0 || !state.FirstEntryReferenceAt.IsZero() {
		t.Fatalf("closed position result=%+v state=%+v; reference must reset", result, state)
	}
}

func TestDipAccumulationUsesCompletedCloseForDipEntry(t *testing.T) {
	config := dipAccumulationConfig()
	state := PersistedState{Asset: "BTC", CurrentState: StatePartialPosition, EntryStep: 1, FirstEntryReferencePrice: 100}
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}
	candles := candleFixture([]float64{100, 95})
	candles[len(candles)-1].Low = 80
	result, next := Evaluate(candles, position, config, state)
	if result.Action != ActionHold || next.EntryStep != 1 {
		t.Fatalf("intraday low created a dip entry: result=%+v state=%+v", result, next)
	}
}

func TestDipAccumulationDefersDrawdownUntilProfitMilestone(t *testing.T) {
	config := dipAccumulationConfig()
	state := PersistedState{Asset: "BTC", CurrentState: StateFullPosition, EntryStep: 3, FirstEntryReferencePrice: 100, HighestPrice: 120, PositionOpen: true}
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}
	result, next := Evaluate(candleFixture([]float64{100, 102}), position, config, state)
	if result.Action != ActionHold || next.Drawdown2Triggered || next.ProfitTaken {
		t.Fatalf("pre-profit drawdown result=%+v state=%+v; want protected hold", result, next)
	}

	state.ProfitTaken = true
	result, next = Evaluate(candleFixture([]float64{100, 102}), position, config, state)
	if result.Action != ActionSellDrawdown || result.ActionPct != config.Drawdown2SellPct || !next.Drawdown2Triggered {
		t.Fatalf("post-profit drawdown result=%+v state=%+v; want the configured drawdown exit", result, next)
	}
}

func TestDipAccumulationIntradayDrawdownAlertDoesNotConsumeSellStage(t *testing.T) {
	config := dipAccumulationConfig()
	config.IntradayDrawdownAlertsEnabled = true
	state := PersistedState{Asset: "BTC", CurrentState: StateProfitProtection, EntryStep: 3, FirstEntryReferencePrice: 100, HighestPrice: 120, PositionOpen: true, ProfitTaken: true}
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}

	result, next := EvaluateWithIntradayPrice(candleFixture([]float64{100, 115}), position, config, state, 107)
	if result.Action != ActionSellDrawdown || !result.IntradayAlert || result.Price != 107 || next.Drawdown1Triggered || next.Drawdown2Triggered || next.Drawdown3Triggered {
		t.Fatalf("intraday alert result=%+v state=%+v; want a non-consuming first drawdown alert", result, next)
	}
}

func TestDipAccumulationBreakEvenFloorBlocksBelowCostDrawdownExit(t *testing.T) {
	config := dipAccumulationConfig()
	state := PersistedState{Asset: "BTC", CurrentState: StateFullPosition, EntryStep: 3, FirstEntryReferencePrice: 100, HighestPrice: 120, PositionOpen: true, ProfitTaken: true}
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}
	result, next := Evaluate(candleFixture([]float64{100, 99}), position, config, state)
	if result.Action != ActionHold || result.BreakEvenFloorNetPrice != 100 || result.BreakEvenFloorGrossPrice <= 100 || next.Drawdown1Triggered {
		t.Fatalf("below-cost drawdown result=%+v state=%+v; expected protected hold", result, next)
	}
}

func TestDipAccumulationDoesNotRearmDrawdownLevelsAtNewHigh(t *testing.T) {
	config := dipAccumulationConfig()
	state := PersistedState{
		Asset: "BTC", CurrentState: StateExiting, EntryStep: 3,
		FirstEntryReferencePrice: 100, HighestPrice: 120, PositionOpen: true,
		ProfitTaken: true, Drawdown1Triggered: true,
	}
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100}

	result, state := Evaluate(candleFixture([]float64{100, 125}), position, config, state)
	if result.Action != ActionHold || state.HighestPrice != 125 || !state.Drawdown1Triggered {
		t.Fatalf("new high re-armed a consumed drawdown level: result=%+v state=%+v", result, state)
	}

	result, state = Evaluate(candleFixture([]float64{100, 112.5}), position, config, state)
	if result.Action != ActionHold || !state.Drawdown1Triggered {
		t.Fatalf("consumed drawdown level triggered again: result=%+v state=%+v", result, state)
	}

	result, state = Evaluate(candleFixture([]float64{100, 106.25}), position, config, state)
	if result.Action != ActionSellDrawdown || !state.Drawdown2Triggered {
		t.Fatalf("next unused drawdown level did not trigger: result=%+v state=%+v", result, state)
	}
}

func TestReconcileRejectedProtectedDrawdownRestoresOnlyDrawdownTriggers(t *testing.T) {
	previous := PersistedState{HighestPrice: 120, Drawdown1Triggered: true}
	candidate := PersistedState{HighestPrice: 120, Drawdown1Triggered: true, Drawdown2Triggered: true, Drawdown3Triggered: true, CurrentState: StateExiting}
	got := ReconcileRejectedProtectedDrawdown(previous, candidate)
	if got.HighestPrice != 120 || !got.Drawdown1Triggered || got.Drawdown2Triggered || got.Drawdown3Triggered {
		t.Fatalf("reconciled state=%+v", got)
	}
}

func dipAccumulationConfig() Config {
	config := engineConfig(TrendModeOff)
	config.StrategyID = StrategyDipAccumulation
	config.Entry2DipFromFirstPct = 10
	config.Entry3DipFromFirstPct = 20
	config.EstimatedSellFeeBps = 10
	config.BreakEvenExitFloorEnabled = true
	return config
}

func TestDipAccumulationProfitTakesPriorityOverUnusedDipBuy(t *testing.T) {
	config := dipAccumulationConfig()
	config.ProfitTrigger1Pct = 50
	position := portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 60}
	state := PersistedState{EntryStep: 1, PositionOpen: true, FirstEntryReferencePrice: 100}
	// This close qualifies both for a 10% dip entry and 50% profit.
	result, state := Evaluate(candleFixture([]float64{100, 90}), position, config, state)
	if result.Action != ActionSellProfit || !state.ProfitTaken || state.EntryStep != 1 {
		t.Fatalf("dip buy took priority over profit: result=%+v state=%+v", result, state)
	}
	// Below Entry 2's threshold again, without a drawdown from the new high.
	result, state = Evaluate(candleFixture([]float64{100, 89}), position, config, state)
	if result.Action != ActionHold || result.State != StateProfitProtection || state.EntryStep != 1 {
		t.Fatalf("unused dip stage resumed after profit: result=%+v state=%+v", result, state)
	}
}

func dipCloseFixture(close float64, at time.Time) []market.Candle {
	candles := candleFixture([]float64{100, close})
	candles[len(candles)-1].Timestamp = at
	return candles
}
