package coinbase

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderReturnsCompletedDailyCandlesFromFixture(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "btc-candles.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products/BTC-USD/candles" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("granularity") != "86400" {
			t.Fatalf("unexpected granularity: %q", r.URL.Query().Get("granularity"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	provider := NewProvider(server.Client(), server.URL)
	provider.now = func() time.Time { return time.Date(2025, time.January, 4, 12, 0, 0, 0, time.UTC) }
	candles, err := provider.GetDailyCandles("BTC", 2)
	if err != nil {
		t.Fatalf("get daily candles: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("candle count = %d, want 2", len(candles))
	}
	if got, want := candles[0].Timestamp, time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("first candle timestamp = %s, want %s", got, want)
	}
	if candles[0].Close != 105 || candles[1].Close != 108 {
		t.Fatalf("unexpected sorted candles: %+v", candles)
	}
}

func TestProviderGetsCurrentPriceAndRejectsUnsupportedAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products/ETH-USD/ticker" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"price":"3500.25"}`))
	}))
	defer server.Close()

	provider := NewProvider(server.Client(), server.URL)
	price, err := provider.GetCurrentPrice("ETH")
	if err != nil || price != 3500.25 {
		t.Fatalf("price = %v, err = %v", price, err)
	}
	if _, err := provider.GetCurrentPrice("DOGE"); err == nil {
		t.Fatal("expected unsupported asset error")
	}
}

func TestProviderRejectsAnUnclosedOnlyCandleResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[[1735948800,108,112,110,111,1000]]`))
	}))
	defer server.Close()

	provider := NewProvider(server.Client(), server.URL)
	provider.now = func() time.Time { return time.Date(2025, time.January, 4, 12, 0, 0, 0, time.UTC) }
	if _, err := provider.GetDailyCandles("BTC", 1); err == nil {
		t.Fatal("expected no completed candle error")
	}
}
