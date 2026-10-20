package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func TestEvaluateStrategyAtomicallyDeduplicatesActionEvents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	candle := market.Candle{Timestamp: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC), Open: 100, High: 100, Low: 100, Close: 100}
	if err := store.SaveDailyCandles(context.Background(), "BTC", []market.Candle{candle}); err != nil {
		t.Fatalf("save candle: %v", err)
	}
	evaluate := func(input StrategyEvaluationInput) (strategy.Result, strategy.PersistedState) {
		return strategy.Result{
			Asset: "BTC", Action: strategy.ActionBuy40, ActionPct: 40, State: strategy.StatePartialPosition,
			Price: input.Candles[len(input.Candles)-1].Close, Reason: "A deterministic test action.",
		}, strategy.PersistedState{Asset: "BTC", CurrentState: strategy.StatePartialPosition, EntryStep: 1}
	}
	for range 2 {
		if _, err := store.EvaluateStrategyAtomically(context.Background(), "BTC", evaluate); err != nil {
			t.Fatalf("evaluate strategy: %v", err)
		}
	}
	events, err := store.ListStrategyEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].Action != string(strategy.ActionBuy40) || !events[0].Timestamp.Equal(candle.Timestamp) {
		t.Fatalf("unexpected idempotent events: %+v", events)
	}
}

func TestStrategyEvaluationUsesManagedDatabaseCandlesForHistoricalPeak(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	history := make([]market.Candle, 300)
	for index := range history {
		history[index] = market.Candle{
			Timestamp: start.AddDate(0, 0, index),
			Open:      100,
			High:      101 + float64(index),
			Low:       99,
			Close:     100,
			Volume:    1,
		}
	}
	if _, err := store.ReplaceManagedBacktestCandleSet(context.Background(), BacktestCandleSet{
		Asset: "BTC", SourceLabel: "Coinbase Exchange API (managed)", SourceFilename: "btc-usd-daily",
		OriginalSHA256: strings.Repeat("a", 64),
	}, history); err != nil {
		t.Fatalf("store managed history: %v", err)
	}
	signalCandle := market.Candle{Timestamp: start.AddDate(0, 0, len(history)), Open: 100, High: 100, Low: 100, Close: 100, Volume: 1}
	if err := store.SaveDailyCandles(context.Background(), "BTC", []market.Candle{signalCandle}); err != nil {
		t.Fatalf("save signal candle: %v", err)
	}

	var referencePeak float64
	_, err = store.EvaluateStrategyAtomically(context.Background(), "BTC", func(input StrategyEvaluationInput) (strategy.Result, strategy.PersistedState) {
		referencePeak = input.Config.ATHReferencePeak
		return strategy.Result{Asset: "BTC", Action: strategy.ActionHold}, input.State
	})
	if err != nil {
		t.Fatalf("evaluate strategy: %v", err)
	}
	if referencePeak <= 0 {
		t.Fatalf("historical reference peak=%v; want managed database candle average", referencePeak)
	}
}

func TestStaleHistoricalPeakEntry(t *testing.T) {
	input := StrategyEvaluationInput{
		Position: portfolio.Asset{Symbol: "BTC"},
		Config:   strategy.Config{ATHEntryOverrideEnabled: true, ATHReferencePeak: 100, ATHEntryThresholdPct: 60},
	}
	pending := strategy.Result{Action: strategy.ActionBuy40, Price: 61, Reason: "ATH entry override: old signal"}
	if !staleHistoricalPeakEntry(pending, input) {
		t.Fatal("expected entry above the new threshold to be stale")
	}
	pending.Price = 60
	pending.RecommendationRuleFingerprint = historicalPeakEntryFingerprint(pending, input.Config)
	if staleHistoricalPeakEntry(pending, input) {
		t.Fatal("entry at the current threshold must remain pending")
	}
	pending.Price, pending.Reason = 61, "Recovery breakout entry"
	if staleHistoricalPeakEntry(pending, input) {
		t.Fatal("non-historical entry must not be dismissed automatically")
	}
	pending.Reason, input.Position.Quantity = "ATH entry override: old signal", 1
	if staleHistoricalPeakEntry(pending, input) {
		t.Fatal("an entry with a recorded position must not be dismissed automatically")
	}
}
