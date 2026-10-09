package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/google/uuid"
)

const backtestCandleSchemaVersion = 1

var (
	ErrBacktestCandleSetDuplicate = errors.New("backtest candle set already exists")
	ErrBacktestCandleSetInUse     = errors.New("backtest candle set is used by saved runs")
)

// BacktestCandleSet describes an immutable imported historical data set. It
// intentionally has no relationship to the live market_snapshots table.
type BacktestCandleSet struct {
	ID             string    `json:"id"`
	Asset          string    `json:"asset"`
	SourceLabel    string    `json:"sourceLabel"`
	SourceFilename string    `json:"sourceFilename"`
	OriginalSHA256 string    `json:"originalSha256"`
	SchemaVersion  int       `json:"schemaVersion"`
	CandleCount    int       `json:"candleCount"`
	FirstTimestamp time.Time `json:"firstTimestamp"`
	LastTimestamp  time.Time `json:"lastTimestamp"`
	ImportedAt     time.Time `json:"importedAt"`
}

// ImportBacktestCandleSet saves a complete, validated candle set in one
// transaction. Existing imports are never overwritten: their original-byte
// SHA-256 is unique and identifies the immutable source file.
func (s *Store) ImportBacktestCandleSet(ctx context.Context, set BacktestCandleSet, candles []market.Candle) (BacktestCandleSet, error) {
	asset, err := marketAsset(set.Asset)
	if err != nil {
		return BacktestCandleSet{}, err
	}
	if err := validateBacktestSetInput(set, candles); err != nil {
		return BacktestCandleSet{}, err
	}
	sourceLabel := strings.TrimSpace(set.SourceLabel)
	filename := filepath.Base(strings.TrimSpace(set.SourceFilename))
	set = BacktestCandleSet{
		ID: uuid.NewString(), Asset: asset, SourceLabel: sourceLabel, SourceFilename: filename,
		OriginalSHA256: strings.ToLower(set.OriginalSHA256), SchemaVersion: backtestCandleSchemaVersion,
		CandleCount: len(candles), FirstTimestamp: candles[0].Timestamp, LastTimestamp: candles[len(candles)-1].Timestamp,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BacktestCandleSet{}, fmt.Errorf("begin backtest candle-set import: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO backtest_candle_sets(id, asset, source_label, source_filename, original_sha256, schema_version, candle_count, first_timestamp, last_timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, set.ID, set.Asset, set.SourceLabel, set.SourceFilename, set.OriginalSHA256, set.SchemaVersion, set.CandleCount, databaseTime(set.FirstTimestamp), databaseTime(set.LastTimestamp)); err != nil {
		if isUniqueConstraint(err) {
			return BacktestCandleSet{}, fmt.Errorf("%w: original SHA-256", ErrBacktestCandleSetDuplicate)
		}
		return BacktestCandleSet{}, fmt.Errorf("insert backtest candle set: %w", err)
	}
	statement, err := tx.PrepareContext(ctx, `INSERT INTO backtest_candles(candle_set_id, timestamp, open, high, low, close, volume) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return BacktestCandleSet{}, fmt.Errorf("prepare backtest candles: %w", err)
	}
	defer statement.Close()
	for _, candle := range candles {
		if _, err := statement.ExecContext(ctx, set.ID, databaseTime(candle.Timestamp), candle.Open, candle.High, candle.Low, candle.Close, candle.Volume); err != nil {
			return BacktestCandleSet{}, fmt.Errorf("insert backtest candle: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BacktestCandleSet{}, fmt.Errorf("commit backtest candle-set import: %w", err)
	}
	set.ImportedAt, err = s.backtestCandleSetImportedAt(ctx, set.ID)
	if err != nil {
		return BacktestCandleSet{}, err
	}
	return set, nil
}

func (s *Store) ListBacktestCandleSets(ctx context.Context, asset string) ([]BacktestCandleSet, error) {
	query := `SELECT id, asset, source_label, source_filename, original_sha256, schema_version, candle_count, first_timestamp, last_timestamp, imported_at FROM backtest_candle_sets`
	args := []any{}
	if strings.TrimSpace(asset) != "" {
		normalized, err := marketAsset(asset)
		if err != nil {
			return nil, err
		}
		query += ` WHERE asset = ?`
		args = append(args, normalized)
	}
	query += ` ORDER BY imported_at DESC, id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list backtest candle sets: %w", err)
	}
	defer rows.Close()
	sets := make([]BacktestCandleSet, 0)
	for rows.Next() {
		set, err := scanBacktestCandleSet(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backtest candle sets: %w", err)
	}
	return sets, nil
}

// DeleteBacktestCandleSet removes an imported source only when no saved run
// references it. This keeps saved simulations reproducible and auditable.
func (s *Store) DeleteBacktestCandleSet(ctx context.Context, id string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin backtest candle-set deletion: %w", err)
	}
	defer tx.Rollback()
	var runs int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM backtest_runs WHERE candle_set_id = ?`, id).Scan(&runs); err != nil {
		return false, fmt.Errorf("count backtest candle-set references: %w", err)
	}
	if runs > 0 {
		return false, ErrBacktestCandleSetInUse
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM backtest_candle_sets WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete backtest candle set: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted backtest candle sets: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit backtest candle-set deletion: %w", err)
	}
	return deleted > 0, nil
}

func (s *Store) GetBacktestCandleSet(ctx context.Context, id string) (BacktestCandleSet, []market.Candle, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, asset, source_label, source_filename, original_sha256, schema_version, candle_count, first_timestamp, last_timestamp, imported_at FROM backtest_candle_sets WHERE id = ?`, id)
	set, err := scanBacktestCandleSet(row)
	if err != nil {
		return BacktestCandleSet{}, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT timestamp, open, high, low, close, volume FROM backtest_candles WHERE candle_set_id = ? ORDER BY timestamp ASC`, id)
	if err != nil {
		return BacktestCandleSet{}, nil, fmt.Errorf("get backtest candles: %w", err)
	}
	defer rows.Close()
	candles := make([]market.Candle, 0, set.CandleCount)
	for rows.Next() {
		var candle market.Candle
		var timestamp string
		var volume sql.NullFloat64
		if err := rows.Scan(&timestamp, &candle.Open, &candle.High, &candle.Low, &candle.Close, &volume); err != nil {
			return BacktestCandleSet{}, nil, fmt.Errorf("scan backtest candle: %w", err)
		}
		candle.Timestamp, err = parseDatabaseTime(timestamp)
		if err != nil {
			return BacktestCandleSet{}, nil, fmt.Errorf("parse backtest candle timestamp: %w", err)
		}
		if volume.Valid {
			candle.Volume = volume.Float64
		}
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return BacktestCandleSet{}, nil, fmt.Errorf("iterate backtest candles: %w", err)
	}
	if len(candles) != set.CandleCount {
		return BacktestCandleSet{}, nil, fmt.Errorf("backtest candle set %s count mismatch", id)
	}
	if err := backtest.ValidateCandles(candles); err != nil {
		return BacktestCandleSet{}, nil, fmt.Errorf("stored backtest candle set %s is invalid: %w", id, err)
	}
	return set, candles, nil
}

type backtestCandleSetScanner interface{ Scan(...any) error }

func scanBacktestCandleSet(scanner backtestCandleSetScanner) (BacktestCandleSet, error) {
	var set BacktestCandleSet
	var first, last, imported string
	if err := scanner.Scan(&set.ID, &set.Asset, &set.SourceLabel, &set.SourceFilename, &set.OriginalSHA256, &set.SchemaVersion, &set.CandleCount, &first, &last, &imported); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BacktestCandleSet{}, err
		}
		return BacktestCandleSet{}, fmt.Errorf("scan backtest candle set: %w", err)
	}
	var err error
	if set.FirstTimestamp, err = parseDatabaseTime(first); err != nil {
		return BacktestCandleSet{}, fmt.Errorf("parse backtest first timestamp: %w", err)
	}
	if set.LastTimestamp, err = parseDatabaseTime(last); err != nil {
		return BacktestCandleSet{}, fmt.Errorf("parse backtest last timestamp: %w", err)
	}
	if set.ImportedAt, err = parseDatabaseTime(imported); err != nil {
		return BacktestCandleSet{}, fmt.Errorf("parse backtest import timestamp: %w", err)
	}
	return set, nil
}

func (s *Store) backtestCandleSetImportedAt(ctx context.Context, id string) (time.Time, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT imported_at FROM backtest_candle_sets WHERE id = ?`, id).Scan(&value); err != nil {
		return time.Time{}, fmt.Errorf("read backtest candle-set import time: %w", err)
	}
	parsed, err := parseDatabaseTime(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse backtest candle-set import time: %w", err)
	}
	return parsed, nil
}

func validateBacktestSetInput(set BacktestCandleSet, candles []market.Candle) error {
	if strings.TrimSpace(set.SourceLabel) == "" {
		return fmt.Errorf("backtest source label is required")
	}
	filename := filepath.Base(strings.TrimSpace(set.SourceFilename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return fmt.Errorf("backtest source filename is required")
	}
	if len(set.OriginalSHA256) != 64 {
		return fmt.Errorf("backtest original SHA-256 must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(set.OriginalSHA256); err != nil {
		return fmt.Errorf("backtest original SHA-256 must be hexadecimal: %w", err)
	}
	if err := backtest.ValidateCandles(candles); err != nil {
		return fmt.Errorf("invalid backtest candles: %w", err)
	}
	if len(candles) < backtest.MinimumCandleCount {
		return fmt.Errorf("backtest candle set requires at least %d candles", backtest.MinimumCandleCount)
	}
	return nil
}

func isUniqueConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
