package strategy

import "github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"

// HigherLow is a confirmed pair of consecutive local lows where the second
// low is strictly higher than the first. Both pivots have complete windows,
// so a higher low is never inferred from an unconfirmed right-edge candle.
type HigherLow struct {
	InitialLow Pivot `json:"initialLow"`
	Pivot      Pivot `json:"pivot"`
}

// Recovery describes the confirmed price structure required before an entry
// breakout can be considered. RecoveryHigh is the latest confirmed local high
// between the initial low and the higher low.
type Recovery struct {
	HigherLow    HigherLow `json:"higherLow"`
	RecoveryHigh Pivot     `json:"recoveryHigh"`
}

// Breakout reports whether the latest completed daily close has exceeded the
// recovery high after the configured buffer. It is a confirmation fact, not a
// buy action.
type Breakout struct {
	Recovery       Recovery `json:"recovery"`
	Close          float64  `json:"close"`
	ThresholdPrice float64  `json:"thresholdPrice"`
	Confirmed      bool     `json:"confirmed"`
}

// LatestHigherLow returns the newest pair of consecutive confirmed local lows
// that forms a higher low. afterIndex restricts both lows to the candles after
// a preceding structure point (such as the correction's local high). Use -1
// when no preceding point needs to be enforced.
func LatestHigherLow(candles []market.Candle, pivotLeft, pivotRight, afterIndex int) (HigherLow, bool) {
	lows := LocalLows(candles, pivotLeft, pivotRight)
	for index := len(lows) - 1; index > 0; index-- {
		initialLow := lows[index-1]
		higherLow := lows[index]
		if initialLow.Index <= afterIndex || higherLow.Price <= initialLow.Price {
			continue
		}
		return HigherLow{InitialLow: initialLow, Pivot: higherLow}, true
	}
	return HigherLow{}, false
}

// FindRecovery identifies a recovery following afterIndex. The selected
// recovery high is the latest confirmed local high between the initial low and
// higher low, which is the immediately relevant resistance for the breakout.
func FindRecovery(candles []market.Candle, pivotLeft, pivotRight, afterIndex int) (Recovery, bool) {
	higherLow, ok := LatestHigherLow(candles, pivotLeft, pivotRight, afterIndex)
	if !ok {
		return Recovery{}, false
	}

	highs := LocalHighs(candles, pivotLeft, pivotRight)
	for index := len(highs) - 1; index >= 0; index-- {
		high := highs[index]
		if high.Index >= higherLow.Pivot.Index {
			continue
		}
		if high.Index <= higherLow.InitialLow.Index {
			break
		}
		return Recovery{HigherLow: higherLow, RecoveryHigh: high}, true
	}
	return Recovery{}, false
}

// FindRecoveryBreakout evaluates the latest completed daily close against a
// confirmed recovery structure. afterIndex normally is the correction local
// high index, ensuring that every pivot in the recovery occurs after the
// correction started. breakoutBufferPct is expressed as a percentage; zero
// preserves the MVP's default close-above-recovery-high rule.
func FindRecoveryBreakout(candles []market.Candle, pivotLeft, pivotRight, afterIndex int, breakoutBufferPct float64) (Breakout, bool) {
	if !finiteNonNegativePercentage(breakoutBufferPct) || len(candles) == 0 {
		return Breakout{}, false
	}

	recovery, ok := FindRecovery(candles, pivotLeft, pivotRight, afterIndex)
	if !ok {
		return Breakout{}, false
	}
	close := candles[len(candles)-1].Close
	if !finitePositive(close) {
		return Breakout{}, false
	}

	threshold := recovery.RecoveryHigh.Price * (1 + breakoutBufferPct/100)
	return Breakout{
		Recovery:       recovery,
		Close:          close,
		ThresholdPrice: threshold,
		Confirmed:      close > threshold,
	}, true
}

func finiteNonNegativePercentage(value float64) bool {
	return value >= 0 && value <= 100 && !isNaNOrInf(value)
}
