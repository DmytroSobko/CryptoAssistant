package strategy

import (
	"fmt"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

// evaluateDipAccumulation uses the same confirmed recovery-breakout gate as
// Recovery Breakout for Entry 1. Later entries deliberately use fixed dips
// from that first-entry reference rather than requiring more breakouts.
func evaluateDipAccumulation(candles []market.Candle, position portfolio.Asset, config Config, previous PersistedState) (Result, PersistedState) {
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
	latest := candles[len(candles)-1]
	price := latest.Close
	result.Price = price

	// A user-managed portfolio going flat after an observed position ends the
	// current dip cycle; the next Entry 1 must be a fresh recovery breakout.
	if state.PositionOpen && !hasPosition(position) {
		state = resetForReentry(state, latest.Timestamp)
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

		// Accumulation gets first priority while a configured dip level remains
		// available. Without this order, a legacy drawdown exit at the same
		// daily close would prevent the intended average-down entry.
		if entryPct, entryStep, threshold, ready := dipEntryAction(config, state, price, latest.Timestamp); ready {
			state.EntryStep = entryStep
			state.LastDipEntryAt = latest.Timestamp
			if state.EntryStep == 3 {
				state.CurrentState = StateFullPosition
			} else {
				state.CurrentState = StatePartialPosition
			}
			nextCondition := "Watch profit protection and completed-candle drawdown exits."
			if entryStep < 3 {
				nextCondition = fmt.Sprintf("Wait for a completed daily close at or below %.2f for the next configured dip entry.", nextDipThreshold(config, state))
			}
			return actionResult(result, state, config, buyAction(entryStep, entryPct), entryPct,
				fmt.Sprintf("Completed daily close %.2f reached the Entry %d dip level of %.2f, %.2f%% below the %.2f first-entry signal reference.", price, entryStep, threshold, dipForStep(config, entryStep), state.FirstEntryReferencePrice),
				nextCondition), state
		}

		if sellPct, level, triggered := drawdownAction(config, state.DrawdownPct, state); triggered {
			floorNet, floorGross := dipBreakEvenFloor(config, position.AverageEntryPrice)
			if floorNet > 0 && price*(1-config.EstimatedSellFeeBps/10000) < floorNet {
				state.CurrentState = dipHoldingState(state)
				result.BreakEvenFloorNetPrice = floorNet
				result.BreakEvenFloorGrossPrice = floorGross
				return holdResult(result, state, config,
					fmt.Sprintf("Drawdown exit withheld: the %.2f completed close is below the estimated break-even sale floor of %.2f.", price, floorGross),
					fmt.Sprintf("Hold until a completed close supports a sale at or above %.2f before fees.", floorGross)), state
			}
			state = markDrawdownTriggered(state, level)
			state.CurrentState = StateExiting
			result = actionResult(result, state, config, ActionSellDrawdown, sellPct,
				fmt.Sprintf("Completed daily close is %.2f%% below the %.2f high-water mark.", -state.DrawdownPct, state.HighestPrice),
				"Update the manually managed position after any sale.")
			result.BreakEvenFloorNetPrice = floorNet
			result.BreakEvenFloorGrossPrice = floorGross
			return result, state
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

	// Existing user-managed holdings receive only the established exit advice;
	// the strategy cannot invent a first-entry reference for them.
	if hasPosition(position) && state.EntryStep == 0 {
		state.CurrentState = StatePartialPosition
		return holdResult(result, state, config, "A manually managed position is open.", "Watch profit protection and drawdown conditions."), state
	}
	if state.EntryStep >= 3 {
		if state.CurrentState != StateProfitProtection && state.CurrentState != StateExiting {
			state.CurrentState = StateFullPosition
		}
		return holdResult(result, state, config, "All configured dip entry stages have been signaled.", "Watch profit protection and drawdown conditions."), state
	}
	if state.EntryStep > 0 && !hasPosition(position) {
		state.CurrentState = StatePartialPosition
		return waitingResult(result, state, config, "An entry recommendation is awaiting a manually updated position.", "Update the position after the entry before evaluating another staged buy."), state
	}
	if hasPosition(position) && state.EntryStep > 0 {
		state.CurrentState = StatePartialPosition
		return holdResult(result, state, config, "A dip entry is active and the next configured dip level has not been reached.", fmt.Sprintf("Wait for a completed daily close at or below %.2f for the next configured dip entry.", nextDipThreshold(config, state))), state
	}

	return dipFirstEntry(result, candles, config, state)
}

func dipFirstEntry(result Result, candles []market.Candle, config Config, state PersistedState) (Result, PersistedState) {
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
		return holdResult(result, state, config, "This recovery breakout has already been used for an entry stage.", "Wait for a manually updated position before the configured dip entries."), state
	}

	entryPct := config.entryPct(1)
	if !validPositivePercentage(entryPct) {
		return waitingResult(result, state, config, "The first entry percentage is invalid.", "Set a positive configured first entry percentage."), state
	}
	state.EntryStep = 1
	state.FirstEntryReferencePrice = result.Price
	state.FirstEntryReferenceAt = candles[len(candles)-1].Timestamp
	state.LastDipEntryAt = candles[len(candles)-1].Timestamp
	state.LastBreakoutHighAt = breakout.Recovery.RecoveryHigh.Timestamp
	state.LastBreakoutHigherLowAt = breakout.Recovery.HigherLow.Pivot.Timestamp
	state.CurrentState = StatePartialPosition
	return actionResult(result, state, config, buyAction(1, entryPct), entryPct,
		fmt.Sprintf("Correction, Higher Low, recovery breakout, and %s trend eligibility are confirmed. The %.2f completed close is the fixed first-entry signal reference.", config.TrendMode, state.FirstEntryReferencePrice),
		fmt.Sprintf("After the position is updated, wait for a completed daily close at or below %.2f for Entry 2.", nextDipThreshold(config, state))), state
}

func dipEntryAction(config Config, state PersistedState, price float64, at time.Time) (entryPct float64, entryStep int, threshold float64, ready bool) {
	if !finitePositive(state.FirstEntryReferencePrice) {
		return 0, 0, 0, false
	}
	if sameTime(state.LastDipEntryAt, at) {
		return 0, 0, 0, false
	}
	nextStep := state.EntryStep + 1
	if nextStep < 2 || nextStep > 3 {
		return 0, 0, 0, false
	}
	threshold = state.FirstEntryReferencePrice * (1 - dipForStep(config, nextStep)/100)
	if price > threshold {
		return 0, 0, threshold, false
	}
	entryPct = config.entryPct(nextStep)
	if !validPositivePercentage(entryPct) {
		return 0, 0, threshold, false
	}
	return entryPct, nextStep, threshold, true
}

func nextDipThreshold(config Config, state PersistedState) float64 {
	nextStep := state.EntryStep + 1
	if !finitePositive(state.FirstEntryReferencePrice) || nextStep < 2 || nextStep > 3 {
		return 0
	}
	return state.FirstEntryReferencePrice * (1 - dipForStep(config, nextStep)/100)
}

func dipForStep(config Config, step int) float64 {
	switch step {
	case 2:
		return config.Entry2DipFromFirstPct
	case 3:
		return config.Entry3DipFromFirstPct
	default:
		return 0
	}
}

func dipBreakEvenFloor(config Config, averageEntryPrice float64) (netFloor, grossFloor float64) {
	if !config.BreakEvenExitFloorEnabled || !finitePositive(averageEntryPrice) {
		return 0, 0
	}
	feeRate := config.EstimatedSellFeeBps / 10000
	if feeRate < 0 || feeRate >= 1 {
		return 0, 0
	}
	return averageEntryPrice, averageEntryPrice / (1 - feeRate)
}

func dipHoldingState(state PersistedState) State {
	if state.EntryStep >= 3 {
		return StateFullPosition
	}
	return StatePartialPosition
}

func unsupportedStrategyResult(candles []market.Candle, position portfolio.Asset, config Config, previous PersistedState) (Result, PersistedState) {
	return unavailableStrategyResult(candles, position, config, previous, fmt.Sprintf("Strategy %q is not supported.", config.StrategyID))
}

func unavailableStrategyResult(candles []market.Candle, position portfolio.Asset, _ Config, previous PersistedState, reason string) (Result, PersistedState) {
	state := previous
	state.Asset = assetFor(position, state)
	if state.CurrentState == "" {
		state.CurrentState = StateCash
	}
	result := Result{Asset: state.Asset, Action: ActionWait, State: state.CurrentState, Trend: string(ClassifyTrend(candles)), Reason: reason, NextCondition: "Select Recovery Breakout until this strategy is available."}
	if validDailyCandles(candles) {
		result.Price = candles[len(candles)-1].Close
	}
	return result, state
}
