package backtest

import (
	"math"
	"testing"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func TestProtectedSellRejectsBelowBreakEvenFloor(t *testing.T) {
	tests := []struct {
		name        string
		feeBps      float64
		slippageBps float64
		open        float64
	}{
		{"fee only", 10, 0, 100},
		{"slippage only", 0, 100, 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := floorSimulationState()
			trade := execute(floorPendingOrder(50, 100), market.Candle{Open: test.open}, &state, Request{StartingCashUSD: 1000, FeeBps: test.feeBps, SlippageBps: test.slippageBps}, 1)
			if trade.Status != "REJECTED_BREAK_EVEN_FLOOR" || trade.BreakEvenFloorNetPrice != 100 {
				t.Fatalf("trade=%+v; expected a floor rejection", trade)
			}
			if state.Position.Quantity != 1 || state.Position.AverageEntryPrice != 100 || state.Position.CashAllocated != 100 || state.CashUSD != 0 {
				t.Fatalf("floor rejection mutated state: %+v", state)
			}
		})
	}
}

func TestProtectedSellExecutesAtOrAboveBreakEvenAndPreservesPartialCostBasis(t *testing.T) {
	state := floorSimulationState()
	trade := execute(floorPendingOrder(50, 100), market.Candle{Open: 101}, &state, Request{StartingCashUSD: 1000, FeeBps: 10}, 1)
	if trade.Status != "EXECUTED" || math.Abs(state.Position.Quantity-0.5) > 1e-10 || state.Position.AverageEntryPrice != 100 || math.Abs(state.Position.CashAllocated-50) > 1e-10 {
		t.Fatalf("protected partial sale=%+v state=%+v", trade, state)
	}
}

func TestProtectedFullSellResetsCostBasis(t *testing.T) {
	state := floorSimulationState()
	trade := execute(floorPendingOrder(100, 100), market.Candle{Open: 100}, &state, Request{StartingCashUSD: 1000}, 1)
	if trade.Status != "EXECUTED" || state.Position.Quantity != 0 || state.Position.AverageEntryPrice != 0 || state.Position.CashAllocated != 0 {
		t.Fatalf("protected full sale=%+v state=%+v", trade, state)
	}
}

func TestRunRejectsGapBelowBreakEvenFloorAndKeepsExitEligible(t *testing.T) {
	candles := dipFloorFixture()
	request := fixtureRequest(candles, 199, len(candles)-1)
	request.StrategyConfig = dipBacktestConfig()
	result, err := Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}

	var rejected Trade
	for _, trade := range result.Trades {
		if trade.Status == "REJECTED_BREAK_EVEN_FLOOR" {
			rejected = trade
			break
		}
	}
	if rejected.Sequence == 0 || rejected.Action != strategy.ActionSellDrawdown || rejected.BreakEvenFloorNetPrice <= 0 {
		t.Fatalf("expected protected rejected drawdown trade, got %+v", result.Trades)
	}
	if result.Summary.BreakEvenFloorBlockedTradeCount != 1 || result.Summary.RejectedTradeCount != 1 || result.FinalState.Position.Quantity <= 0 {
		t.Fatalf("unexpected protected-exit summary/state: %+v / %+v", result.Summary, result.FinalState)
	}
	if result.FinalState.StrategyState.Drawdown1Triggered || math.Abs(result.FinalState.StrategyState.FirstEntryReferencePrice-100.05) > 1e-9 {
		t.Fatalf("rejected exit consumed protection or Entry 1 reference was not reconciled: %+v", result.FinalState.StrategyState)
	}
	if result.Assumptions.BreakEvenExitFloor == "" || result.Assumptions.ReferencePricePolicy == "" {
		t.Fatalf("floor assumptions were not saved: %+v", result.Assumptions)
	}
}

func floorSimulationState() SimulationState {
	return SimulationState{Position: portfolio.Asset{Symbol: "BTC", Quantity: 1, AverageEntryPrice: 100, CashAllocated: 100}}
}

func floorPendingOrder(pct, floor float64) PendingOrder {
	return PendingOrder{Action: strategy.ActionSellDrawdown, ActionPct: pct, BreakEvenFloorNetPrice: floor}
}

func dipFloorFixture() []market.Candle {
	candles := make([]market.Candle, 0, 216)
	for index := 0; index < 200; index++ {
		candles = append(candles, candleAt(index, 100))
	}
	for _, price := range []float64{100, 120, 100, 80, 90, 85, 95, 100, 100, 90, 90, 80, 80, 110, 99, 90} {
		candles = append(candles, candleAt(len(candles), price))
	}
	return candles
}

func dipBacktestConfig() strategy.Config {
	config := testConfig()
	config.StrategyID = strategy.StrategyDipAccumulation
	config.Entry2DipFromFirstPct = 10
	config.Entry3DipFromFirstPct = 20
	config.EstimatedSellFeeBps = 10
	config.BreakEvenExitFloorEnabled = true
	return config
}
