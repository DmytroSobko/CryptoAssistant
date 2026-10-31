package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

const (
	MinPriceRefreshIntervalSeconds = 1
	MaxPriceRefreshIntervalSeconds = 86400
	priceRefreshIntervalKey        = "price_refresh_interval_seconds"
)

func (s *Store) PriceRefreshIntervalSeconds(ctx context.Context, fallback int) (int, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, priceRefreshIntervalKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load price refresh interval: %w", err)
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < MinPriceRefreshIntervalSeconds || seconds > MaxPriceRefreshIntervalSeconds {
		return 0, fmt.Errorf("stored price refresh interval %q is invalid", raw)
	}
	return seconds, nil
}

func (s *Store) SavePriceRefreshIntervalSeconds(ctx context.Context, seconds int) error {
	if seconds < MinPriceRefreshIntervalSeconds || seconds > MaxPriceRefreshIntervalSeconds {
		return fmt.Errorf("price refresh interval must be between %d and %d seconds", MinPriceRefreshIntervalSeconds, MaxPriceRefreshIntervalSeconds)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO settings(key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, priceRefreshIntervalKey, strconv.Itoa(seconds)); err != nil {
		return fmt.Errorf("save price refresh interval: %w", err)
	}
	return nil
}
