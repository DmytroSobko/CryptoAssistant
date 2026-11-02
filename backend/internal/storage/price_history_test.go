package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

func TestPriceHistoryRetainsConfiguredRecordsPerAsset(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if limit, err := store.PriceHistoryRecordLimit(ctx, "BTC"); err != nil || limit != DefaultPriceHistoryRecordLimit {
		t.Fatalf("default BTC limit = %d, err = %v", limit, err)
	}
	if err := store.SavePriceHistoryRecordLimit(ctx, "BTC", 3); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	for index := 0; index < 5; index++ {
		if err := store.SaveCurrentPrice(ctx, market.Snapshot{Symbol: "BTC", Price: 100 + float64(index), UpdatedAt: start.Add(time.Duration(index) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 2; index++ {
		if err := store.SaveCurrentPrice(ctx, market.Snapshot{Symbol: "ETH", Price: 200 + float64(index), UpdatedAt: start.Add(time.Duration(index) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}

	btc, err := store.ListPriceHistory(ctx, "BTC", time.Time{})
	if err != nil || len(btc) != 3 || btc[0].Price != 102 || btc[2].Price != 104 {
		t.Fatalf("BTC history = %+v, err = %v", btc, err)
	}
	eth, err := store.ListPriceHistory(ctx, "ETH", time.Time{})
	if err != nil || len(eth) != 2 || eth[0].Price != 200 || eth[1].Price != 201 {
		t.Fatalf("ETH history = %+v, err = %v", eth, err)
	}
	var currentSnapshotCount int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM market_snapshots WHERE asset = 'BTC' AND open IS NULL`).Scan(&currentSnapshotCount); err != nil || currentSnapshotCount != 1 {
		t.Fatalf("current BTC snapshots = %d, err = %v", currentSnapshotCount, err)
	}

	if err := store.SavePriceHistoryRecordLimit(ctx, "BTC", 2); err != nil {
		t.Fatal(err)
	}
	btc, err = store.ListPriceHistory(ctx, "BTC", time.Time{})
	if err != nil || len(btc) != 2 || btc[0].Price != 103 || btc[1].Price != 104 {
		t.Fatalf("pruned BTC history = %+v, err = %v", btc, err)
	}
	eth, err = store.ListPriceHistory(ctx, "ETH", time.Time{})
	if err != nil || len(eth) != 2 {
		t.Fatalf("BTC retention change affected ETH history: %+v, err = %v", eth, err)
	}
}

func TestPriceHistoryRejectsInvalidRecordLimits(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePriceHistoryRecordLimit(context.Background(), "BTC", 0); err == nil {
		t.Fatal("expected zero record limit to be rejected")
	}
	if err := store.SavePriceHistoryRecordLimit(context.Background(), "BTC", MaxPriceHistoryRecordLimit+1); err == nil {
		t.Fatal("expected oversized record limit to be rejected")
	}
}
