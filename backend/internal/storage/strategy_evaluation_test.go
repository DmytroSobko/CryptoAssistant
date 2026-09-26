package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
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
