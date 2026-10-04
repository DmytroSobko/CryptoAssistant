package backtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/ath"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

// Run replays only the requested period while retaining preceding candles for
// indicator warm-up. Signals are formed at each completed close and any order
// is filled at the following in-range daily open; it never mutates candles.
func Run(request Request, candles []market.Candle) (Result, error) {
	if err := ValidateRequest(request); err != nil {
		return Result{}, err
	}
	if err := ValidateCandles(candles); err != nil {
		return Result{}, err
	}
	startIndex, endIndex, err := rangeIndexes(candles, request.Start, request.End)
	if err != nil {
		return Result{}, err
	}

	result := Result{Request: request, Assumptions: assumptionsFor(request), DataFingerprint: fingerprint(request, candles[startIndex:endIndex+1])}
	state := initialState(request.Asset, request.StartingCashUSD)
	evaluationConfig := request.StrategyConfig
	// Backtests know the fee assumption used at execution. Use it for the
	// strategy's close-time break-even estimate as well; the next-open order is
	// still conditionally rejected if its actual fill falls below the floor.
	if evaluationConfig.ResolvedStrategyID() == strategy.StrategyDipAccumulation {
		evaluationConfig.EstimatedSellFeeBps = request.FeeBps
	}
	peak := request.StartingCashUSD
	tradeSequence := 0

	for index := startIndex; index <= endIndex; index++ {
		candle := candles[index]
		if state.PendingOrder != nil {
			order := *state.PendingOrder
			tradeSequence++
			trade := execute(order, candle, &state, request, tradeSequence)
			if order.BreakEvenFloorNetPrice > 0 && trade.Status == "REJECTED_BREAK_EVEN_FLOOR" {
				state.StrategyState = strategy.ReconcileRejectedProtectedDrawdown(order.PriorStrategyState, state.StrategyState)
			}
			if trade.Status == "EXECUTED" {
				state.StrategyState = strategy.ReconcileSimulatedFirstEntryFill(request.StrategyConfig, state.StrategyState, order.Action, trade.FillPrice, candle.Timestamp)
			}
			result.Trades = append(result.Trades, trade)
			result.Signals[order.SignalSequence-1].OrderStatus = trade.Status
			state.PendingOrder = nil
		}

		point := equityPoint(candle, state, peak)
		peak = point.PeakEquityUSD
		result.EquityCurve = append(result.EquityCurve, point)

		previousStrategyState := state.StrategyState
		if evaluationConfig.ATHEntryOverrideEnabled {
			peak, found, peakErr := ath.PeakBefore(request.Asset, candle.Timestamp)
			if peakErr != nil {
				return Result{}, peakErr
			}
			for _, prior := range candles[:index] {
				if prior.High > peak {
					peak, found = prior.High, true
				}
			}
			if found {
				evaluationConfig.ATHReferencePeak = peak
			} else {
				evaluationConfig.ATHReferencePeak = 0
			}
		}
		decision, nextState := strategy.Evaluate(candles[:index+1], state.Position, evaluationConfig, previousStrategyState)
		state.StrategyState = nextState
		signal := Signal{Sequence: len(result.Signals) + 1, Timestamp: candle.Timestamp, DecisionClose: candle.Close, Action: decision.Action, ActionPct: decision.ActionPct, StrategyState: decision.State, Reason: decision.Reason, NextCondition: decision.NextCondition, OrderStatus: "NO_ORDER", BreakEvenFloorNetPrice: decision.BreakEvenFloorNetPrice}
		result.Signals = append(result.Signals, signal)
		if actionable(decision.Action) {
			if index == endIndex {
				result.Signals[len(result.Signals)-1].OrderStatus = "UNEXECUTED_END_OF_DATA"
			} else {
				state.PendingOrder = &PendingOrder{SignalSequence: signal.Sequence, SignalTimestamp: candle.Timestamp, ExecuteAt: candles[index+1].Timestamp, Action: decision.Action, ActionPct: decision.ActionPct, State: decision.State, Reason: decision.Reason, BreakEvenFloorNetPrice: decision.BreakEvenFloorNetPrice, PriorStrategyState: previousStrategyState}
				result.Signals[len(result.Signals)-1].OrderStatus = "PENDING"
			}
		}
	}
	result.FinalState = state
	result.Summary = buildSummary(&result, candles, startIndex, endIndex)
	return result, nil
}

func assumptionsFor(request Request) Assumptions {
	assumptions := Assumptions{
		ExecutionModel: request.ExecutionModel, SignalTiming: "Evaluate after each completed UTC daily candle close.",
		FillTiming: "Assume-filled at the following UTC daily candle open.",
		BuySizing:  "ActionPct of cash available at the first buy of each position cycle; this budget stays fixed for staged entries and compounds prior net proceeds. Buys are capped by available cash including fees.",
		SellSizing: "ActionPct of quantity held at fill time, capped at available quantity.",
		FeeBps:     request.FeeBps, SlippageBps: request.SlippageBps, NoMarginOrBorrowing: true, TaxesIncluded: false,
		ProfitTrigger2Implementation: "Stored and validated by the strategy, but not a distinct current-engine sell rule.",
	}
	if request.StrategyConfig.ResolvedStrategyID() == strategy.StrategyDipAccumulation {
		assumptions.ReferencePricePolicy = "Dip Accumulation uses the actual simulated Entry 1 fill as its fixed reference price; live advisory uses its Entry 1 signal close."
		assumptions.BreakEvenExitFloor = "Dip Accumulation drawdown exits require next-open net sale proceeds after simulated fees to meet the remaining weighted-average entry price."
	} else {
		assumptions.ReferencePricePolicy = "Recovery Breakout does not use a fixed first-entry dip reference price."
		assumptions.BreakEvenExitFloor = "Not enabled for Recovery Breakout."
	}
	if request.StrategyConfig.ATHEntryOverrideEnabled {
		assumptions.ATHEntryOverride = fmt.Sprintf("Enabled: on a fresh cash cycle, one Entry 1 is signalled when the completed daily close is at or below %.2f%% of the highest daily high known before that day.", request.StrategyConfig.ATHEntryThresholdPct)
	} else {
		assumptions.ATHEntryOverride = "Historical-peak Entry 1 override disabled for this run."
	}
	return assumptions
}

func fingerprint(request Request, candles []market.Candle) string {
	payload := struct {
		SchemaVersion int             `json:"schemaVersion"`
		Asset         string          `json:"asset"`
		Start         string          `json:"start"`
		End           string          `json:"end"`
		Candles       []market.Candle `json:"candles"`
	}{SchemaVersion: 1, Asset: request.Asset, Start: request.Start.Format("2006-01-02T15:04:05Z"), End: request.End.Format("2006-01-02T15:04:05Z"), Candles: candles}
	bytes, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("canonical backtest fingerprint: %v", err))
	}
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:])
}
