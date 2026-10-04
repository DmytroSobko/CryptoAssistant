package backtest

import (
	"fmt"
	"math"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

const epsilon = 1e-10

func actionable(action strategy.Action) bool {
	_, ok := actionSide(action)
	return ok
}

func actionSide(action strategy.Action) (string, bool) {
	switch action {
	case strategy.ActionBuy, strategy.ActionBuy40, strategy.ActionBuy30, strategy.ActionBuy30Final:
		return "BUY", true
	case strategy.ActionSellProfit, strategy.ActionSellDrawdown, strategy.ActionSellDrawdown10, strategy.ActionSellDrawdown15, strategy.ActionSellDrawdown20:
		return "SELL", true
	default:
		return "", false
	}
}

func execute(order PendingOrder, candle market.Candle, state *SimulationState, request Request, sequence int) Trade {
	side, _ := actionSide(order.Action)
	trade := Trade{
		Sequence: sequence, SignalSequence: order.SignalSequence, SignalTimestamp: order.SignalTimestamp,
		ExecutionTimestamp: candle.Timestamp, Side: side, Action: order.Action, RequestedPct: order.ActionPct,
		RawOpenPrice: candle.Open, CashAfterUSD: state.CashUSD, QuantityAfter: state.Position.Quantity,
		AverageEntryAfter:      state.Position.AverageEntryPrice,
		BreakEvenFloorNetPrice: order.BreakEvenFloorNetPrice,
	}
	slippage := request.SlippageBps / 10000
	feeRate := request.FeeBps / 10000
	if side == "BUY" {
		trade.FillPrice = candle.Open * (1 + slippage)
		return executeBuy(trade, state, feeRate)
	}
	trade.FillPrice = candle.Open * (1 - slippage)
	return executeSell(trade, state, feeRate)
}

func executeBuy(trade Trade, state *SimulationState, feeRate float64) Trade {
	cycleBudget := state.CycleBudgetUSD
	if state.Position.Quantity <= epsilon {
		cycleBudget = state.CashUSD
	}
	requestedNotional := cycleBudget * trade.RequestedPct / 100
	maxGross := state.CashUSD / (1 + feeRate)
	gross := math.Min(requestedNotional, maxGross)
	if gross <= epsilon || trade.FillPrice <= epsilon {
		trade.Status = "REJECTED_INSUFFICIENT_CASH"
		trade.Reason = "No cash is available for the simulated buy, including fees."
		return trade
	}
	quantity := gross / trade.FillPrice
	state.CycleBudgetUSD = cycleBudget
	fee := gross * feeRate
	previousCost := state.Position.CashAllocated
	state.CashUSD -= gross + fee
	state.Position.Quantity += quantity
	state.Position.CashAllocated = previousCost + gross + fee
	state.Position.AverageEntryPrice = state.Position.CashAllocated / state.Position.Quantity
	normalizeState(state)
	trade.Status, trade.Reason = "EXECUTED", "Simulated buy filled at the next daily open after slippage and fees."
	trade.Quantity, trade.GrossNotionalUSD, trade.FeeUSD = quantity, gross, fee
	trade.CashAfterUSD, trade.QuantityAfter, trade.AverageEntryAfter = state.CashUSD, state.Position.Quantity, state.Position.AverageEntryPrice
	return trade
}

func executeSell(trade Trade, state *SimulationState, feeRate float64) Trade {
	if state.Position.Quantity <= epsilon || trade.FillPrice <= epsilon {
		trade.Status = "REJECTED_INSUFFICIENT_QUANTITY"
		trade.Reason = "No asset quantity is available for the simulated sale."
		return trade
	}
	quantity := math.Min(state.Position.Quantity*trade.RequestedPct/100, state.Position.Quantity)
	if quantity <= epsilon {
		trade.Status = "REJECTED_INSUFFICIENT_QUANTITY"
		trade.Reason = "The simulated sale quantity is zero."
		return trade
	}
	if trade.BreakEvenFloorNetPrice > epsilon && trade.FillPrice*(1-feeRate) < trade.BreakEvenFloorNetPrice-epsilon {
		trade.Status = "REJECTED_BREAK_EVEN_FLOOR"
		trade.Reason = fmt.Sprintf("Simulated net sale price %.8f is below the break-even floor %.8f.", trade.FillPrice*(1-feeRate), trade.BreakEvenFloorNetPrice)
		return trade
	}
	gross := quantity * trade.FillPrice
	fee := gross * feeRate
	priorQuantity := state.Position.Quantity
	state.CashUSD += gross - fee
	state.Position.Quantity -= quantity
	if state.Position.Quantity <= epsilon {
		state.Position.Quantity, state.Position.AverageEntryPrice, state.Position.CashAllocated = 0, 0, 0
	} else {
		state.Position.CashAllocated *= state.Position.Quantity / priorQuantity
	}
	normalizeState(state)
	trade.Status, trade.Reason = "EXECUTED", "Simulated sale filled at the next daily open after slippage and fees."
	trade.Quantity, trade.GrossNotionalUSD, trade.FeeUSD = quantity, gross, fee
	trade.CashAfterUSD, trade.QuantityAfter, trade.AverageEntryAfter = state.CashUSD, state.Position.Quantity, state.Position.AverageEntryPrice
	return trade
}

func normalizeState(state *SimulationState) {
	if math.Abs(state.CashUSD) <= epsilon {
		state.CashUSD = 0
	}
	if math.Abs(state.Position.Quantity) <= epsilon {
		state.Position.Quantity, state.Position.AverageEntryPrice, state.Position.CashAllocated = 0, 0, 0
	}
	if state.CashUSD < -epsilon || state.Position.Quantity < -epsilon {
		panic(fmt.Sprintf("backtest invariant violated: negative balance cash=%f quantity=%f", state.CashUSD, state.Position.Quantity))
	}
}

func initialState(asset string, cash float64) SimulationState {
	return SimulationState{CashUSD: cash, CycleBudgetUSD: cash, Position: portfolio.Asset{Symbol: asset}, StrategyState: strategy.PersistedState{Asset: asset}}
}
