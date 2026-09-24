package strategy

import (
	"math"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

// Pivot is a confirmed local extremum in a chronological candle series. Index
// refers to the input slice, which lets later strategy stages relate pivots to
// one another without depending on timestamps being unique.
type Pivot struct {
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
}

// LocalHighs returns confirmed local highs. A high must be strictly greater
// than every high in its left and right pivot windows; equal highs deliberately
// do not create an ambiguous pivot. Candles are expected in chronological
// order and must be completed daily candles.
func LocalHighs(candles []market.Candle, pivotLeft, pivotRight int) []Pivot {
	return localPivots(candles, pivotLeft, pivotRight, func(candle market.Candle) float64 {
		return candle.High
	}, func(value, comparison float64) bool {
		return value > comparison
	})
}

// LocalLows returns confirmed local lows. A low must be strictly lower than
// every low in its left and right pivot windows.
func LocalLows(candles []market.Candle, pivotLeft, pivotRight int) []Pivot {
	return localPivots(candles, pivotLeft, pivotRight, func(candle market.Candle) float64 {
		return candle.Low
	}, func(value, comparison float64) bool {
		return value < comparison
	})
}

// LatestLocalHigh returns the most recent confirmed local high. It is the
// relevant high used by the initial correction-eligibility rule.
func LatestLocalHigh(candles []market.Candle, pivotLeft, pivotRight int) (Pivot, bool) {
	highs := LocalHighs(candles, pivotLeft, pivotRight)
	if len(highs) == 0 {
		return Pivot{}, false
	}
	return highs[len(highs)-1], true
}

func localPivots(candles []market.Candle, pivotLeft, pivotRight int, price func(market.Candle) float64, compare func(float64, float64) bool) []Pivot {
	if pivotLeft <= 0 || pivotRight <= 0 || len(candles) < pivotLeft+pivotRight+1 {
		return nil
	}

	pivots := make([]Pivot, 0)
	for index := pivotLeft; index+pivotRight < len(candles); index++ {
		candidate := price(candles[index])
		if !finitePositive(candidate) || !isStrictPivot(candles, index, pivotLeft, pivotRight, candidate, price, compare) {
			continue
		}
		pivots = append(pivots, Pivot{Index: index, Timestamp: candles[index].Timestamp, Price: candidate})
	}
	return pivots
}

func isStrictPivot(candles []market.Candle, index, pivotLeft, pivotRight int, candidate float64, price func(market.Candle) float64, compare func(float64, float64) bool) bool {
	for comparisonIndex := index - pivotLeft; comparisonIndex <= index+pivotRight; comparisonIndex++ {
		if comparisonIndex == index {
			continue
		}
		comparison := price(candles[comparisonIndex])
		if !finitePositive(comparison) || !compare(candidate, comparison) {
			return false
		}
	}
	return true
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
