package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

func TestMigrateAndPersistPortfolio(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	input := portfolio.Portfolio{
		CashBalance: 2500,
		Assets:      []portfolio.Asset{{Symbol: "BTC", Quantity: 0.12, AverageEntryPrice: 71200, CashAllocated: 8544}},
	}
	if err := store.SavePortfolio(context.Background(), input); err != nil {
		t.Fatalf("save portfolio: %v", err)
	}
	actual, err := store.GetPortfolio(context.Background())
	if err != nil {
		t.Fatalf("get portfolio: %v", err)
	}
	if actual.CashBalance != 2500 || len(actual.Assets) != 2 {
		t.Fatalf("unexpected portfolio: %+v", actual)
	}
	if actual.Assets[0].Symbol != "BTC" || actual.Assets[0].Quantity != 0.12 {
		t.Fatalf("BTC was not persisted: %+v", actual.Assets[0])
	}
}

func TestMigrateIsIdempotentAndConfigDefaultsAreIndependent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("repeat migrate: %v", err)
	}

	btc, err := store.GetStrategyConfig(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("get BTC config: %v", err)
	}
	eth, err := store.GetStrategyConfig(context.Background(), "ETH")
	if err != nil {
		t.Fatalf("get ETH config: %v", err)
	}
	if btc.PullbackMinPct != 10 || eth.PullbackMinPct != 15 {
		t.Fatalf("unexpected independent defaults: BTC=%+v ETH=%+v", btc, eth)
	}
}

func TestPersistMarketCandlesAndCurrentPriceSeparately(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dayOne := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	candles := []market.Candle{
		{Timestamp: dayOne, Open: 90, High: 110, Low: 80, Close: 100, Volume: 42},
		{Timestamp: dayOne.AddDate(0, 0, 1), Open: 100, High: 120, Low: 95, Close: 115, Volume: 64},
	}
	if err := store.SaveDailyCandles(context.Background(), "btc", candles); err != nil {
		t.Fatalf("save candles: %v", err)
	}
	if err := store.SaveCurrentPrice(context.Background(), market.Snapshot{Symbol: "BTC", Price: 117, UpdatedAt: dayOne.AddDate(0, 0, 2).Add(3 * time.Hour)}); err != nil {
		t.Fatalf("save current price: %v", err)
	}

	actualCandles, err := store.ListDailyCandles(context.Background(), "BTC", 200)
	if err != nil {
		t.Fatalf("list candles: %v", err)
	}
	if len(actualCandles) != 2 || actualCandles[0].Close != 100 || actualCandles[1].Close != 115 {
		t.Fatalf("unexpected candles: %+v", actualCandles)
	}
	snapshot, err := store.LatestSnapshot(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("get current price: %v", err)
	}
	if snapshot.Price != 117 || !snapshot.UpdatedAt.Equal(dayOne.AddDate(0, 0, 2).Add(3*time.Hour)) {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	if err := store.SaveDailyCandles(context.Background(), "BTC", []market.Candle{{Timestamp: dayOne, Open: 91, High: 111, Low: 81, Close: 101, Volume: 43}}); err != nil {
		t.Fatalf("upsert candle: %v", err)
	}
	actualCandles, err = store.ListDailyCandles(context.Background(), "BTC", 0)
	if err != nil || actualCandles[0].Close != 101 {
		t.Fatalf("candle was not updated: candles=%+v err=%v", actualCandles, err)
	}
}

func TestMarketStorageRejectsInvalidValues(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	err = store.SaveCurrentPrice(context.Background(), market.Snapshot{Symbol: "DOGE", Price: 1, UpdatedAt: time.Now()})
	if err == nil {
		t.Fatalf("expected invalid asset error, got %v", err)
	}
}
