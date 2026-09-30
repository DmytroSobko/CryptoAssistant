package strategy

import (
	"fmt"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

// evaluateDipAccumulationUnavailable is deliberately inert until Phase B
// implements its entry rules. The selector is not exposed by storage or the
// UI in Phase A, so this safe response prevents a partially configured future
// strategy from accidentally issuing a trade recommendation.
func evaluateDipAccumulationUnavailable(candles []market.Candle, position portfolio.Asset, config Config, previous PersistedState) (Result, PersistedState) {
	return unavailableStrategyResult(candles, position, config, previous, "Dip Accumulation is not available until its entry rules are implemented.")
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
