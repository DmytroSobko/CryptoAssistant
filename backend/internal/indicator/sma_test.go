package indicator

import (
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

func TestSMAUsesTheMostRecentCompletedFixtureCandles(t *testing.T) {
	candles := fixtureCandles(200)
	if got, ok := SMA50(candles); !ok || got != 175.5 {
		t.Fatalf("SMA50 = %v, %v; want 175.5, true", got, ok)
	}
	if got, ok := SMA200(candles); !ok || got != 100.5 {
		t.Fatalf("SMA200 = %v, %v; want 100.5, true", got, ok)
	}
}

func TestSMARequiresEnoughCandles(t *testing.T) {
	if _, ok := SMA200(fixtureCandles(199)); ok {
		t.Fatal("SMA200 should be unavailable with fewer than 200 candles")
	}
	if _, ok := SMA(fixtureCandles(2), 0); ok {
		t.Fatal("SMA should reject a non-positive period")
	}
}

func fixtureCandles(count int) []market.Candle {
	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]market.Candle, count)
	for i := range candles {
		close := float64(i + 1)
		candles[i] = market.Candle{Timestamp: start.AddDate(0, 0, i), Open: close, High: close, Low: close, Close: close}
	}
	return candles
}
