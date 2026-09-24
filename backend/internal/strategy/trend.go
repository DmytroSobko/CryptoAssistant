package strategy

import (
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/indicator"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

// Trend is a factual completed-candle classification, not a trading action or
// a prediction. Future entry logic can apply the configured TrendMode to it.
type Trend string

const (
	TrendUnknown Trend = "UNKNOWN"
	TrendBullish Trend = "BULLISH"
	TrendBearish Trend = "BEARISH"
)

// ClassifyTrend applies the MVP's strict SMA200 condition to the latest
// completed daily close. It returns UNKNOWN until 200 completed candles are
// available. A close equal to SMA200 is BEARISH because STRICT requires a
// close greater than SMA200.
func ClassifyTrend(candles []market.Candle) Trend {
	if len(candles) == 0 || !finitePositive(candles[len(candles)-1].Close) {
		return TrendUnknown
	}
	sma200, ok := indicator.SMA200(candles)
	if !ok {
		return TrendUnknown
	}
	if candles[len(candles)-1].Close > sma200 {
		return TrendBullish
	}
	return TrendBearish
}
