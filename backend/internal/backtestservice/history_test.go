package backtestservice

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

type historyRangeProviderStub struct {
	requests [][2]time.Time
	err      error
}

func (p *historyRangeProviderStub) GetDailyCandlesRange(_ string, start, end time.Time) ([]market.Candle, error) {
	p.requests = append(p.requests, [2]time.Time{start, end})
	if p.err != nil {
		return nil, p.err
	}
	candles := make([]market.Candle, 0)
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		price := float64(100 + len(candles))
		candles = append(candles, market.Candle{Timestamp: day, Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 10})
	}
	return candles, nil
}

func TestManagedHistoryBackfillsAndAppendsOnlyMissingDays(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	provider := &historyRangeProviderStub{}
	history := NewManagedHistory(store, provider)
	now := time.Date(2016, time.January, 1, 12, 0, 0, 0, time.UTC)
	history.now = func() time.Time { return now }

	first, err := history.Refresh(context.Background(), "BTC")
	if err != nil {
		t.Fatal(err)
	}
	if first.CandleSet.ID != "managed-btc" || first.AddedCandles != 365 || first.CandleSet.CandleCount != 365 || len(provider.requests) != 2 {
		t.Fatalf("initial refresh=%+v", first)
	}
	second, err := history.Refresh(context.Background(), "BTC")
	if err != nil || second.AddedCandles != 0 || second.CandleSet.CandleCount != 365 {
		t.Fatalf("same-day refresh=%+v err=%v", second, err)
	}

	now = now.AddDate(0, 0, 1)
	third, err := history.Refresh(context.Background(), "BTC")
	if err != nil {
		t.Fatal(err)
	}
	if third.AddedCandles != 1 || third.CandleSet.CandleCount != 366 {
		t.Fatalf("incremental refresh=%+v", third)
	}
	lastRequest := provider.requests[len(provider.requests)-1]
	if want := time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC); !lastRequest[0].Equal(want) {
		t.Fatalf("incremental request starts %s, want %s", lastRequest[0], want)
	}
	var rowCount int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM backtest_candles WHERE candle_set_id = 'managed-btc'`).Scan(&rowCount); err != nil || rowCount != 366 {
		t.Fatalf("stored candle count=%d err=%v", rowCount, err)
	}
}

func TestManagedHistoryFailureKeepsStoredCandles(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	provider := &historyRangeProviderStub{}
	history := NewManagedHistory(store, provider)
	now := time.Date(2016, time.January, 1, 12, 0, 0, 0, time.UTC)
	history.now = func() time.Time { return now }
	if _, err := history.Refresh(context.Background(), "BTC"); err != nil {
		t.Fatal(err)
	}
	now = now.AddDate(0, 0, 1)
	provider.err = context.DeadlineExceeded
	if _, err := history.Refresh(context.Background(), "BTC"); err == nil {
		t.Fatal("expected provider failure")
	}
	set, candles, err := store.GetBacktestCandleSet(context.Background(), "managed-btc")
	if err != nil || set.CandleCount != 365 || len(candles) != 365 {
		t.Fatalf("failed refresh changed stored history: set=%+v candles=%d err=%v", set, len(candles), err)
	}
}

func TestLongestContiguousSuffixDropsOnlyHistoryBeforeLastGap(t *testing.T) {
	day := time.Date(2016, time.May, 1, 0, 0, 0, 0, time.UTC)
	candles := []market.Candle{
		{Timestamp: day},
		{Timestamp: day.AddDate(0, 0, 1)},
		{Timestamp: day.AddDate(0, 0, 3)},
		{Timestamp: day.AddDate(0, 0, 4)},
	}
	result := longestContiguousSuffix(candles)
	if len(result) != 2 || !result[0].Timestamp.Equal(day.AddDate(0, 0, 3)) {
		t.Fatalf("contiguous suffix=%+v", result)
	}
}
