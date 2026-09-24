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

type fakeProvider struct {
	price   float64
	candles []market.Candle
}

func (p fakeProvider) GetCurrentPrice(string) (float64, error) { return p.price, nil }

func (p fakeProvider) GetDailyCandles(string, int) ([]market.Candle, error) {
	return p.candles, nil
}
