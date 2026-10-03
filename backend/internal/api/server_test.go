package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func TestBacktestEndpointsCreateAuditableRunWithoutChangingLiveResponses(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	handler := NewServer(store, config.Config{}, nil).Routes()

	portfolioBefore := serve(handler, http.MethodGet, "/api/portfolio", nil)
	historyBefore := serve(handler, http.MethodGet, "/api/history", nil)
	strategyBefore := serve(handler, http.MethodGet, "/api/strategy/BTC", nil)
	if portfolioBefore.Code != http.StatusOK || historyBefore.Code != http.StatusOK || strategyBefore.Code != http.StatusOK {
		t.Fatalf("unexpected live baseline responses: portfolio=%d history=%d strategy=%d", portfolioBefore.Code, historyBefore.Code, strategyBefore.Code)
	}

	importBody, err := json.Marshal(importBacktestCandleSetInput{Asset: "BTC", SourceLabel: "test fixture", SourceFilename: "btc-daily.csv", CSV: apiCSVFixture(201)})
	if err != nil {
		t.Fatal(err)
	}
	imported := serve(handler, http.MethodPost, "/api/backtest/candle-sets", importBody)
	if imported.Code != http.StatusCreated {
		t.Fatalf("import status=%d body=%s", imported.Code, imported.Body.String())
	}
	var candleSet storage.BacktestCandleSet
	if err := json.Unmarshal(imported.Body.Bytes(), &candleSet); err != nil {
		t.Fatal(err)
	}

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 199)
	runBody, err := json.Marshal(createBacktestInput{CandleSetID: candleSet.ID, Asset: "BTC", Start: start, End: start.AddDate(0, 0, 1), StartingCashUSD: 10000, StrategyConfig: apiBacktestConfig(), FeeBps: 10, SlippageBps: 5, ExecutionModel: "NEXT_DAILY_OPEN"})
	if err != nil {
		t.Fatal(err)
	}
	created := serve(handler, http.MethodPost, "/api/backtests", runBody)
	if created.Code != http.StatusCreated || !bytes.Contains(created.Body.Bytes(), []byte(`"dataFingerprint"`)) {
		t.Fatalf("run status=%d body=%s", created.Code, created.Body.String())
	}
	var run storage.BacktestRun
	if err := json.Unmarshal(created.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.ID == "" || run.Result.Summary.SignalCount != 2 {
		t.Fatalf("unexpected created run: %+v", run)
	}
	unknownBody, err := json.Marshal(createBacktestInput{CandleSetID: "missing-set", Asset: "BTC", Start: start, End: start.AddDate(0, 0, 1), StartingCashUSD: 10000, StrategyConfig: apiBacktestConfig(), FeeBps: 10, SlippageBps: 5, ExecutionModel: "NEXT_DAILY_OPEN"})
	if err != nil {
		t.Fatal(err)
	}
	unknown := serve(handler, http.MethodPost, "/api/backtests", unknownBody)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown candle set status=%d body=%s", unknown.Code, unknown.Body.String())
	}
	mismatchBody, err := json.Marshal(createBacktestInput{CandleSetID: candleSet.ID, Asset: "ETH", Start: start, End: start.AddDate(0, 0, 1), StartingCashUSD: 10000, StrategyConfig: apiBacktestConfig(), FeeBps: 10, SlippageBps: 5, ExecutionModel: "NEXT_DAILY_OPEN"})
	if err != nil {
		t.Fatal(err)
	}
	mismatch := serve(handler, http.MethodPost, "/api/backtests", mismatchBody)
	if mismatch.Code != http.StatusBadRequest {
		t.Fatalf("asset mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}

	listed := serve(handler, http.MethodGet, "/api/backtests", nil)
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte(run.ID)) {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}
	detail := serve(handler, http.MethodGet, "/api/backtests/"+run.ID, nil)
	if detail.Code != http.StatusOK || !bytes.Contains(detail.Body.Bytes(), []byte(`"equityCurve"`)) {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	missing := serve(handler, http.MethodGet, "/api/backtests/not-a-run", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing run status=%d body=%s", missing.Code, missing.Body.String())
	}
	deleted := serve(handler, http.MethodDelete, "/api/backtests/"+run.ID, nil)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	if response := serve(handler, http.MethodGet, "/api/backtests/"+run.ID, nil); response.Code != http.StatusNotFound {
		t.Fatalf("deleted run status=%d body=%s", response.Code, response.Body.String())
	}
	if response := serve(handler, http.MethodDelete, "/api/backtests/"+run.ID, nil); response.Code != http.StatusNotFound {
		t.Fatalf("repeat delete status=%d body=%s", response.Code, response.Body.String())
	}

	portfolioAfter := serve(handler, http.MethodGet, "/api/portfolio", nil)
	historyAfter := serve(handler, http.MethodGet, "/api/history", nil)
	strategyAfter := serve(handler, http.MethodGet, "/api/strategy/BTC", nil)
	if !bytes.Equal(portfolioBefore.Body.Bytes(), portfolioAfter.Body.Bytes()) || !bytes.Equal(historyBefore.Body.Bytes(), historyAfter.Body.Bytes()) || !bytes.Equal(strategyBefore.Body.Bytes(), strategyAfter.Body.Bytes()) {
		t.Fatalf("backtest changed live responses:\nportfolio %s -> %s\nhistory %s -> %s\nstrategy %s -> %s", portfolioBefore.Body.String(), portfolioAfter.Body.String(), historyBefore.Body.String(), historyAfter.Body.String(), strategyBefore.Body.String(), strategyAfter.Body.String())
	}
}

func TestBacktestEndpointsRejectBadInputAndOversizedCSV(t *testing.T) {
	handler := newTestHandler(t)
	invalid := serve(handler, http.MethodPost, "/api/backtest/candle-sets", []byte(`{"asset":"DOGE","sourceLabel":"x","sourceFilename":"x.csv","csv":"timestamp,open,high,low,close,volume"}`))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid asset status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	over := importBacktestCandleSetInput{Asset: "BTC", SourceLabel: "x", SourceFilename: "x.csv", CSV: strings.Repeat("x", maxBacktestCSVBytes+1)}
	body, err := json.Marshal(over)
	if err != nil {
		t.Fatal(err)
	}
	overResponse := serve(handler, http.MethodPost, "/api/backtest/candle-sets", body)
	if overResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized CSV status=%d body=%s", overResponse.Code, overResponse.Body.String())
	}
}

