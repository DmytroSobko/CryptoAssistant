package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	DefaultPriceHistoryRecordLimit = 3000
	MinPriceHistoryRecordLimit     = 1
	MaxPriceHistoryRecordLimit     = 100000
)

type PricePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
}

func priceHistorySettingKey(symbol string) string {
	return "price_history_limit_" + symbol
}

func priceHistoryRecordLimit(ctx context.Context, db queryRower, symbol string) (int, error) {
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, priceHistorySettingKey(symbol)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultPriceHistoryRecordLimit, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load %s price history record limit: %w", symbol, err)
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < MinPriceHistoryRecordLimit || limit > MaxPriceHistoryRecordLimit {
		return 0, fmt.Errorf("stored %s price history record limit %q is invalid", symbol, raw)
	}
	return limit, nil
}

func (s *Store) PriceHistoryRecordLimit(ctx context.Context, symbol string) (int, error) {
	symbol, err := marketAsset(symbol)
	if err != nil {
		return 0, err
	}
	return priceHistoryRecordLimit(ctx, s.db, symbol)
}

func (s *Store) SavePriceHistoryRecordLimit(ctx context.Context, symbol string, limit int) error {
	symbol, err := marketAsset(symbol)
	if err != nil {
		return err
	}
	if limit < MinPriceHistoryRecordLimit || limit > MaxPriceHistoryRecordLimit {
		return fmt.Errorf("price history record limit must be between %d and %d", MinPriceHistoryRecordLimit, MaxPriceHistoryRecordLimit)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save %s price history record limit: %w", symbol, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, priceHistorySettingKey(symbol), strconv.Itoa(limit)); err != nil {
		return fmt.Errorf("save %s price history record limit: %w", symbol, err)
	}
	if err := prunePriceHistory(ctx, tx, symbol, limit); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s price history record limit: %w", symbol, err)
	}
	return nil
}

func (s *Store) ListPriceHistory(ctx context.Context, symbol string, since time.Time) ([]PricePoint, error) {
	symbol, err := marketAsset(symbol)
	if err != nil {
		return nil, err
	}
	query := `SELECT timestamp, price FROM market_price_history WHERE asset = ?`
	args := []any{symbol}
	if !since.IsZero() {
		query += ` AND timestamp >= ?`
		args = append(args, databaseTime(since))
	}
	query += ` ORDER BY timestamp ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list %s price history: %w", symbol, err)
	}
	defer rows.Close()
	points := make([]PricePoint, 0)
	for rows.Next() {
		var point PricePoint
		var timestamp string
		if err := rows.Scan(&timestamp, &point.Price); err != nil {
			return nil, fmt.Errorf("scan %s price history: %w", symbol, err)
		}
		point.Timestamp, err = parseDatabaseTime(timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse %s price history timestamp: %w", symbol, err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s price history: %w", symbol, err)
	}
	return points, nil
}

type contextExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func prunePriceHistory(ctx context.Context, db contextExecer, symbol string, limit int) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM market_price_history WHERE id IN (SELECT id FROM market_price_history WHERE asset = ? ORDER BY timestamp DESC, id DESC LIMIT -1 OFFSET ?)`, symbol, limit); err != nil {
		return fmt.Errorf("prune %s price history: %w", symbol, err)
	}
	return nil
}
