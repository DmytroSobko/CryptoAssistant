package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

// StrategyEvaluationInput is the complete persisted input to one deterministic
// strategy evaluation. In particular, it deliberately has no current price.
type StrategyEvaluationInput struct {
	Candles  []market.Candle
	Position portfolio.Asset
	Config   strategy.Config
	State    strategy.PersistedState
}

// EvaluateStrategyAtomically reads the inputs, invokes evaluate, and saves the
// resulting state and any actionable recommendation in one transaction. Calls
// are serialized in-process so concurrent HTTP requests cannot evaluate the
// same prior state twice. Event keys make a retried transaction idempotent.
func (s *Store) EvaluateStrategyAtomically(ctx context.Context, asset string, evaluate func(StrategyEvaluationInput) (strategy.Result, strategy.PersistedState)) (strategy.Result, error) {
	asset, err := strategyAsset(asset)
	if err != nil {
		return strategy.Result{}, err
	}

	s.strategyMu.Lock()
	defer s.strategyMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return strategy.Result{}, fmt.Errorf("begin %s strategy evaluation: %w", asset, err)
	}
	defer tx.Rollback()

	input, err := strategyEvaluationInput(ctx, tx, asset)
	if err != nil {
		return strategy.Result{}, err
	}
	result, next := evaluate(input)
	if err := saveStrategyState(ctx, tx, next); err != nil {
		return strategy.Result{}, err
	}
	if actionableStrategyAction(result.Action) {
		if len(input.Candles) == 0 {
			return strategy.Result{}, fmt.Errorf("record %s strategy action: completed daily candle is required", asset)
		}
		event := StrategyEvent{
			Asset: asset, Timestamp: input.Candles[len(input.Candles)-1].Timestamp,
			Action: string(result.Action), Price: result.Price, Reason: result.Reason, State: string(result.State),
			EventKey: strategyEventKey(asset, input.Candles[len(input.Candles)-1].Timestamp, result),
		}
		if err := appendStrategyEvent(ctx, tx, event); err != nil {
			return strategy.Result{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return strategy.Result{}, fmt.Errorf("commit %s strategy evaluation: %w", asset, err)
	}
	return result, nil
}

func strategyEvaluationInput(ctx context.Context, tx *sql.Tx, asset string) (StrategyEvaluationInput, error) {
	candles, err := listDailyCandles(ctx, tx, asset)
	if err != nil {
		return StrategyEvaluationInput{}, err
	}
	position := portfolio.Asset{Symbol: asset}
	var updatedAt string
	err = tx.QueryRowContext(ctx, `SELECT symbol, quantity, average_entry_price, cash_allocated, updated_at FROM portfolio WHERE symbol = ?`, asset).Scan(
		&position.Symbol, &position.Quantity, &position.AverageEntryPrice, &position.CashAllocated, &updatedAt,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return StrategyEvaluationInput{}, fmt.Errorf("load %s portfolio position: %w", asset, err)
	}
	if err == nil {
		position.UpdatedAt, err = parseDatabaseTime(updatedAt)
		if err != nil {
			return StrategyEvaluationInput{}, fmt.Errorf("parse %s portfolio timestamp: %w", asset, err)
		}
	}
	config, err := getStrategyConfig(ctx, tx, asset)
	if err != nil {
		return StrategyEvaluationInput{}, fmt.Errorf("load %s strategy config: %w", asset, err)
	}
	state, err := getStrategyState(ctx, tx, asset)
	if err != nil {
		return StrategyEvaluationInput{}, err
	}
	return StrategyEvaluationInput{Candles: candles, Position: position, Config: config, State: state}, nil
}

func listDailyCandles(ctx context.Context, tx *sql.Tx, asset string) ([]market.Candle, error) {
	rows, err := tx.QueryContext(ctx, `SELECT timestamp, open, high, low, close, volume FROM market_snapshots WHERE asset = ? AND open IS NOT NULL ORDER BY timestamp DESC`, asset)
	if err != nil {
		return nil, fmt.Errorf("list %s candles: %w", asset, err)
	}
	defer rows.Close()

	candles := make([]market.Candle, 0)
	for rows.Next() {
		var candle market.Candle
		var timestamp string
		var volume sql.NullFloat64
		if err := rows.Scan(&timestamp, &candle.Open, &candle.High, &candle.Low, &candle.Close, &volume); err != nil {
			return nil, fmt.Errorf("scan %s candle: %w", asset, err)
		}
		candle.Timestamp, err = parseDatabaseTime(timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse %s candle timestamp: %w", asset, err)
		}
		if volume.Valid {
			candle.Volume = volume.Float64
		}
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s candles: %w", asset, err)
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Timestamp.Before(candles[j].Timestamp) })
	return candles, nil
}

func actionableStrategyAction(action strategy.Action) bool {
	switch action {
	case strategy.ActionBuy, strategy.ActionBuy40, strategy.ActionBuy30, strategy.ActionBuy30Final,
		strategy.ActionSellProfit, strategy.ActionSellDrawdown, strategy.ActionSellDrawdown10,
		strategy.ActionSellDrawdown15, strategy.ActionSellDrawdown20:
		return true
	default:
		return false
	}
}

func strategyEventKey(asset string, timestamp time.Time, result strategy.Result) string {
	// The action, resulting state, and percentage distinguish separate stages
	// that happen to be evaluated from the same completed daily candle.
	return fmt.Sprintf("%s|%s|%s|%s|%.8f", asset, databaseTime(timestamp), result.Action, result.State, result.ActionPct)
}
