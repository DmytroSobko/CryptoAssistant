package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtestservice"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

const maxBacktestCSVBytes = 10 << 20

type importBacktestCandleSetInput struct {
	Asset          string `json:"asset"`
	SourceLabel    string `json:"sourceLabel"`
	SourceFilename string `json:"sourceFilename"`
	CSV            string `json:"csv"`
}

type createBacktestInput struct {
	CandleSetID     string          `json:"candleSetId"`
	Asset           string          `json:"asset"`
	Start           time.Time       `json:"start"`
	End             time.Time       `json:"end"`
	StartingCashUSD float64         `json:"startingCashUsd"`
	StrategyConfig  strategy.Config `json:"strategyConfig"`
	FeeBps          float64         `json:"feeBps"`
	SlippageBps     float64         `json:"slippageBps"`
	ExecutionModel  string          `json:"executionModel"`
}

func (s *Server) handleImportBacktestCandleSet(w http.ResponseWriter, r *http.Request) {
	var input importBacktestCandleSetInput
	if err := decodeJSONLimit(r, &input, maxBacktestCSVBytes); err != nil {
		writeDecodeError(w, err)
		return
	}
	set, err := s.backtestService.ImportCandleSet(r.Context(), backtestservice.ImportRequest{Asset: input.Asset, SourceLabel: input.SourceLabel, SourceFilename: input.SourceFilename, CSV: []byte(input.CSV)})
	if err != nil {
		writeBacktestError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, set)
}

func (s *Server) handleListBacktestCandleSets(w http.ResponseWriter, r *http.Request) {
	sets, err := s.backtestService.ListCandleSets(r.Context(), r.URL.Query().Get("asset"))
	if err != nil {
		writeBacktestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sets)
}

func (s *Server) handleCreateBacktest(w http.ResponseWriter, r *http.Request) {
	s.handleRunBacktest(w, r, true)
}

func (s *Server) handlePreviewBacktest(w http.ResponseWriter, r *http.Request) {
	s.handleRunBacktest(w, r, false)
}

func (s *Server) handleRunBacktest(w http.ResponseWriter, r *http.Request, save bool) {
	var input createBacktestInput
	if err := decodeJSON(r, &input); err != nil {
		writeDecodeError(w, err)
		return
	}
	asset, ok := validAsset(input.Asset)
	if !ok {
		writeError(w, http.StatusBadRequest, "asset must be BTC or ETH")
		return
	}
	runSimulation := s.backtestService.Preview
	if save {
		runSimulation = s.backtestService.Run
	}
	run, err := runSimulation(r.Context(), backtestservice.RunRequest{CandleSetID: input.CandleSetID, Request: backtest.Request{Asset: asset, Start: input.Start, End: input.End, StartingCashUSD: input.StartingCashUSD, StrategyConfig: input.StrategyConfig, FeeBps: input.FeeBps, SlippageBps: input.SlippageBps, ExecutionModel: backtest.ExecutionModel(input.ExecutionModel)}})
	if err != nil {
		writeBacktestError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) handleListBacktests(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = parsed
	}
	runs, err := s.backtestService.ListRuns(r.Context(), limit)
	if err != nil {
		writeBacktestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetBacktest(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusNotFound, "backtest run was not found")
		return
	}
	run, err := s.backtestService.GetRun(r.Context(), id)
	if err != nil {
		writeBacktestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleDeleteBacktest(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusNotFound, "backtest run was not found")
		return
	}
	if err := s.backtestService.DeleteRun(r.Context(), id); err != nil {
		writeBacktestError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeBacktestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, backtestservice.ErrNotFound):
		writeError(w, http.StatusNotFound, "backtest resource was not found")
	case errors.Is(err, backtestservice.ErrInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), backtestservice.ErrInvalid.Error()+": "))
	default:
		writeError(w, http.StatusInternalServerError, "could not process backtest request")
	}
}
