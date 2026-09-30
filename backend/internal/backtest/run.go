package backtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

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
	if err := validateCandles(candles); err != nil {
		return Result{}, err
	}
	startIndex, endIndex, err := rangeIndexes(candles, request.Start, request.End)
	if err != nil {
		return Result{}, err
	}

	result := Result{Request: request, Assumptions: assumptionsFor(request), DataFingerprint: fingerprint(request, candles[startIndex:endIndex+1])}
	state := initialState(request.Asset, request.StartingCashUSD)
	peak := request.StartingCashUSD
	tradeSequence := 0

	for index := startIndex; index <= endIndex; index++ {
		candle := candles[index]
		if state.PendingOrder != nil {
			tradeSequence++
			trade := execute(*state.PendingOrder, candle, &state, request, tradeSequence)
			result.Trades = append(result.Trades, trade)
			result.Signals[state.PendingOrder.SignalSequence-1].OrderStatus = trade.Status
			state.PendingOrder = nil
		}

		point := equityPoint(candle, state, peak)
		peak = point.PeakEquityUSD
		result.EquityCurve = append(result.EquityCurve, point)

		decision, nextState := strategy.Evaluate(candles[:index+1], state.Position, request.StrategyConfig, state.StrategyState)
		state.StrategyState = nextState
		signal := Signal{Sequence: len(result.Signals) + 1, Timestamp: candle.Timestamp, DecisionClose: candle.Close, Action: decision.Action, ActionPct: decision.ActionPct, StrategyState: decision.State, Reason: decision.Reason, NextCondition: decision.NextCondition, OrderStatus: "NO_ORDER"}
		result.Signals = append(result.Signals, signal)
		if actionable(decision.Action) {
			if index == endIndex {
				result.Signals[len(result.Signals)-1].OrderStatus = "UNEXECUTED_END_OF_DATA"
			} else {
				state.PendingOrder = &PendingOrder{SignalSequence: signal.Sequence, SignalTimestamp: candle.Timestamp, ExecuteAt: candles[index+1].Timestamp, Action: decision.Action, ActionPct: decision.ActionPct, State: decision.State, Reason: decision.Reason}
				result.Signals[len(result.Signals)-1].OrderStatus = "PENDING"
			}
		}
	}
	result.FinalState = state
	result.Summary = buildSummary(&result, candles, startIndex, endIndex)
	return result, nil
}

func assumptionsFor(request Request) Assumptions {
	return Assumptions{
		ExecutionModel: request.ExecutionModel, SignalTiming: "Evaluate after each completed UTC daily candle close.",
		FillTiming: "Assume-filled at the following UTC daily candle open.",
		BuySizing:  "ActionPct of original starting cash, capped by available cash including fees.",
		SellSizing: "ActionPct of quantity held at fill time, capped at available quantity.",
		FeeBps:     request.FeeBps, SlippageBps: request.SlippageBps, NoMarginOrBorrowing: true, TaxesIncluded: false,
		ProfitTrigger2Implementation: "Stored and validated by the strategy, but not a distinct current-engine sell rule.",
	}
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
