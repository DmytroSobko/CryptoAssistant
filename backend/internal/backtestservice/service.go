// Package backtestservice coordinates immutable historical data, the pure
// simulator, and isolated persistence. It never reads live portfolio or
// strategy-state tables.
package backtestservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtestdata"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

var (
	ErrNotFound = errors.New("backtest resource not found")
	ErrInvalid  = errors.New("invalid backtest input")
	ErrConflict = errors.New("backtest resource conflict")
)

type ImportRequest struct {
	Asset          string
	SourceLabel    string
	SourceFilename string
	CSV            []byte
}

type RunRequest struct {
	CandleSetID string
	Request     backtest.Request
}

type Service struct{ store *storage.Store }

func New(store *storage.Store) *Service { return &Service{store: store} }

func (s *Service) ImportCandleSet(ctx context.Context, request ImportRequest) (storage.BacktestCandleSet, error) {
	asset := strings.ToUpper(strings.TrimSpace(request.Asset))
	if asset != "BTC" && asset != "ETH" {
		return storage.BacktestCandleSet{}, invalid(fmt.Errorf("asset must be BTC or ETH"))
	}
	if len(request.CSV) == 0 {
		return storage.BacktestCandleSet{}, invalid(fmt.Errorf("CSV is required"))
	}
	if strings.TrimSpace(request.SourceLabel) == "" {
		return storage.BacktestCandleSet{}, invalid(fmt.Errorf("source label is required"))
	}
	filename := filepath.Base(strings.TrimSpace(request.SourceFilename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return storage.BacktestCandleSet{}, invalid(fmt.Errorf("source filename is required"))
	}
	candles, err := backtestdata.ParseCSV(request.CSV)
	if err != nil {
		return storage.BacktestCandleSet{}, invalid(err)
	}
	set, err := s.store.ImportBacktestCandleSet(ctx, storage.BacktestCandleSet{Asset: asset, SourceLabel: request.SourceLabel, SourceFilename: request.SourceFilename, OriginalSHA256: backtestdata.SHA256(request.CSV)}, candles)
	if errors.Is(err, storage.ErrBacktestCandleSetDuplicate) {
		return storage.BacktestCandleSet{}, invalid(err)
	}
	if err != nil {
		return storage.BacktestCandleSet{}, err
	}
	return set, nil
}

func (s *Service) ListCandleSets(ctx context.Context, asset string) ([]storage.BacktestCandleSet, error) {
	if normalized := strings.ToUpper(strings.TrimSpace(asset)); normalized != "" && normalized != "BTC" && normalized != "ETH" {
		return nil, invalid(fmt.Errorf("asset must be BTC or ETH"))
	}
	sets, err := s.store.ListBacktestCandleSets(ctx, asset)
	if err != nil {
		return nil, err
	}
	return sets, nil
}

func (s *Service) DeleteCandleSet(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("backtest candle set: %w", ErrNotFound)
	}
	deleted, err := s.store.DeleteBacktestCandleSet(ctx, id)
	if errors.Is(err, storage.ErrBacktestCandleSetInUse) {
		return fmt.Errorf("%w: delete saved runs that use this candle set first", ErrConflict)
	}
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("backtest candle set %q: %w", id, ErrNotFound)
	}
	return nil
}

func (s *Service) Run(ctx context.Context, request RunRequest) (storage.BacktestRun, error) {
	return s.run(ctx, request, true)
}

// Preview evaluates an isolated simulation without adding it to the archive.
func (s *Service) Preview(ctx context.Context, request RunRequest) (storage.BacktestRun, error) {
	return s.run(ctx, request, false)
}

func (s *Service) run(ctx context.Context, request RunRequest, save bool) (storage.BacktestRun, error) {
	if strings.TrimSpace(request.CandleSetID) == "" {
		return storage.BacktestRun{}, invalid(fmt.Errorf("candle set ID is required"))
	}
	if err := backtest.ValidateRequest(request.Request); err != nil {
		return storage.BacktestRun{}, invalid(err)
	}
	set, candles, err := s.store.GetBacktestCandleSet(ctx, request.CandleSetID)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.BacktestRun{}, fmt.Errorf("candle set %q: %w", request.CandleSetID, ErrNotFound)
	}
	if err != nil {
		return storage.BacktestRun{}, err
	}
	if set.Asset != request.Request.Asset {
		return storage.BacktestRun{}, invalid(fmt.Errorf("candle-set asset must match run asset"))
	}
	result, err := backtest.Run(request.Request, candles)
	if err != nil {
		return storage.BacktestRun{}, invalid(err)
	}
	if !save {
		return storage.BacktestRun{CandleSetID: set.ID, Result: result}, nil
	}
	run, err := s.store.SaveBacktestRun(ctx, set.ID, result)
	if err != nil {
		return storage.BacktestRun{}, err
	}
	return run, nil
}

func (s *Service) GetRun(ctx context.Context, id string) (storage.BacktestRun, error) {
	run, err := s.store.GetBacktestRun(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.BacktestRun{}, fmt.Errorf("backtest run %q: %w", id, ErrNotFound)
	}
	return run, err
}

func (s *Service) DeleteRun(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("backtest run: %w", ErrNotFound)
	}
	deleted, err := s.store.DeleteBacktestRun(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("backtest run %q: %w", id, ErrNotFound)
	}
	return nil
}

func (s *Service) DeleteAllRuns(ctx context.Context) (int64, error) {
	return s.store.DeleteAllBacktestRuns(ctx)
}

func (s *Service) ListRuns(ctx context.Context, limit int) ([]storage.BacktestRunSummary, error) {
	return s.store.ListBacktestRuns(ctx, limit)
}

func invalid(err error) error { return fmt.Errorf("%w: %v", ErrInvalid, err) }
