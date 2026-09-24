// Package api provides the HTTP boundary for the desktop client. Handlers are
// intentionally thin; strategy decisions remain in internal/strategy.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

type Server struct {
	store *storage.Store
	cfg   config.Config
}

func NewServer(store *storage.Store, cfg config.Config) *Server {
	return &Server{store: store, cfg: cfg}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/market/{asset}", s.handleMarket)
	mux.HandleFunc("GET /api/strategy/{asset}", s.handleStrategy)
	mux.HandleFunc("GET /api/portfolio", s.handleGetPortfolio)
	mux.HandleFunc("PUT /api/portfolio", s.handlePutPortfolio)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config/{asset}", s.handlePutConfig)
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

func (s *Server) handleStrategy(w http.ResponseWriter, r *http.Request) {
	asset, ok := validAsset(r.PathValue("asset"))
	if !ok {
		writeError(w, http.StatusBadRequest, "asset must be BTC or ETH")
		return
	}
	writeJSON(w, http.StatusNotImplemented, strategy.Result{
		Asset:         asset,
		Action:        strategy.ActionWait,
		State:         strategy.StateCash,
		Reason:        "Strategy evaluation will be enabled after the market-data and rule-engine phases are implemented.",
		NextCondition: "Load daily candles and evaluate the configured rules.",
	})
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
	if err := s.store.SaveStrategyConfig(r.Context(), asset, input); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save configuration")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validAsset(asset string) (string, bool) {
	asset = strings.ToUpper(asset)
	return asset, asset == "BTC" || asset == "ETH"
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
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
