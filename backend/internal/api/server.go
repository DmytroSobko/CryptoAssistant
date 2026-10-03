// Package api provides the HTTP boundary for the desktop client. Handlers are
// intentionally thin; strategy decisions remain in internal/strategy.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtestservice"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategyservice"
)

// MarketRefresher updates locally persisted public market data. Keeping this
// small interface at the HTTP boundary avoids coupling API handlers to a
// particular provider while leaving strategy evaluation independent of it.
type MarketRefresher interface {
	RefreshAll(context.Context) error
}

type Server struct {
	store           *storage.Store
	cfg             config.Config
	marketRefresher MarketRefresher
	strategyService *strategyservice.Service
	backtestService *backtestservice.Service
}

func NewServer(store *storage.Store, cfg config.Config, marketRefresher MarketRefresher) *Server {
	return &Server{store: store, cfg: cfg, marketRefresher: marketRefresher, strategyService: strategyservice.New(store), backtestService: backtestservice.New(store)}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/market/{asset}", s.handleMarket)
	mux.HandleFunc("POST /api/market/refresh", s.handleRefreshMarket)
	mux.HandleFunc("GET /api/strategy/{asset}", s.handleStrategy)
	mux.HandleFunc("GET /api/portfolio", s.handleGetPortfolio)
	mux.HandleFunc("PUT /api/portfolio", s.handlePutPortfolio)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config/{asset}", s.handlePutConfig)
	mux.HandleFunc("GET /api/strategy-settings", s.handleGetStrategySettings)
	mux.HandleFunc("PUT /api/strategy-settings/{asset}", s.handlePutStrategySettings)
	mux.HandleFunc("POST /api/backtest/candle-sets", s.handleImportBacktestCandleSet)
	mux.HandleFunc("GET /api/backtest/candle-sets", s.handleListBacktestCandleSets)
	mux.HandleFunc("POST /api/backtests", s.handleCreateBacktest)
	mux.HandleFunc("GET /api/backtests", s.handleListBacktests)
	mux.HandleFunc("GET /api/backtests/{id}", s.handleGetBacktest)
	mux.HandleFunc("DELETE /api/backtests/{id}", s.handleDeleteBacktest)
	return withCORS(withLogging(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMarket(w http.ResponseWriter, r *http.Request) {
	asset, ok := validAsset(r.PathValue("asset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "asset must be BTC or ETH")
		return
	}
	snapshot, err := s.store.LatestSnapshot(r.Context(), asset)
	if err != nil {
		writeError(w, http.StatusNotFound, "market data has not been loaded yet")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

// handleRefreshMarket refreshes public display prices and completed daily
// candles. It deliberately does not evaluate a strategy or alter a portfolio.
func (s *Server) handleRefreshMarket(w http.ResponseWriter, r *http.Request) {
	if s.marketRefresher == nil {
		writeError(w, http.StatusServiceUnavailable, "market refresh is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 70*time.Second)
	defer cancel()
	if err := s.marketRefresher.RefreshAll(ctx); err != nil {
		log.Printf("manual market refresh: %v", err)
		writeError(w, http.StatusBadGateway, "could not refresh market data; existing prices were kept")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStrategy(w http.ResponseWriter, r *http.Request) {
	asset, ok := validAsset(r.PathValue("asset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "asset must be BTC or ETH")
		return
	}
	result, err := s.strategyService.Evaluate(r.Context(), asset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not evaluate strategy")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetPortfolio(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.GetPortfolio(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load portfolio")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePutPortfolio(w http.ResponseWriter, r *http.Request) {
	var input portfolio.Portfolio
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.CashBalance < 0 {
		writeError(w, http.StatusBadRequest, "cashBalance cannot be negative")
		return
	}
	if err := s.store.SavePortfolio(r.Context(), input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.ListStrategyEvents(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load strategy history")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	btc, err := s.store.GetStrategyConfig(r.Context(), "BTC")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load BTC configuration")
		return
	}
	eth, err := s.store.GetStrategyConfig(r.Context(), "ETH")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load ETH configuration")
		return
	}
	writeJSON(w, http.StatusOK, map[string]strategy.Config{"BTC": btc, "ETH": eth})
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	asset, ok := validAsset(r.PathValue("asset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "asset must be BTC or ETH")
		return
	}
	var input strategy.Config
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := strategy.ValidateConfig(input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SaveStrategyConfig(r.Context(), asset, input); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save configuration")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetStrategySettings exposes the complete per-asset profile used by
// the forthcoming selector UI. The existing /api/config endpoint remains a
// compatibility view of the currently active configuration.
func (s *Server) handleGetStrategySettings(w http.ResponseWriter, r *http.Request) {
	btc, err := s.store.GetAssetStrategySettings(r.Context(), "BTC")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load BTC strategy settings")
		return
	}
	eth, err := s.store.GetAssetStrategySettings(r.Context(), "ETH")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load ETH strategy settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]strategy.AssetStrategySettings{"BTC": btc, "ETH": eth})
}

func (s *Server) handlePutStrategySettings(w http.ResponseWriter, r *http.Request) {
	asset, ok := validAsset(r.PathValue("asset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "asset must be BTC or ETH")
		return
	}
	var input strategy.AssetStrategySettings
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := input.Normalized(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SaveAssetStrategySettings(r.Context(), asset, input); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save strategy settings")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validAsset(asset string) (string, bool) {
	asset = strings.ToUpper(asset)
	return asset, asset == "BTC" || asset == "ETH"
}

func decodeJSON(r *http.Request, target any) error {
	return decodeJSONLimit(r, target, 1<<20)
}

func decodeJSONLimit(r *http.Request, target any, maxBytes int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:1420")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("%s %s", r.Method, r.URL.Path)
	})
}
