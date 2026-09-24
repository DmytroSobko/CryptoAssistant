package strategy

// Trend modes define how a factual completed-candle trend classification is
// allowed to gate an otherwise valid entry breakout.
const (
	TrendModeStrict   = "STRICT"
	TrendModeRecovery = "RECOVERY"
	TrendModeOff      = "OFF"
)

// EntryAllowedByTrend applies the configured moving-average filter. A strong
// recovery is the caller's structural confirmation (a completed-candle
// recovery breakout), not an intraday price move.
//
// STRICT requires a BULLISH classification, which means the latest completed
// close is above SMA200. RECOVERY also permits a strong recovery while the
// trend is BEARISH. OFF deliberately bypasses the moving-average filter. An
// unknown mode fails closed.
func EntryAllowedByTrend(mode string, trend Trend, strongRecovery bool) bool {
	switch mode {
	case TrendModeStrict:
		return trend == TrendBullish
	case TrendModeRecovery:
		return trend == TrendBullish || (trend == TrendBearish && strongRecovery)
	case TrendModeOff:
		return true
	default:
		return false
	}
}
