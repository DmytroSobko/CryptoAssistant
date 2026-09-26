package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	return NewServer(store, config.Config{}).Routes()
}

func TestPortfolioAndConfigEndpoints(t *testing.T) {
	handler := newTestHandler(t)

	put := httptest.NewRequest(http.MethodPut, "/api/portfolio", bytes.NewBufferString(`{"cashBalance":2500,"assets":[{"symbol":"BTC","quantity":0.12,"averageEntryPrice":71200,"cashAllocated":8544}]}`))
	put.Header.Set("Content-Type", "application/json")
	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, put)
	if putResponse.Code != http.StatusNoContent {
		t.Fatalf("save portfolio status = %d, body = %s", putResponse.Code, putResponse.Body.String())
	}

	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/portfolio", nil))
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get portfolio status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	if !bytes.Contains(getResponse.Body.Bytes(), []byte(`"cashBalance":2500`)) || !bytes.Contains(getResponse.Body.Bytes(), []byte(`"symbol":"BTC"`)) {
		t.Fatalf("unexpected portfolio body: %s", getResponse.Body.String())
	}

	configResponse := httptest.NewRecorder()
	handler.ServeHTTP(configResponse, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if configResponse.Code != http.StatusOK || !bytes.Contains(configResponse.Body.Bytes(), []byte(`"pullbackMinPct":10`)) {
		t.Fatalf("unexpected config response (%d): %s", configResponse.Code, configResponse.Body.String())
	}
}

func TestMarketEndpointReturnsPersistedCurrentPrice(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	updatedAt := time.Date(2026, time.January, 3, 12, 0, 0, 0, time.UTC)
	if err := store.SaveCurrentPrice(context.Background(), market.Snapshot{Symbol: "BTC", Price: 84200.5, UpdatedAt: updatedAt}); err != nil {
		t.Fatalf("save market snapshot: %v", err)
	}
	handler := NewServer(store, config.Config{}).Routes()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/market/btc", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("market status = %d, body = %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"symbol":"BTC"`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"price":84200.5`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"updatedAt":"2026-01-03T12:00:00Z"`)) {
		t.Fatalf("unexpected market response: %s", response.Body.String())
	}
}

func TestStrategyEndpointEvaluatesDailyCandlesAndRecordsOneActionEvent(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := store.SaveStrategyConfig(context.Background(), "BTC", strategy.Config{
		PullbackMinPct: 10, PivotLeft: 1, PivotRight: 1, TrendMode: strategy.TrendModeOff,
		Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30,
		ProfitTrigger1Pct: 10, ProfitTrigger2Pct: 20, ProfitTakePct: 25,
		Drawdown1Pct: -10, Drawdown1SellPct: 20, Drawdown2Pct: -15, Drawdown2SellPct: 30, Drawdown3Pct: -20, Drawdown3SellPct: 70,
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	prices := []float64{100, 120, 100, 80, 90, 85, 95, 96}
	candles := make([]market.Candle, len(prices))
	for index, price := range prices {
		candles[index] = market.Candle{Timestamp: start.AddDate(0, 0, index), Open: price, High: price, Low: price, Close: price}
	}
	handler := NewServer(store, config.Config{}).Routes()
	if err := store.SaveDailyCandles(context.Background(), "BTC", candles[:4]); err != nil {
		t.Fatalf("save correction candles: %v", err)
	}
	correction := httptest.NewRecorder()
	handler.ServeHTTP(correction, httptest.NewRequest(http.MethodGet, "/api/strategy/BTC", nil))
	if correction.Code != http.StatusOK || !bytes.Contains(correction.Body.Bytes(), []byte(`"state":"CORRECTION"`)) {
		t.Fatalf("correction strategy response (%d): %s", correction.Code, correction.Body.String())
	}
	if err := store.SaveDailyCandles(context.Background(), "BTC", candles[4:]); err != nil {
		t.Fatalf("save recovery candles: %v", err)
	}
	if err := store.SaveCurrentPrice(context.Background(), market.Snapshot{Symbol: "BTC", Price: 500000, UpdatedAt: start.Add(8 * 24 * time.Hour)}); err != nil {
		t.Fatalf("save display price: %v", err)
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/strategy/BTC", nil))
	if first.Code != http.StatusOK || !bytes.Contains(first.Body.Bytes(), []byte(`"action":"BUY_40"`)) || !bytes.Contains(first.Body.Bytes(), []byte(`"price":96`)) {
		t.Fatalf("first strategy response (%d): %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/strategy/BTC", nil))
	if second.Code != http.StatusOK || !bytes.Contains(second.Body.Bytes(), []byte(`"action":"HOLD"`)) {
		t.Fatalf("duplicate strategy response (%d): %s", second.Code, second.Body.String())
	}

	state, err := store.GetStrategyState(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("load strategy state: %v", err)
	}
	if state.EntryStep != 1 || state.CurrentState != strategy.StatePartialPosition {
		t.Fatalf("unexpected persisted state: %+v", state)
	}
	events, err := store.ListStrategyEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("load strategy events: %v", err)
	}
	if len(events) != 1 || events[0].Action != string(strategy.ActionBuy40) || events[0].Price != 96 || !events[0].Timestamp.Equal(candles[len(candles)-1].Timestamp) {
		t.Fatalf("strategy events = %+v", events)
	}
}
