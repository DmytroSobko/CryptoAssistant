package strategy

import (
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

func TestLocalPivotsDetectStrictHighsAndLows(t *testing.T) {
	t.Run("highs", func(t *testing.T) {
		candles := candleFixture([]float64{10, 12, 16, 13, 11, 14, 18, 15, 12})
		assertPivotIndexes(t, LocalHighs(candles, 2, 2), 2, 6)
	})
	t.Run("lows", func(t *testing.T) {
		candles := candleFixture([]float64{18, 15, 10, 14, 17, 13, 8, 12, 16})
		assertPivotIndexes(t, LocalLows(candles, 2, 2), 2, 6)
	})
}

func TestLocalPivotsRequireACompleteStrictWindow(t *testing.T) {
	t.Run("equal neighbors are not pivots", func(t *testing.T) {
		candles := candleFixture([]float64{10, 15, 15, 12, 10})
		assertPivotIndexes(t, LocalHighs(candles, 1, 1))
	})
	t.Run("unconfirmed right edge is not a pivot", func(t *testing.T) {
		candles := candleFixture([]float64{10, 12, 11, 20})
		assertPivotIndexes(t, LocalHighs(candles, 1, 1), 1)
	})
	t.Run("invalid windows do not create pivots", func(t *testing.T) {
		candles := candleFixture([]float64{10, 12, 10})
		assertPivotIndexes(t, LocalHighs(candles, 0, 1))
		assertPivotIndexes(t, LocalLows(candles, 1, 0))
	})
}

func TestFindCorrectionUsesMostRecentConfirmedLocalHighAndLatestClose(t *testing.T) {
	// There are highs at indexes 1 ($110) and 4 ($105). The correction must
	// use the newer, relevant $105 high rather than the larger but older one.
	candles := candleFixture([]float64{90, 110, 90, 95, 105, 95, 95})
	candles[len(candles)-1].Close = 90
	candles[len(candles)-1].Low = 85

	correction, ok := FindCorrection(candles, 1, 1, 10)
	if !ok {
		t.Fatal("FindCorrection reported no usable correction inputs")
	}
	if correction.LocalHigh.Index != 4 || correction.LocalHigh.Price != 105 {
		t.Fatalf("local high = %+v; want index 4 at 105", correction.LocalHigh)
	}
	if correction.Close != 90 || correction.ThresholdPrice != 94.5 || !approximatelyEqual(correction.PullbackPct, 100.0/7.0) {
		t.Fatalf("correction = %+v; want close 90, threshold 94.5, pullback %v", correction, 100.0/7.0)
	}
	if !correction.Eligible {
		t.Fatal("correction should be eligible at a close below the 10% threshold")
	}
}

func TestFindCorrectionIsEligibilityOnly(t *testing.T) {
	candles := candleFixture([]float64{90, 100, 90, 92, 92})
	candles[len(candles)-1].Close = 91
	candles[len(candles)-1].Low = 90

	correction, ok := FindCorrection(candles, 1, 1, 10)
	if !ok {
		t.Fatal("FindCorrection reported no usable correction inputs")
	}
	if correction.Eligible {
		t.Fatalf("correction = %+v; close above threshold must not be eligible", correction)
	}
	if _, ok := FindCorrection(candles, 1, 1, 0); ok {
		t.Fatal("a non-positive pullback configuration should be unavailable")
	}

	thresholdCandles := candleFixture([]float64{90, 100, 90, 90})
	atThreshold, ok := FindCorrection(thresholdCandles, 1, 1, 10)
	if !ok || !atThreshold.Eligible {
		t.Fatalf("correction at threshold = %+v, %v; it must be eligible", atThreshold, ok)
	}
}

func TestClassifyTrendUsesLatestCompletedCloseAndSMA200(t *testing.T) {
	candles := make([]market.Candle, 200)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for index := range candles {
		close := 100.0
		candles[index] = market.Candle{Timestamp: start.AddDate(0, 0, index), Open: close, High: close, Low: close, Close: close}
	}
	if got := ClassifyTrend(candles[:199]); got != TrendUnknown {
		t.Fatalf("trend with 199 candles = %q; want %q", got, TrendUnknown)
	}
	if got := ClassifyTrend(candles); got != TrendBearish {
		t.Fatalf("trend at SMA200 = %q; want %q", got, TrendBearish)
	}

	candles[len(candles)-1].Close = 110
	candles[len(candles)-1].High = 110
	if got := ClassifyTrend(candles); got != TrendBullish {
		t.Fatalf("trend above SMA200 = %q; want %q", got, TrendBullish)
	}

	candles[len(candles)-1].Close = 90
	candles[len(candles)-1].Low = 90
	if got := ClassifyTrend(candles); got != TrendBearish {
		t.Fatalf("trend below SMA200 = %q; want %q", got, TrendBearish)
	}
}

func candleFixture(prices []float64) []market.Candle {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]market.Candle, len(prices))
	for index, price := range prices {
		candles[index] = market.Candle{
			Timestamp: start.AddDate(0, 0, index),
			Open:      price,
			High:      price,
			Low:       price,
			Close:     price,
		}
	}
	return candles
}

func assertPivotIndexes(t *testing.T, pivots []Pivot, want ...int) {
	t.Helper()
	if len(pivots) != len(want) {
		t.Fatalf("pivot count = %d; want %d (%+v)", len(pivots), len(want), pivots)
	}
	for index, pivot := range pivots {
		if pivot.Index != want[index] {
			t.Fatalf("pivot %d index = %d; want %d (%+v)", index, pivot.Index, want[index], pivots)
		}
	}
}

func approximatelyEqual(got, want float64) bool {
	const tolerance = 1e-12
	return got-want < tolerance && want-got < tolerance
}
