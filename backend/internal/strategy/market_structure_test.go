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

func TestLatestHigherLowUsesConfirmedConsecutiveLocalLowsAfterCorrection(t *testing.T) {
	tests := []struct {
		name       string
		prices     []float64
		afterIndex int
		wantOK     bool
		initial    int
		higher     int
	}{
		{
			name:       "detects the newest higher low after the correction high",
			prices:     []float64{100, 120, 100, 80, 90, 85, 95, 96, 110},
			afterIndex: 1,
			wantOK:     true,
			initial:    3,
			higher:     5,
		},
		{
			name:       "does not use lows before the correction high",
			prices:     []float64{80, 100, 85, 120, 110, 115, 112},
			afterIndex: 3,
			wantOK:     false,
		},
		{
			name:       "a lower second low is not a higher low",
			prices:     []float64{100, 120, 100, 90, 100, 80, 100},
			afterIndex: 1,
			wantOK:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			higherLow, ok := LatestHigherLow(candleFixture(test.prices), 1, 1, test.afterIndex)
			if ok != test.wantOK {
				t.Fatalf("LatestHigherLow ok = %v; want %v (%+v)", ok, test.wantOK, higherLow)
			}
			if ok && (higherLow.InitialLow.Index != test.initial || higherLow.Pivot.Index != test.higher) {
				t.Fatalf("higher low = %+v; want lows at %d then %d", higherLow, test.initial, test.higher)
			}
		})
	}
}

func TestFindRecoveryBreakoutUsesRecoveryHighAndLatestCompletedClose(t *testing.T) {
	tests := []struct {
		name      string
		prices    []float64
		close     float64
		bufferPct float64
		wantOK    bool
		wantHigh  int
		wantBreak bool
		threshold float64
	}{
		{
			name:      "breaks above the confirmed recovery high",
			prices:    []float64{100, 120, 100, 80, 90, 85, 95, 96},
			close:     96,
			bufferPct: 0,
			wantOK:    true,
			wantHigh:  4,
			wantBreak: true,
			threshold: 90,
		},
		{
			name:      "buffer requires a close strictly above the buffered high",
			prices:    []float64{100, 120, 100, 80, 90, 85, 95, 94.5},
			close:     94.5,
			bufferPct: 5,
			wantOK:    true,
			wantHigh:  4,
			wantBreak: false,
			threshold: 94.5,
		},
		{
			name:      "does not treat one confirmed low as a recovery",
			prices:    []float64{100, 120, 100, 80, 85, 90},
			close:     90,
			bufferPct: 0,
			wantOK:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candles := candleFixture(test.prices)
			candles[len(candles)-1].Close = test.close
			breakout, ok := FindRecoveryBreakout(candles, 1, 1, 1, test.bufferPct)
			if ok != test.wantOK {
				t.Fatalf("FindRecoveryBreakout ok = %v; want %v (%+v)", ok, test.wantOK, breakout)
			}
			if !ok {
				return
			}
			if breakout.Recovery.RecoveryHigh.Index != test.wantHigh || breakout.Confirmed != test.wantBreak || !approximatelyEqual(breakout.ThresholdPrice, test.threshold) {
				t.Fatalf("breakout = %+v; want high index %d, confirmed %v, threshold %v", breakout, test.wantHigh, test.wantBreak, test.threshold)
			}
		})
	}

	if _, ok := FindRecoveryBreakout(candleFixture([]float64{100, 120, 100, 80, 90, 85, 95, 88, 100, 96}), 1, 1, 1, -0.1); ok {
		t.Fatal("a negative breakout buffer should be unavailable")
	}
}

func TestEntryAllowedByTrend(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		trend          Trend
		strongRecovery bool
		want           bool
	}{
		{"strict permits only a close above SMA200", TrendModeStrict, TrendBullish, false, true},
		{"strict rejects bearish recovery", TrendModeStrict, TrendBearish, true, false},
		{"strict rejects insufficient SMA200 history", TrendModeStrict, TrendUnknown, true, false},
		{"recovery permits a strong recovery below SMA200", TrendModeRecovery, TrendBearish, true, true},
		{"recovery still requires structure below SMA200", TrendModeRecovery, TrendBearish, false, false},
		{"recovery permits bullish entries", TrendModeRecovery, TrendBullish, false, true},
		{"off has no moving average filter", TrendModeOff, TrendUnknown, false, true},
		{"unknown modes fail closed", "anything", TrendBullish, true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EntryAllowedByTrend(test.mode, test.trend, test.strongRecovery); got != test.want {
				t.Fatalf("EntryAllowedByTrend(%q, %q, %v) = %v; want %v", test.mode, test.trend, test.strongRecovery, got, test.want)
			}
		})
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
