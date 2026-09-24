// Package indicator contains deterministic calculations over completed market
// candles. It has no dependency on storage, HTTP, or the strategy engine.
package indicator

import "github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"

const (
	SMA50Period  = 50
	SMA200Period = 200
)

// SMA returns the simple moving average of the most recent period closes. The
// boolean is false until enough completed candles are available.
func SMA(candles []market.Candle, period int) (float64, bool) {
	if period <= 0 || len(candles) < period {
		return 0, false
	}
	var total float64
	for _, candle := range candles[len(candles)-period:] {
		total += candle.Close
	}
	return total / float64(period), true
}

// SMA50 and SMA200 provide the trend indicators defined by the MVP without
// embedding any trend or trading decision logic.
func SMA50(candles []market.Candle) (float64, bool) {
	return SMA(candles, SMA50Period)
}

func SMA200(candles []market.Candle) (float64, bool) {
	return SMA(candles, SMA200Period)
}
