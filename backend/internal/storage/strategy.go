package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

// GetStrategyState returns the facts needed by the pure strategy engine to
// evaluate an asset deterministically. A state is created only after a caller
// saves one, so an unseen asset starts in CASH.
func (s *Store) GetStrategyState(ctx context.Context, asset string) (strategy.PersistedState, error) {
	asset, err := strategyAsset(asset)
	if err != nil {
		return strategy.PersistedState{}, err
	}

	state := strategy.PersistedState{Asset: asset, CurrentState: strategy.StateCash}
	var currentState string
	var correctionHighAt, lastBreakoutHighAt, lastBreakoutHigherLowAt, reentryAfter sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT state,
			COALESCE(local_high, 0), COALESCE(local_low, 0), COALESCE(higher_low, 0),
			COALESCE(highest_price, 0), COALESCE(drawdown_pct, 0),
			correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
			entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open
		FROM strategy_states WHERE asset = ?`, asset).Scan(
		&currentState,
		&state.LocalHigh, &state.LocalLow, &state.HigherLow, &state.HighestPrice, &state.DrawdownPct,
		&correctionHighAt, &lastBreakoutHighAt, &lastBreakoutHigherLowAt, &reentryAfter,
		&state.EntryStep, &state.ProfitTaken, &state.Drawdown1Triggered, &state.Drawdown2Triggered, &state.Drawdown3Triggered, &state.PositionOpen,
	)
	if err == sql.ErrNoRows {
		return state, nil
	}
	if err != nil {
		return strategy.PersistedState{}, fmt.Errorf("get %s strategy state: %w", asset, err)
	}
	state.CurrentState = strategy.State(currentState)
	if state.CorrectionHighAt, err = nullableDatabaseTime(correctionHighAt); err != nil {
		return strategy.PersistedState{}, fmt.Errorf("parse %s correction high timestamp: %w", asset, err)
	}
	if state.LastBreakoutHighAt, err = nullableDatabaseTime(lastBreakoutHighAt); err != nil {
		return strategy.PersistedState{}, fmt.Errorf("parse %s breakout high timestamp: %w", asset, err)
	}
	if state.LastBreakoutHigherLowAt, err = nullableDatabaseTime(lastBreakoutHigherLowAt); err != nil {
		return strategy.PersistedState{}, fmt.Errorf("parse %s breakout higher-low timestamp: %w", asset, err)
	}
	if state.ReentryAfter, err = nullableDatabaseTime(reentryAfter); err != nil {
		return strategy.PersistedState{}, fmt.Errorf("parse %s re-entry timestamp: %w", asset, err)
	}
	return state, nil
}

// SaveStrategyState persists only engine state. It does not evaluate a
// strategy, mutate a portfolio, or create an event.
func (s *Store) SaveStrategyState(ctx context.Context, state strategy.PersistedState) error {
	asset, err := strategyAsset(state.Asset)
	if err != nil {
		return err
	}
	if !validStrategyState(state.CurrentState) {
		return fmt.Errorf("invalid strategy state %q", state.CurrentState)
	}
	if state.EntryStep < 0 || state.EntryStep > 3 {
		return fmt.Errorf("strategy entry step must be between 0 and 3")
	}
	for _, value := range []float64{state.LocalHigh, state.LocalLow, state.HigherLow, state.HighestPrice, state.DrawdownPct} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("strategy state values must be finite")
		}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO strategy_states (
			asset, state, local_high, local_low, higher_low, highest_price, drawdown_pct,
			correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
			entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(asset) DO UPDATE SET
			state = excluded.state, local_high = excluded.local_high, local_low = excluded.local_low,
			higher_low = excluded.higher_low, highest_price = excluded.highest_price, drawdown_pct = excluded.drawdown_pct,
			correction_high_at = excluded.correction_high_at, last_breakout_high_at = excluded.last_breakout_high_at,
			last_breakout_higher_low_at = excluded.last_breakout_higher_low_at, reentry_after = excluded.reentry_after,
			entry_step = excluded.entry_step, profit_taken = excluded.profit_taken,
			drawdown1_triggered = excluded.drawdown1_triggered, drawdown2_triggered = excluded.drawdown2_triggered,
			drawdown3_triggered = excluded.drawdown3_triggered, position_open = excluded.position_open,
			updated_at = CURRENT_TIMESTAMP`,
		asset, state.CurrentState, state.LocalHigh, state.LocalLow, state.HigherLow, state.HighestPrice, state.DrawdownPct,
		nullableTime(state.CorrectionHighAt), nullableTime(state.LastBreakoutHighAt), nullableTime(state.LastBreakoutHigherLowAt), nullableTime(state.ReentryAfter),
		state.EntryStep, state.ProfitTaken, state.Drawdown1Triggered, state.Drawdown2Triggered, state.Drawdown3Triggered, state.PositionOpen,
	)
	if err != nil {
		return fmt.Errorf("save %s strategy state: %w", asset, err)
	}
	return nil
}

// AppendStrategyEvent records an advisory strategy result. Callers decide
// which results are meaningful events; this method never deduplicates or
// triggers alerts.
func (s *Store) AppendStrategyEvent(ctx context.Context, event StrategyEvent) error {
	asset, err := strategyAsset(event.Asset)
	if err != nil {
		return err
	}
	if event.Timestamp.IsZero() {
		return fmt.Errorf("strategy event timestamp is required")
	}
	if event.Action == "" || event.State == "" || event.Reason == "" {
		return fmt.Errorf("strategy event action, reason, and state are required")
	}
	if math.IsNaN(event.Price) || math.IsInf(event.Price, 0) || event.Price < 0 {
		return fmt.Errorf("strategy event price must be finite and non-negative")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO strategy_events(asset, timestamp, action, price, reason, state) VALUES (?, ?, ?, ?, ?, ?)`,
		asset, databaseTime(event.Timestamp), event.Action, event.Price, event.Reason, event.State)
	if err != nil {
		return fmt.Errorf("append %s strategy event: %w", asset, err)
	}
	return nil
}

func strategyAsset(asset string) (string, error) { return marketAsset(asset) }

func validStrategyState(state strategy.State) bool {
	switch state {
	case strategy.StateCash, strategy.StateCorrection, strategy.StateRecoveryPending, strategy.StateEntryPending,
		strategy.StatePartialPosition, strategy.StateFullPosition, strategy.StateProfitProtection,
		strategy.StateExiting, strategy.StateWaitingForReentry:
		return true
	default:
		return false
	}
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return databaseTime(value)
}

func nullableDatabaseTime(value sql.NullString) (time.Time, error) {
	if !value.Valid || value.String == "" {
		return time.Time{}, nil
	}
	return parseDatabaseTime(value.String)
}
