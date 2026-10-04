package backtest

import (
	"math"
	"testing"
)

func TestExecutionAppliesSizingFeesSlippageAndCostBasis(t *testing.T) {
	request := fixtureRequest(entryFixture(), 199, 200)
	request.StartingCashUSD, request.FeeBps, request.SlippageBps = 1000, 100, 100
	state := initialState("BTC", 1000)
	first := execute(PendingOrder{Action: "BUY", ActionPct: 40}, candleAt(200, 100), &state, request, 1)
	if first.Status != "EXECUTED" || math.Abs(first.GrossNotionalUSD-400) > 1e-9 || math.Abs(first.FeeUSD-4) > 1e-9 {
		t.Fatalf("first buy = %+v", first)
	}
	second := execute(PendingOrder{Action: "BUY", ActionPct: 30}, candleAt(201, 200), &state, request, 2)
	if second.Status != "EXECUTED" || state.Position.Quantity <= 0 || state.Position.AverageEntryPrice <= 0 {
		t.Fatalf("second buy = %+v state=%+v", second, state)
	}
	averageBeforeSell := state.Position.AverageEntryPrice
	partial := execute(PendingOrder{Action: "SELL_PROFIT", ActionPct: 25}, candleAt(202, 300), &state, request, 3)
	if partial.Status != "EXECUTED" || state.Position.AverageEntryPrice != averageBeforeSell {
		t.Fatalf("partial sell changed average entry: trade=%+v state=%+v", partial, state)
	}
	full := execute(PendingOrder{Action: "SELL_DRAWDOWN", ActionPct: 100}, candleAt(203, 300), &state, request, 4)
	if full.Status != "EXECUTED" || state.Position.Quantity != 0 || state.Position.AverageEntryPrice != 0 || state.Position.CashAllocated != 0 {
		t.Fatalf("full sell did not reset position: trade=%+v state=%+v", full, state)
	}
}

func TestEquityPointAndBuyAndHoldSummary(t *testing.T) {
	candles := entryFixture()
	state := initialState("BTC", 100)
	first := equityPoint(candles[199], state, 100)
	state.CashUSD, state.Position.Quantity = 0, 1
	second := equityPoint(candles[200], state, first.PeakEquityUSD)
	if second.EquityUSD != 100 || second.DrawdownPct != 0 {
		t.Fatalf("second equity point = %+v", second)
	}
	third := equityPoint(candleAt(201, 80), state, second.PeakEquityUSD)
	if third.DrawdownPct != -20 {
		t.Fatalf("drawdown = %v, want -20", third.DrawdownPct)
	}
}

func TestBuyBudgetCompoundsBetweenCyclesAndStaysFixedWithinCycle(t *testing.T) {
	for _, salePrice := range []float64{200, 50} {
		request := fixtureRequest(entryFixture(), 199, 200)
		request.FeeBps, request.SlippageBps = 10, 0
		state := initialState("ETH", 10000)
		first := execute(PendingOrder{Action: "BUY", ActionPct: 20}, candleAt(200, 100), &state, request, 1)
		second := execute(PendingOrder{Action: "BUY", ActionPct: 30}, candleAt(201, 100), &state, request, 2)
		if first.GrossNotionalUSD != 2000 || second.GrossNotionalUSD != 3000 {
			t.Fatalf("first-cycle buys must use the fixed 10000 budget: %+v / %+v", first, second)
		}
		sale := execute(PendingOrder{Action: "SELL_DRAWDOWN", ActionPct: 100}, candleAt(202, salePrice), &state, request, 3)
		if sale.Status != "EXECUTED" || state.Position.Quantity != 0 {
			t.Fatalf("cycle did not close: %+v", sale)
		}
		budget := state.CashUSD
		// Remaining cash plus net sale proceeds form the next fixed budget.
		wantBudget := 4995 + 50*salePrice*.999
		if math.Abs(budget-wantBudget) > 1e-9 {
			t.Fatalf("budget=%f want=%f", budget, wantBudget)
		}
		next := execute(PendingOrder{Action: "BUY", ActionPct: 20}, candleAt(203, 100), &state, request, 4)
		if math.Abs(next.GrossNotionalUSD-budget*.2) > 1e-9 || state.CycleBudgetUSD != budget {
			t.Fatalf("next cycle did not compound cash: budget=%f trade=%+v", budget, next)
		}
		// Partial sale proceeds must not change the current cycle budget.
		execute(PendingOrder{Action: "SELL_PROFIT", ActionPct: 25}, candleAt(204, 110), &state, request, 5)
		staged := execute(PendingOrder{Action: "BUY", ActionPct: 30}, candleAt(205, 100), &state, request, 6)
		if math.Abs(staged.GrossNotionalUSD-budget*.3) > 1e-9 || state.CycleBudgetUSD != budget {
			t.Fatalf("staged entry changed its cycle budget: budget=%f trade=%+v", budget, staged)
		}
		final := execute(PendingOrder{Action: "BUY", ActionPct: 50}, candleAt(206, 100), &state, request, 7)
		if math.Abs(final.GrossNotionalUSD-budget*.5) > 1e-9 || state.CashUSD < 0 {
			t.Fatalf("third entry failed fixed-budget sizing: budget=%f trade=%+v", budget, final)
		}
	}
}
