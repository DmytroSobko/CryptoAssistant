package marketdata

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

func TestRefresherPersistsCompletedCandlesAndCurrentPrice(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	provider := fakeProvider{price: 105, candles: []market.Candle{
		{Timestamp: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), Open: 90, High: 110, Low: 80, Close: 100, Volume: 12},
		{Timestamp: time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC), Open: 100, High: 108, Low: 95, Close: 105, Volume: 16},
	}}
	refresher := NewRefresher(provider, store)
	refresher.now = func() time.Time { return time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC) }

	if err := refresher.RefreshAsset(context.Background(), "BTC"); err != nil {
		t.Fatalf("refresh asset: %v", err)
	}
	candles, err := store.ListDailyCandles(context.Background(), "BTC", 0)
	if err != nil || len(candles) != 2 || candles[1].Close != 105 {
		t.Fatalf("persisted candles = %+v, err = %v", candles, err)
	}
	snapshot, err := store.LatestSnapshot(context.Background(), "BTC")
	if err != nil || snapshot.Price != 105 || !snapshot.UpdatedAt.Equal(refresher.now()) {
		t.Fatalf("persisted snapshot = %+v, err = %v", snapshot, err)
	}
}

func TestRefreshPricesSkipsDailyCandles(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	provider := &countingProvider{price: 105}
	refresher := NewRefresher(provider, store)
	refresher.now = func() time.Time { return time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC) }

	if err := refresher.RefreshPrices(context.Background()); err != nil {
		t.Fatalf("refresh prices: %v", err)
	}
	if provider.priceCalls != 2 || provider.candleCalls != 0 {
		t.Fatalf("provider calls: prices=%d candles=%d", provider.priceCalls, provider.candleCalls)
	}
	for _, symbol := range []string{"BTC", "ETH"} {
		snapshot, err := store.LatestSnapshot(context.Background(), symbol)
		if err != nil || snapshot.Price != 105 || !snapshot.UpdatedAt.Equal(refresher.now()) {
			t.Fatalf("%s snapshot = %+v, err = %v", symbol, snapshot, err)
		}
	}
}

type fakeProvider struct {
	price   float64
	candles []market.Candle
}

func (p fakeProvider) GetCurrentPrice(string) (float64, error) { return p.price, nil }

func (p fakeProvider) GetDailyCandles(string, int) ([]market.Candle, error) {
	return p.candles, nil
}

type countingProvider struct {
	price       float64
	priceCalls  int
	candleCalls int
}

func (p *countingProvider) GetCurrentPrice(string) (float64, error) {
	p.priceCalls++
	return p.price, nil
}

func (p *countingProvider) GetDailyCandles(string, int) ([]market.Candle, error) {
	p.candleCalls++
	return nil, nil
}
