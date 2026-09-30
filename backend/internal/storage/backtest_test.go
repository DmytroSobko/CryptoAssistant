package storage

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

func TestBacktestingMigrationUpgradesAnExistingMVPDatabase(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB().Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_initial.sql", "002_strategy_state_engine_fields.sql", "003_strategy_event_idempotency.sql"} {
		contents, err := os.ReadFile(filepath.Join("../../migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().Exec(string(contents)); err != nil {
			t.Fatalf("apply existing %s: %v", name, err)
		}
		if _, err := store.DB().Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("upgrade database: %v", err)
	}
	var count int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'backtest_candle_sets'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backtest migration did not apply: count=%d err=%v", count, err)
	}
}

func TestBacktestCandleSetRoundTripIsIsolatedFromLiveMarketData(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}

	candles := backtestCandles(201)
	input := BacktestCandleSet{Asset: "BTC", SourceLabel: "Coinbase daily export", SourceFilename: "/private/user-downloads/btc-daily.csv", OriginalSHA256: strings.Repeat("a", 64)}
	saved, err := store.ImportBacktestCandleSet(context.Background(), input, candles)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.SourceFilename != "btc-daily.csv" || saved.SchemaVersion != 1 || saved.CandleCount != len(candles) || saved.ImportedAt.IsZero() {
		t.Fatalf("unexpected saved candle set: %+v", saved)
	}

	loaded, actualCandles, err := store.GetBacktestCandleSet(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != saved.ID || loaded.OriginalSHA256 != input.OriginalSHA256 || !reflect.DeepEqual(actualCandles, candles) {
		t.Fatalf("candle-set round trip mismatch: set=%+v candles=%+v", loaded, actualCandles)
	}
	sets, err := store.ListBacktestCandleSets(context.Background(), "BTC")
	if err != nil || len(sets) != 1 || sets[0].ID != saved.ID {
		t.Fatalf("list candle sets: sets=%+v err=%v", sets, err)
	}
	ethSets, err := store.ListBacktestCandleSets(context.Background(), "ETH")
	if err != nil || len(ethSets) != 0 {
		t.Fatalf("ETH list: sets=%+v err=%v", ethSets, err)
	}
	if live, err := store.ListDailyCandles(context.Background(), "BTC", 0); err != nil || len(live) != 0 {
		t.Fatalf("backtest import changed live candles: candles=%+v err=%v", live, err)
	}
}

func TestBacktestCandleSetDuplicateAndInvalidInputDoNotPersistRows(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	candles := backtestCandles(backtest.MinimumCandleCount)
	input := BacktestCandleSet{Asset: "BTC", SourceLabel: "test", SourceFilename: "btc.csv", OriginalSHA256: strings.Repeat("b", 64)}
	if _, err := store.ImportBacktestCandleSet(context.Background(), input, candles); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportBacktestCandleSet(context.Background(), input, candles); err == nil {
		t.Fatal("expected duplicate SHA error")
	}
	bad := input
	bad.OriginalSHA256 = strings.Repeat("c", 64)
	if _, err := store.ImportBacktestCandleSet(context.Background(), bad, candles[:200]); err == nil {
		t.Fatal("expected small data-set error")
	}
	sets, err := store.ListBacktestCandleSets(context.Background(), "BTC")
	if err != nil || len(sets) != 1 {
		t.Fatalf("invalid import persisted a set: sets=%+v err=%v", sets, err)
	}
}

func backtestCandles(count int) []market.Candle {
	candles := make([]market.Candle, count)
	for index := range candles {
		price := float64(100 + index)
		candles[index] = market.Candle{Timestamp: time.Date(2020, 1, 1+index, 0, 0, 0, 0, time.UTC), Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 1}
	}
	return candles
}
