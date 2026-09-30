package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
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
	legacyConfig := DefaultStrategyConfig("BTC")
	legacyConfig.Entry1Pct = 35
	legacyJSON, err := json.Marshal(legacyConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO settings(key, value, updated_at) VALUES ('strategy_config_BTC', ?, CURRENT_TIMESTAMP)`, string(legacyJSON)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO strategy_states(asset, state, local_high, local_low, higher_low, highest_price, drawdown_pct, entry_step, position_open) VALUES ('BTC', 'FULL_POSITION', 110, 90, 100, 115, -5, 2, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("upgrade database: %v", err)
	}
	var count int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'backtest_candle_sets'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backtest migration did not apply: count=%d err=%v", count, err)
	}
	profile, err := store.GetAssetStrategySettings(context.Background(), "BTC")
	if err != nil {
		t.Fatal(err)
	}
	if profile.SelectedStrategyID != strategy.StrategyRecoveryBreakout || profile.RecoveryBreakout.Entry1Pct != 35 {
		t.Fatalf("legacy settings were not migrated: %+v", profile)
	}
	state, err := getStrategyStateForStrategy(context.Background(), store.DB(), "BTC", strategy.StrategyRecoveryBreakout)
	if err != nil {
		t.Fatal(err)
	}
	if state.CurrentState != strategy.StateFullPosition || state.EntryStep != 2 || !state.PositionOpen || state.HighestPrice != 115 {
		t.Fatalf("legacy strategy state was not migrated: %+v", state)
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

func TestBacktestRunRoundTripAndAssetMismatchAreIsolated(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	candles := backtestCandles(backtest.MinimumCandleCount)
	set, err := store.ImportBacktestCandleSet(context.Background(), BacktestCandleSet{Asset: "BTC", SourceLabel: "test", SourceFilename: "btc.csv", OriginalSHA256: strings.Repeat("d", 64)}, candles)
	if err != nil {
		t.Fatal(err)
	}
	request := backtest.Request{Asset: "BTC", Start: candles[199].Timestamp, End: candles[200].Timestamp, StartingCashUSD: 1000, StrategyConfig: storageBacktestConfig(), FeeBps: 10, SlippageBps: 5, ExecutionModel: backtest.ExecutionModelNextDailyOpen}
	result, err := backtest.Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}
	// The simulator normally sets this on protected Dip Accumulation exits.
	// Populate it here to verify the nullable audit column survives a complete
	// storage round-trip independently of a particular market fixture.
	result.Signals[0].BreakEvenFloorNetPrice = 123.45
	saved, err := store.SaveBacktestRun(context.Background(), set.ID, result)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetBacktestRun(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CandleSetID != set.ID || !reflect.DeepEqual(loaded.Result, result) {
		t.Fatalf("run round trip mismatch: loaded=%+v result=%+v", loaded, result)
	}
	list, err := store.ListBacktestRuns(context.Background(), 10)
	if err != nil || len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list runs: runs=%+v err=%v", list, err)
	}
	mismatch := result
	mismatch.Request.Asset = "ETH"
	if _, err := store.SaveBacktestRun(context.Background(), set.ID, mismatch); err == nil {
		t.Fatal("expected candle-set asset mismatch")
	}
	list, err = store.ListBacktestRuns(context.Background(), 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("failed run corrupted saved runs: runs=%+v err=%v", list, err)
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

func storageBacktestConfig() strategy.Config {
	return strategy.Config{PullbackMinPct: 10, PivotLeft: 1, PivotRight: 1, TrendMode: strategy.TrendModeOff, Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30, ProfitTrigger1Pct: 10, ProfitTrigger2Pct: 20, ProfitTakePct: 25, Drawdown1Pct: -10, Drawdown1SellPct: 20, Drawdown2Pct: -15, Drawdown2SellPct: 30, Drawdown3Pct: -20, Drawdown3SellPct: 70}
}
