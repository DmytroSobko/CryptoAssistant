// Package strategy contains the pure deterministic strategy engine. It
// deliberately imports neither HTTP, SQLite, market providers, nor UI code.
package strategy

import (
	"fmt"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

type Action string

const (
	ActionHold           Action = "HOLD"
	ActionWait           Action = "WAIT"
	ActionBuy            Action = "BUY"
	ActionBuy40          Action = "BUY_40"
	ActionBuy30          Action = "BUY_30"
	ActionBuy30Final     Action = "BUY_30_FINAL"
	ActionSellProfit     Action = "SELL_PROFIT"
	ActionSellDrawdown   Action = "SELL_DRAWDOWN"
	ActionSellDrawdown10 Action = "SELL_DRAWDOWN_10" // retained for future API compatibility.
	ActionSellDrawdown15 Action = "SELL_DRAWDOWN_15" // retained for future API compatibility.
	ActionSellDrawdown20 Action = "SELL_DRAWDOWN_20" // retained for future API compatibility.
)

type State string

const (
	StateCash              State = "CASH"
	StateCorrection        State = "CORRECTION"
	StateRecoveryPending   State = "RECOVERY_PENDING"
	StateEntryPending      State = "ENTRY_PENDING"
	StatePartialPosition   State = "PARTIAL_POSITION"
	StateFullPosition      State = "FULL_POSITION"
	StateProfitProtection  State = "PROFIT_PROTECTION"
	StateExiting           State = "EXITING"
	StateWaitingForReentry State = "WAITING_FOR_REENTRY"
)

type Config struct {
	PullbackMinPct    float64 `json:"pullbackMinPct"`
	PivotLeft         int     `json:"pivotLeft"`
	PivotRight        int     `json:"pivotRight"`
	BreakoutBufferPct float64 `json:"breakoutBufferPct"`
	TrendMode         string  `json:"trendMode"`
	Entry1Pct         float64 `json:"entry1Pct"`
	Entry2Pct         float64 `json:"entry2Pct"`
	Entry3Pct         float64 `json:"entry3Pct"`
	ProfitTrigger1Pct float64 `json:"profitTrigger1Pct"`
	ProfitTrigger2Pct float64 `json:"profitTrigger2Pct"`
	ProfitTakePct     float64 `json:"profitTakePct"`
	Drawdown1Pct      float64 `json:"drawdown1Pct"`
	Drawdown1SellPct  float64 `json:"drawdown1SellPct"`
	Drawdown2Pct      float64 `json:"drawdown2Pct"`
	Drawdown2SellPct  float64 `json:"drawdown2SellPct"`
	Drawdown3Pct      float64 `json:"drawdown3Pct"`
	Drawdown3SellPct  float64 `json:"drawdown3SellPct"`
}

// PersistedState contains only facts needed to make a subsequent evaluation
// deterministic. Storage is intentionally not coupled to this type yet.
type PersistedState struct {
	Asset        string  `json:"asset"`
	CurrentState State   `json:"state"`
	LocalHigh    float64 `json:"localHigh"`
	LocalLow     float64 `json:"localLow"`
	HigherLow    float64 `json:"higherLow"`
	HighestPrice float64 `json:"highestPrice"`
	DrawdownPct  float64 `json:"drawdownPct"`

	CorrectionHighAt        time.Time `json:"correctionHighAt"`
	LastBreakoutHighAt      time.Time `json:"lastBreakoutHighAt"`
	LastBreakoutHigherLowAt time.Time `json:"lastBreakoutHigherLowAt"`
	ReentryAfter            time.Time `json:"reentryAfter"`
	EntryStep               int       `json:"entryStep"`
	ProfitTaken             bool      `json:"profitTaken"`
	Drawdown1Triggered      bool      `json:"drawdown1Triggered"`
	Drawdown2Triggered      bool      `json:"drawdown2Triggered"`
	Drawdown3Triggered      bool      `json:"drawdown3Triggered"`
	PositionOpen            bool      `json:"positionOpen"`
}

type Result struct {
	Asset               string  `json:"asset"`
	Action              Action  `json:"action"`
	ActionPct           float64 `json:"actionPct"`
	State               State   `json:"state"`
	Price               float64 `json:"price"`
	Trend               string  `json:"trend"`
	PositionPct         float64 `json:"positionPct"`
	ProfitLossPct       float64 `json:"profitLossPct"`
	PullbackPct         float64 `json:"pullbackPct"`
	LocalHigh           float64 `json:"localHigh"`
	LocalLow            float64 `json:"localLow"`
	DrawdownFromHighPct float64 `json:"drawdownFromHighPct"`
	Reason              string  `json:"reason"`
	NextCondition       string  `json:"nextCondition"`
}

// Engine is a small seam for callers that prefer dependency injection.
type Engine interface {
	Evaluate(candles []market.Candle, position portfolio.Asset, config Config, state PersistedState) (Result, PersistedState)
}

// Evaluate derives one advisory action from completed daily candles. It never
// mutates a portfolio: returned actions are recommendations, while the caller
// is responsible for persisting both the portfolio and next state.
//
// A staged entry consumes one distinct confirmed recovery breakout. This is
// deliberately stricter than treating every later close above the same level
// as a new entry; the latter would create duplicate daily recommendations.
func Evaluate(candles []market.Candle, position portfolio.Asset, config Config, previous PersistedState) (Result, PersistedState) {
	state := previous
	state.Asset = assetFor(position, state)
	if state.CurrentState == "" {
		state.CurrentState = StateCash
	}

	result := Result{Asset: state.Asset, Action: ActionWait, State: state.CurrentState, Trend: string(ClassifyTrend(candles))}
	if !validDailyCandles(candles) {
		result.Reason = "A valid completed daily candle is required."
		result.NextCondition = "Wait for completed daily market data."
		return result, state
	}
	price := candles[len(candles)-1].Close
	result.Price = price

	// A user-managed portfolio going flat after an observed position is a new
	// cycle. Re-entry must not reuse a breakout that preceded the exit.
	if state.PositionOpen && !hasPosition(position) {
		state = resetForReentry(state, candles[len(candles)-1].Timestamp)
		return waitingResult(result, state, config, "The previous position is closed.", "Wait for a new correction and confirmed recovery breakout."), state
	}

	if hasPosition(position) {
		state.PositionOpen = true
		result.ProfitLossPct = percentChange(position.AverageEntryPrice, price)
		if state.HighestPrice <= 0 || price > state.HighestPrice {
			state.HighestPrice = price
			state.Drawdown1Triggered = false
			state.Drawdown2Triggered = false
			state.Drawdown3Triggered = false
		}
		state.DrawdownPct = percentChange(state.HighestPrice, price)
		result.DrawdownFromHighPct = state.DrawdownPct

		if sellPct, level, triggered := drawdownAction(config, state.DrawdownPct, state); triggered {
			state = markDrawdownTriggered(state, level)
			state.CurrentState = StateExiting
			return actionResult(result, state, config, ActionSellDrawdown, sellPct,
				fmt.Sprintf("Completed daily close is %.2f%% below the %.2f high-water mark.", -state.DrawdownPct, state.HighestPrice),
				"Update the manually managed position after any sale."), state
		}

		if !state.ProfitTaken && finitePositive(position.AverageEntryPrice) && validPositivePercentage(config.ProfitTakePct) &&
			finitePositive(config.ProfitTrigger1Pct) && result.ProfitLossPct >= config.ProfitTrigger1Pct {
			state.ProfitTaken = true
			state.CurrentState = StateProfitProtection
			return actionResult(result, state, config, ActionSellProfit, config.ProfitTakePct,
				fmt.Sprintf("P/L is %.2f%%, at or above the configured %.2f%% profit trigger.", result.ProfitLossPct, config.ProfitTrigger1Pct),
				"Hold the remaining position and watch completed-candle drawdown exits."), state
		}
	}

	// Existing holdings with no strategy entry history receive only risk
	// management recommendations; the engine will not invent an entry stage.
	if hasPosition(position) && state.EntryStep == 0 {
		state.CurrentState = StatePartialPosition
		return holdResult(result, state, config, "A manually managed position is open.", "Watch profit protection and drawdown conditions."), state
	}
	if state.EntryStep >= 3 {
		if state.CurrentState != StateProfitProtection && state.CurrentState != StateExiting {
			state.CurrentState = StateFullPosition
		}
		return holdResult(result, state, config, "All configured entry stages have been signaled.", "Watch profit protection and drawdown conditions."), state
	}

	correction, anchorIndex, activeCorrection := correctionAnchor(candles, config, state)
	if !activeCorrection {
		state.CurrentState = StateWaitingForReentry
		if state.ReentryAfter.IsZero() {
			state.CurrentState = StateCash
		}
		return waitingResult(result, state, config, "No eligible completed-candle correction is active.", "Wait for the configured pullback from a confirmed local high."), state
	}
	state.LocalHigh = correction.LocalHigh.Price
	state.CorrectionHighAt = correction.LocalHigh.Timestamp
	result.LocalHigh = state.LocalHigh
	result.PullbackPct = correction.PullbackPct

	recovery, hasRecovery := FindRecovery(candles, config.PivotLeft, config.PivotRight, anchorIndex)
	if !hasRecovery {
		state.CurrentState = StateCorrection
		return waitingResult(result, state, config, "A correction is eligible, but a confirmed Higher Low has not formed.", "Wait for a completed-candle recovery structure."), state
	}
	state.LocalLow = recovery.HigherLow.InitialLow.Price
	state.HigherLow = recovery.HigherLow.Pivot.Price
	result.LocalLow = state.LocalLow

	breakout, hasBreakout := FindRecoveryBreakout(candles, config.PivotLeft, config.PivotRight, anchorIndex, config.BreakoutBufferPct)
	if !hasBreakout || !breakout.Confirmed {
		state.CurrentState = StateRecoveryPending
		return waitingResult(result, state, config, "A Higher Low is confirmed, but the recovery high has not been broken on a daily close.", "Wait for a completed close above the recovery high."), state
	}
	if !EntryAllowedByTrend(config.TrendMode, Trend(result.Trend), true) {
		state.CurrentState = StateEntryPending
		return waitingResult(result, state, config, "A recovery breakout is confirmed, but the trend mode does not allow entry.", "Wait for the configured trend filter to allow the breakout."), state
	}
	if sameTime(state.LastBreakoutHighAt, breakout.Recovery.RecoveryHigh.Timestamp) && sameTime(state.LastBreakoutHigherLowAt, breakout.Recovery.HigherLow.Pivot.Timestamp) {
		state.CurrentState = StatePartialPosition
		return holdResult(result, state, config, "This recovery breakout has already been used for an entry stage.", "Wait for a new confirmed recovery breakout."), state
	}

	entryPct := config.entryPct(state.EntryStep + 1)
	if !validPositivePercentage(entryPct) {
		return waitingResult(result, state, config, "The next entry percentage is invalid.", "Set a positive configured entry percentage."), state
	}
	state.EntryStep++
	state.LastBreakoutHighAt = breakout.Recovery.RecoveryHigh.Timestamp
	state.LastBreakoutHigherLowAt = breakout.Recovery.HigherLow.Pivot.Timestamp
	if state.EntryStep == 3 {
		state.CurrentState = StateFullPosition
	} else {
		state.CurrentState = StatePartialPosition
	}
	return actionResult(result, state, config, buyAction(state.EntryStep, entryPct), entryPct,
		fmt.Sprintf("Correction, Higher Low, recovery breakout, and %s trend eligibility are confirmed.", config.TrendMode),
		"Wait for a new confirmed recovery breakout before the next entry stage."), state
}

func correctionAnchor(candles []market.Candle, config Config, state PersistedState) (Correction, int, bool) {
	if !state.CorrectionHighAt.IsZero() {
		if !finitePositive(state.LocalHigh) {
			return Correction{}, 0, false
		}
		for index, candle := range candles {
			if sameTime(candle.Timestamp, state.CorrectionHighAt) {
				return Correction{LocalHigh: Pivot{Index: index, Timestamp: candle.Timestamp, Price: state.LocalHigh}, Close: candles[len(candles)-1].Close, PullbackPct: percentChange(state.LocalHigh, candles[len(candles)-1].Close)}, index, true
			}
		}
		return Correction{}, 0, false
	}
	correction, ok := FindCorrection(candles, config.PivotLeft, config.PivotRight, config.PullbackMinPct)
	if !ok || !correction.Eligible || (!state.ReentryAfter.IsZero() && !correction.LocalHigh.Timestamp.After(state.ReentryAfter)) {
		return Correction{}, 0, false
	}
	return correction, correction.LocalHigh.Index, true
}

func drawdownAction(config Config, drawdown float64, state PersistedState) (float64, int, bool) {
	// Select the deepest newly crossed level. Lower levels are consumed too so a
	// daily gap cannot produce delayed, out-of-order sales on later days.
	if validNegativePercentage(config.Drawdown3Pct) && validPositivePercentage(config.Drawdown3SellPct) && !state.Drawdown3Triggered && drawdown <= config.Drawdown3Pct {
		return config.Drawdown3SellPct, 3, true
	}
	if validNegativePercentage(config.Drawdown2Pct) && validPositivePercentage(config.Drawdown2SellPct) && !state.Drawdown2Triggered && drawdown <= config.Drawdown2Pct {
		return config.Drawdown2SellPct, 2, true
	}
	if validNegativePercentage(config.Drawdown1Pct) && validPositivePercentage(config.Drawdown1SellPct) && !state.Drawdown1Triggered && drawdown <= config.Drawdown1Pct {
		return config.Drawdown1SellPct, 1, true
	}
	return 0, 0, false
}

func markDrawdownTriggered(state PersistedState, level int) PersistedState {
	if level >= 1 {
		state.Drawdown1Triggered = true
	}
	if level >= 2 {
		state.Drawdown2Triggered = true
	}
	if level >= 3 {
		state.Drawdown3Triggered = true
	}
	return state
}

func actionResult(result Result, state PersistedState, config Config, action Action, pct float64, reason, next string) Result {
	result.Action = action
	result.ActionPct = pct
	result.State = state.CurrentState
	result.PositionPct = stagedPct(state.EntryStep, config)
	result.Reason = reason
	result.NextCondition = next
	return result
}

func holdResult(result Result, state PersistedState, config Config, reason, next string) Result {
	result.Action = ActionHold
	result.State = state.CurrentState
	result.PositionPct = stagedPct(state.EntryStep, config)
	result.Reason = reason
	result.NextCondition = next
	return result
}

func waitingResult(result Result, state PersistedState, config Config, reason, next string) Result {
	result.Action = ActionWait
	result.State = state.CurrentState
	result.PositionPct = stagedPct(state.EntryStep, config)
	result.Reason = reason
	result.NextCondition = next
	return result
}

func resetForReentry(state PersistedState, at time.Time) PersistedState {
	state.CurrentState = StateWaitingForReentry
	state.LocalHigh = 0
	state.LocalLow = 0
	state.HigherLow = 0
	state.HighestPrice = 0
	state.DrawdownPct = 0
	state.CorrectionHighAt = time.Time{}
	state.LastBreakoutHighAt = time.Time{}
	state.LastBreakoutHigherLowAt = time.Time{}
	state.ReentryAfter = at
	state.EntryStep = 0
	state.ProfitTaken = false
	state.Drawdown1Triggered = false
	state.Drawdown2Triggered = false
	state.Drawdown3Triggered = false
	state.PositionOpen = false
	return state
}

func (config Config) entryPct(step int) float64 {
	switch step {
	case 1:
		return config.Entry1Pct
	case 2:
		return config.Entry2Pct
	case 3:
		return config.Entry3Pct
	default:
		return 0
	}
}

func stagedPct(step int, config Config) float64 {
	var total float64
	for current := 1; current <= step; current++ {
		total += config.entryPct(current)
	}
	return total
}

func buyAction(step int, pct float64) Action {
	if pct == 40 && step == 1 {
		return ActionBuy40
	}
	if pct == 30 && step == 2 {
		return ActionBuy30
	}
	if pct == 30 && step == 3 {
		return ActionBuy30Final
	}
	return ActionBuy
}

func hasPosition(position portfolio.Asset) bool { return finitePositive(position.Quantity) }

func assetFor(position portfolio.Asset, state PersistedState) string {
	if position.Symbol != "" {
		return position.Symbol
	}
	return state.Asset
}

func validDailyCandles(candles []market.Candle) bool {
	for index, candle := range candles {
		if candle.Timestamp.IsZero() || !finitePositive(candle.Close) || (index > 0 && !candle.Timestamp.After(candles[index-1].Timestamp)) {
			return false
		}
	}
	return len(candles) > 0
}

func validPositivePercentage(value float64) bool {
	return value > 0 && value <= 100 && !isNaNOrInf(value)
}
func validNegativePercentage(value float64) bool {
	return value < 0 && value >= -100 && !isNaNOrInf(value)
}
func percentChange(base, value float64) float64 {
	if !finitePositive(base) || isNaNOrInf(value) {
		return 0
	}
	return (value - base) / base * 100
}
func sameTime(a, b time.Time) bool { return !a.IsZero() && !b.IsZero() && a.Equal(b) }