func TestCORSAllowsBacktestRunDeletion(t *testing.T) {
	response := serve(newTestHandler(t), http.MethodOptions, "/api/backtests/example", nil)
	if response.Code != http.StatusNoContent || !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete) {
		t.Fatalf("delete CORS preflight response=%d methods=%q", response.Code, response.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestStrategySettingsEndpointsSelectAnAssetStrategyWithoutChangingTheOtherAsset(t *testing.T) {
	handler := newTestHandler(t)
	initial := serve(handler, http.MethodGet, "/api/strategy-settings", nil)
	if initial.Code != http.StatusOK {
		t.Fatalf("initial profile status=%d body=%s", initial.Code, initial.Body.String())
	}
	var profiles map[string]strategy.AssetStrategySettings
	if err := json.Unmarshal(initial.Body.Bytes(), &profiles); err != nil {
		t.Fatal(err)
	}
	btc := profiles["BTC"]
	eth := profiles["ETH"]
	btc.SelectedStrategyID = strategy.StrategyDipAccumulation
	btc.DipAccumulation.Entry1Pct = 35
	btc.ATHEntryOverride = strategy.ATHEntryOverrideSettings{Enabled: true, ThresholdPct: 55}
	body, err := json.Marshal(btc)
	if err != nil {
		t.Fatal(err)
	}
	saved := serve(handler, http.MethodPut, "/api/strategy-settings/BTC", body)
	if saved.Code != http.StatusNoContent {
		t.Fatalf("save profile status=%d body=%s", saved.Code, saved.Body.String())
	}

	loaded := serve(handler, http.MethodGet, "/api/strategy-settings", nil)
	if loaded.Code != http.StatusOK {
		t.Fatalf("loaded profile status=%d body=%s", loaded.Code, loaded.Body.String())
	}
	if err := json.Unmarshal(loaded.Body.Bytes(), &profiles); err != nil {
		t.Fatal(err)
	}
	if profiles["BTC"].SelectedStrategyID != strategy.StrategyDipAccumulation || profiles["BTC"].DipAccumulation.Entry1Pct != 35 || !profiles["BTC"].ATHEntryOverride.Enabled || profiles["BTC"].ATHEntryOverride.ThresholdPct != 55 || profiles["ETH"] != eth {
		t.Fatalf("profiles after BTC selection=%+v", profiles)
	}

	active := serve(handler, http.MethodGet, "/api/config", nil)
	if active.Code != http.StatusOK {
		t.Fatalf("active config status=%d body=%s", active.Code, active.Body.String())
	}
	var configs map[string]strategy.Config
	if err := json.Unmarshal(active.Body.Bytes(), &configs); err != nil {
		t.Fatal(err)
	}
	if configs["BTC"].StrategyID != strategy.StrategyDipAccumulation || configs["BTC"].Entry1Pct != 35 || configs["ETH"].StrategyID != strategy.StrategyRecoveryBreakout {
		t.Fatalf("active configs=%+v", configs)
	}
}

func serve(handler http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func apiCSVFixture(count int) string {
	var builder strings.Builder
	builder.WriteString("timestamp,open,high,low,close,volume\n")
	for index := 0; index < count; index++ {
		price := 100 + index
		fmt.Fprintf(&builder, "%s,%d,%d,%d,%d,1\n", time.Date(2020, 1, 1+index, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), price, price+1, price-1, price)
	}
	return builder.String()
}

func apiBacktestConfig() strategy.Config {
	return strategy.Config{PullbackMinPct: 10, PivotLeft: 1, PivotRight: 1, TrendMode: strategy.TrendModeOff, Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30, ProfitTrigger1Pct: 10, ProfitTrigger2Pct: 20, ProfitTakePct: 25, Drawdown1Pct: -10, Drawdown1SellPct: 20, Drawdown2Pct: -15, Drawdown2SellPct: 30, Drawdown3Pct: -20, Drawdown3SellPct: 70}
}

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
	return NewServer(store, config.Config{}, nil).Routes()
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

	invalidConfig := httptest.NewRequest(http.MethodPut, "/api/config/BTC", bytes.NewBufferString(`{"pullbackMinPct":0}`))
	invalidConfig.Header.Set("Content-Type", "application/json")
	invalidConfigResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidConfigResponse, invalidConfig)
	if invalidConfigResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid config status = %d, body = %s", invalidConfigResponse.Code, invalidConfigResponse.Body.String())
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
	handler := NewServer(store, config.Config{}, nil).Routes()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/market/btc", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("market status = %d, body = %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"symbol":"BTC"`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"price":84200.5`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"updatedAt":"2026-01-03T12:00:00Z"`)) {
		t.Fatalf("unexpected market response: %s", response.Body.String())
	}
}

func TestMarketRefreshEndpointRefreshesPublicDataWithoutEvaluatingStrategy(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	refresher := &fakeMarketRefresher{}
	handler := NewServer(store, config.Config{}, refresher).Routes()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/market/refresh", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("market refresh status = %d, body = %s", response.Code, response.Body.String())
	}
	if refresher.calls != 1 {
		t.Fatalf("market refresh calls = %d, want 1", refresher.calls)
	}
	state, err := store.GetStrategyState(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("load strategy state: %v", err)
	}
	if state.CurrentState != strategy.StateCash {
		t.Fatalf("market refresh unexpectedly evaluated strategy: %+v", state)
	}
}

func TestMarketRefreshEndpointReportsProviderFailure(t *testing.T) {
	refresher := &fakeMarketRefresher{err: errors.New("provider unavailable")}
	handler := NewServer(nil, config.Config{}, refresher).Routes()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/market/refresh", nil))
	if response.Code != http.StatusBadGateway || !bytes.Contains(response.Body.Bytes(), []byte("existing prices were kept")) {
		t.Fatalf("market refresh failure response (%d): %s", response.Code, response.Body.String())
	}
}

type fakeMarketRefresher struct {
	calls int
	err   error
}

func (f *fakeMarketRefresher) RefreshAll(context.Context) error {
	f.calls++
	return f.err
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
	handler := NewServer(store, config.Config{}, nil).Routes()
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
