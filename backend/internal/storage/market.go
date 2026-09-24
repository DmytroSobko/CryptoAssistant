package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

// LatestSnapshot reads the most recently persisted current price. Current
// prices are stored without OHLC fields so they cannot be confused with a
// completed daily candle.
func (s *Store) LatestSnapshot(ctx context.Context, symbol string) (market.Snapshot, error) {
	symbol, err := marketAsset(symbol)
	if err != nil {
		return market.Snapshot{}, err
	}
	var snapshot market.Snapshot
	var updatedAt string
	err = s.db.QueryRowContext(ctx, `SELECT asset, price, timestamp FROM market_snapshots WHERE asset = ? AND open IS NULL ORDER BY timestamp DESC LIMIT 1`, symbol).Scan(&snapshot.Symbol, &snapshot.Price, &updatedAt)
	if err != nil {
		return market.Snapshot{}, fmt.Errorf("get latest %s snapshot: %w", symbol, err)
	}
	snapshot.UpdatedAt, err = parseDatabaseTime(updatedAt)
	if err != nil {
		return market.Snapshot{}, fmt.Errorf("parse latest %s timestamp: %w", symbol, err)
	}
	return snapshot, nil
}

// SaveCurrentPrice persists a point-in-time, intraday display price. It does
// not participate in daily-candle based strategy decisions.
func (s *Store) SaveCurrentPrice(ctx context.Context, snapshot market.Snapshot) error {
	symbol, err := marketAsset(snapshot.Symbol)
	if err != nil {
		return err
	}
	if !validPositive(snapshot.Price) {
		return fmt.Errorf("current %s price must be positive", symbol)
	}
	if snapshot.UpdatedAt.IsZero() {
		return fmt.Errorf("current %s price timestamp is required", symbol)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO market_snapshots(asset, timestamp, price)
		VALUES (?, ?, ?)
		ON CONFLICT(asset, timestamp) DO UPDATE SET price = excluded.price,
			open = NULL, high = NULL, low = NULL, close = NULL, volume = NULL`,
		symbol, databaseTime(snapshot.UpdatedAt), snapshot.Price)
	if err != nil {
		return fmt.Errorf("save current %s price: %w", symbol, err)
	}
	return nil
}

// SaveDailyCandles upserts completed UTC daily OHLC candles. The close is
// duplicated into price to preserve the table's existing snapshot shape.
func (s *Store) SaveDailyCandles(ctx context.Context, symbol string, candles []market.Candle) error {
	symbol, err := marketAsset(symbol)
	if err != nil {
		return err
	}
	if len(candles) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save %s candles: %w", symbol, err)
	}
	defer tx.Rollback()

	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO market_snapshots(asset, timestamp, price, open, high, low, close, volume)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(asset, timestamp) DO UPDATE SET
			price = excluded.price, open = excluded.open, high = excluded.high,
			low = excluded.low, close = excluded.close, volume = excluded.volume`)
	if err != nil {
		return fmt.Errorf("prepare save %s candles: %w", symbol, err)
	}
	defer statement.Close()

	for _, candle := range candles {
		if err := validateCandle(candle); err != nil {
			return fmt.Errorf("invalid %s candle: %w", symbol, err)
		}
		if _, err := statement.ExecContext(ctx, symbol, databaseTime(candle.Timestamp), candle.Close, candle.Open, candle.High, candle.Low, candle.Close, candle.Volume); err != nil {
			return fmt.Errorf("save %s candle: %w", symbol, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s candles: %w", symbol, err)
	}
	return nil
}

// ListDailyCandles returns persisted completed daily candles in chronological
// order. A non-positive limit returns all available candles.
func (s *Store) ListDailyCandles(ctx context.Context, symbol string, limit int) ([]market.Candle, error) {
	symbol, err := marketAsset(symbol)
	if err != nil {
		return nil, err
	}
	query := `SELECT timestamp, open, high, low, close, volume FROM market_snapshots WHERE asset = ? AND open IS NOT NULL ORDER BY timestamp DESC`
	args := []any{symbol}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list %s candles: %w", symbol, err)
	}
	defer rows.Close()

	candles := make([]market.Candle, 0)
	for rows.Next() {
		var candle market.Candle
		var timestamp string
		var volume sql.NullFloat64
		if err := rows.Scan(&timestamp, &candle.Open, &candle.High, &candle.Low, &candle.Close, &volume); err != nil {
			return nil, fmt.Errorf("scan %s candle: %w", symbol, err)
		}
		candle.Timestamp, err = parseDatabaseTime(timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse %s candle timestamp: %w", symbol, err)
		}
		if volume.Valid {
			candle.Volume = volume.Float64
		}
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s candles: %w", symbol, err)
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Timestamp.Before(candles[j].Timestamp) })
	return candles, nil
}

func marketAsset(symbol string) (string, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol != "BTC" && symbol != "ETH" {
		return "", fmt.Errorf("market asset must be BTC or ETH")
	}
	return symbol, nil
}

func validateCandle(candle market.Candle) error {
	if candle.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	if !validPositive(candle.Open) || !validPositive(candle.High) || !validPositive(candle.Low) || !validPositive(candle.Close) {
		return fmt.Errorf("OHLC prices must be positive")
	}
	if candle.Low > candle.High || candle.Open < candle.Low || candle.Open > candle.High || candle.Close < candle.Low || candle.Close > candle.High {
		return fmt.Errorf("OHLC values are inconsistent")
	}
	if math.IsNaN(candle.Volume) || math.IsInf(candle.Volume, 0) || candle.Volume < 0 {
		return fmt.Errorf("volume must be non-negative")
	}
	return nil
}

func validPositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func databaseTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
