package storage

import (
	"context"
	"path/filepath"
	"testing"

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
