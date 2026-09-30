package backtest

import (
	"fmt"
	"math"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

const minimumHistory = 200

// MinimumCandleCount is the smallest complete imported data set that can
// provide the SMA200 warm-up plus at least one decision candle.
const MinimumCandleCount = minimumHistory + 1

func ValidateRequest(request Request) error {
	if request.Asset != "BTC" && request.Asset != "ETH" {
		return fmt.Errorf("asset must be BTC or ETH")
	}
	if !isUTCMidnight(request.Start) || !isUTCMidnight(request.End) {
		return fmt.Errorf("start and end must be UTC-midnight daily timestamps")
	}
	if request.Start.After(request.End) {
		return fmt.Errorf("start must be on or before end")
	}
	if !finitePositive(request.StartingCashUSD) {
		return fmt.Errorf("starting cash must be finite and greater than zero")
	}
	if !finiteNonNegative(request.FeeBps) || !finiteNonNegative(request.SlippageBps) {
		return fmt.Errorf("fee and slippage basis points must be finite and non-negative")
	}
	if request.ExecutionModel != ExecutionModelNextDailyOpen {
		return fmt.Errorf("execution model must be %s", ExecutionModelNextDailyOpen)
	}
	if err := strategy.ValidateConfig(request.StrategyConfig); err != nil {
		return fmt.Errorf("strategy config: %w", err)
	}
	return nil
}

// ValidateCandles verifies the canonical daily-candle constraints shared by
// CSV import, SQLite reads, and the pure simulator.
func ValidateCandles(candles []market.Candle) error {
	if len(candles) == 0 {
		return fmt.Errorf("at least one candle is required")
	}
	for index, candle := range candles {
		if !isUTCMidnight(candle.Timestamp) {
			return fmt.Errorf("candle %d timestamp must be UTC midnight", index)
		}
		if !finitePositive(candle.Open) || !finitePositive(candle.High) || !finitePositive(candle.Low) || !finitePositive(candle.Close) {
			return fmt.Errorf("candle %d OHLC values must be finite and greater than zero", index)
		}
		if candle.High < max(candle.Open, candle.Close, candle.Low) || candle.Low > min(candle.Open, candle.Close, candle.High) {
			return fmt.Errorf("candle %d has invalid OHLC relationship", index)
		}
		if !finiteNonNegative(candle.Volume) {
			return fmt.Errorf("candle %d volume must be finite and non-negative", index)
		}
		if index > 0 {
			previous := candles[index-1].Timestamp
			if !candle.Timestamp.After(previous) {
				return fmt.Errorf("candles must be strictly ascending")
			}
			if candle.Timestamp.Sub(previous) != 24*time.Hour {
				return fmt.Errorf("candles must not have missing UTC days")
			}
		}
	}
	return nil
}

func rangeIndexes(candles []market.Candle, start, end time.Time) (int, int, error) {
	startIndex, endIndex := -1, -1
	for index, candle := range candles {
		if candle.Timestamp.Equal(start) {
			startIndex = index
		}
		if candle.Timestamp.Equal(end) {
			endIndex = index
		}
	}
	if startIndex < 0 || endIndex < 0 {
		return 0, 0, fmt.Errorf("requested range must be contained in candle data")
	}
	if startIndex < minimumHistory-1 {
		return 0, 0, fmt.Errorf("requested start requires at least %d completed daily candles of warm-up", minimumHistory)
	}
	return startIndex, endIndex, nil
}

func isUTCMidnight(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC && value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 && value.Nanosecond() == 0
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func max(values ...float64) float64 {
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func min(values ...float64) float64 {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
