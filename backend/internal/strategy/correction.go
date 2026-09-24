package strategy

import "github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"

// Correction describes the latest completed close relative to its most recent
// confirmed local high. PullbackPct is positive below the high and negative
// above it. Eligible only reports correction eligibility; it never implies a
// buy action.
type Correction struct {
	LocalHigh      Pivot   `json:"localHigh"`
	Close          float64 `json:"close"`
	ThresholdPrice float64 `json:"thresholdPrice"`
	PullbackPct    float64 `json:"pullbackPct"`
	Eligible       bool    `json:"eligible"`
}

// FindCorrection evaluates the latest completed daily close against the most
// recent confirmed local high. The returned boolean is false when there is no
// usable local high, close, or pullback configuration.
func FindCorrection(candles []market.Candle, pivotLeft, pivotRight int, pullbackMinPct float64) (Correction, bool) {
	if !finitePositive(pullbackMinPct) || pullbackMinPct > 100 || len(candles) == 0 {
		return Correction{}, false
	}

	localHigh, ok := LatestLocalHigh(candles, pivotLeft, pivotRight)
	if !ok {
		return Correction{}, false
	}
	close := candles[len(candles)-1].Close
	if !finitePositive(close) {
		return Correction{}, false
	}

	threshold := localHigh.Price * (1 - pullbackMinPct/100)
	pullbackPct := (localHigh.Price - close) / localHigh.Price * 100
	return Correction{
		LocalHigh:      localHigh,
		Close:          close,
		ThresholdPrice: threshold,
		PullbackPct:    pullbackPct,
		Eligible:       close <= threshold,
	}, true
}
