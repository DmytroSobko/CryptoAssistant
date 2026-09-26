package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func DefaultStrategyConfig(symbol string) strategy.Config {
	if symbol == "ETH" {
		return strategy.Config{PullbackMinPct: 15, PivotLeft: 2, PivotRight: 2, TrendMode: "STRICT", Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30, ProfitTrigger1Pct: 15, ProfitTrigger2Pct: 20, ProfitTakePct: 25, Drawdown1Pct: -12, Drawdown1SellPct: 20, Drawdown2Pct: -18, Drawdown2SellPct: 30, Drawdown3Pct: -25, Drawdown3SellPct: 70}
	}
	return strategy.Config{PullbackMinPct: 10, PivotLeft: 2, PivotRight: 2, TrendMode: "STRICT", Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30, ProfitTrigger1Pct: 10, ProfitTrigger2Pct: 20, ProfitTakePct: 25, Drawdown1Pct: -10, Drawdown1SellPct: 20, Drawdown2Pct: -15, Drawdown2SellPct: 30, Drawdown3Pct: -20, Drawdown3SellPct: 70}
}

func (s *Store) GetStrategyConfig(ctx context.Context, symbol string) (strategy.Config, error) {
	return getStrategyConfig(ctx, s.db, symbol)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getStrategyConfig(ctx context.Context, db queryRower, symbol string) (strategy.Config, error) {
	key := "strategy_config_" + symbol
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultStrategyConfig(symbol), nil
	}
	if err != nil {
		return strategy.Config{}, err
	}
	var cfg strategy.Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return strategy.Config{}, fmt.Errorf("decode %s: %w", key, err)
	}
	return cfg, nil
}

func (s *Store) SaveStrategyConfig(ctx context.Context, symbol string, config strategy.Config) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, "strategy_config_"+symbol, string(raw))
	return err
}
