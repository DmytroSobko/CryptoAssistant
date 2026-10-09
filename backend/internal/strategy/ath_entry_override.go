package strategy

import (
	"fmt"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

// athEntryOverride supplies a one-time first entry for either strategy when a
// fresh cycle closes at or below the configured fraction of its historical
// peak. The caller is responsible for supplying a peak known before the
// latest candle, which keeps the helper free of storage and lookahead.
func athEntryOverride(candles []market.Candle, position portfolio.Asset, config Config, previous PersistedState) (Result, PersistedState, bool) {
	if !config.ATHEntryOverrideEnabled || !finitePositive(config.ATHReferencePeak) || !finitePositive(config.ATHEntryThresholdPct) || len(candles) == 0 {
		return Result{}, PersistedState{}, false
	}
	// EntryStep and PositionOpen define the position cycle. A normal strategy
	// can label a flat, never-entered cycle as CORRECTION or WAITING_FOR_REENTRY;
	// neither label should prevent this global first-entry rule from applying.
	if previous.EntryStep != 0 || previous.PositionOpen || position.Quantity > 0 {
		return Result{}, PersistedState{}, false
	}
	latest := candles[len(candles)-1]
	threshold := config.ATHReferencePeak * config.ATHEntryThresholdPct / 100
	if latest.Close > threshold {
		return Result{}, PersistedState{}, false
	}
	state := previous
	state.CurrentState = StatePartialPosition
	state.EntryStep = 1
	state.PositionOpen = true
	state.FirstEntryReferencePrice = latest.Close
	state.FirstEntryReferenceAt = latest.Timestamp
	state.LastDipEntryAt = latest.Timestamp
	result := Result{
		Asset: state.Asset, Action: ActionBuy40, ActionPct: config.Entry1Pct, State: state.CurrentState,
		Price: latest.Close, Trend: string(ClassifyTrend(candles)), PositionPct: config.Entry1Pct,
		Reason:        fmt.Sprintf("ATH entry override: completed daily close %.2f is at or below %.2f%% of the %.2f average from %d spaced prior peaks (threshold %.2f).", latest.Close, config.ATHEntryThresholdPct, config.ATHReferencePeak, config.ATHPeakCount, threshold),
		NextCondition: "After this first-entry advisory, follow the selected strategy's normal staged-entry rules.",
	}
	return result, state, true
}
