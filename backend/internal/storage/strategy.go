package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

// GetStrategyState returns the state for the asset's currently selected
// strategy. Each strategy has an independent state row, so selecting one does
// not overwrite the other strategy's cycle.
func (s *Store) GetStrategyState(ctx context.Context, asset string) (strategy.PersistedState, error) {
	config, err := s.GetStrategyConfig(ctx, asset)
	if err != nil {
		return strategy.PersistedState{}, err
	}
	return getStrategyStateForStrategy(ctx, s.db, asset, config.ResolvedStrategyID())
}

func getStrategyState(ctx context.Context, db queryRower, asset string) (strategy.PersistedState, error) {
	config, err := getStrategyConfig(ctx, db, asset)
	if err != nil {
		return strategy.PersistedState{}, err
	}
	return getStrategyStateForStrategy(ctx, db, asset, config.ResolvedStrategyID())
}

func getStrategyStateForStrategy(ctx context.Context, db queryRower, asset string, strategyID strategy.StrategyID) (strategy.PersistedState, error) {
	asset, err := strategyAsset(asset)
	if err != nil {
		return strategy.PersistedState{}, err
	}
	if strategyID == "" {
		strategyID = strategy.StrategyRecoveryBreakout
	}
	state := strategy.PersistedState{Asset: asset, StrategyID: strategyID, CurrentState: strategy.StateCash}
	var currentState string
	var firstEntryReferenceAt, lastDipEntryAt, correctionHighAt, lastBreakoutHighAt, lastBreakoutHigherLowAt, reentryAfter sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT state,
			COALESCE(local_high, 0), COALESCE(local_low, 0), COALESCE(higher_low, 0),
			COALESCE(highest_price, 0), COALESCE(drawdown_pct, 0), COALESCE(first_entry_reference_price, 0),
			first_entry_reference_at, last_dip_entry_at,
			correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
			entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open
		FROM strategy_states_v2 WHERE asset = ? AND strategy_id = ?`, asset, strategyID).Scan(
		&currentState,
		&state.LocalHigh, &state.LocalLow, &state.HigherLow, &state.HighestPrice, &state.DrawdownPct, &state.FirstEntryReferencePrice,
		&firstEntryReferenceAt, &lastDipEntryAt,
		&correctionHighAt, &lastBreakoutHighAt, &lastBreakoutHigherLowAt, &reentryAfter,
		&state.EntryStep, &state.ProfitTaken, &state.Drawdown1Triggered, &state.Drawdown2Triggered, &state.Drawdown3Triggered, &state.PositionOpen,
	)
	if err == sql.ErrNoRows {
		return state, nil
	}
	if err != nil {
		return strategy.PersistedState{}, fmt.Errorf("get %s %s strategy state: %w", asset, strategyID, err)
	}
	state.CurrentState = strategy.State(currentState)
	for _, field := range []struct {
		name   string
		value  sql.NullString
		target *time.Time
	}{
		{"first entry reference", firstEntryReferenceAt, &state.FirstEntryReferenceAt},
		{"last dip entry", lastDipEntryAt, &state.LastDipEntryAt},
		{"correction high", correctionHighAt, &state.CorrectionHighAt},
		{"breakout high", lastBreakoutHighAt, &state.LastBreakoutHighAt},
		{"breakout higher-low", lastBreakoutHigherLowAt, &state.LastBreakoutHigherLowAt},
		{"re-entry", reentryAfter, &state.ReentryAfter},
	} {
		parsed, parseErr := nullableDatabaseTime(field.value)
		if parseErr != nil {
			return strategy.PersistedState{}, fmt.Errorf("parse %s %s timestamp: %w", asset, field.name, parseErr)
		}
		*field.target = parsed
	}
	return state, nil
}

// SaveStrategyState persists only engine state. It does not evaluate a
// strategy, mutate a portfolio, or create an event.
func (s *Store) SaveStrategyState(ctx context.Context, state strategy.PersistedState) error {
	return saveStrategyState(ctx, s.db, state)
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func saveStrategyState(ctx context.Context, db execer, state strategy.PersistedState) error {
	asset, err := strategyAsset(state.Asset)
	if err != nil {
		return err
	}
	strategyID := state.StrategyID
	if strategyID == "" {
		strategyID = strategy.StrategyRecoveryBreakout
	}
	if strategyID != strategy.StrategyRecoveryBreakout && strategyID != strategy.StrategyDipAccumulation {
		return fmt.Errorf("invalid strategy ID %q", strategyID)
	}
	if !validStrategyState(state.CurrentState) {
		return fmt.Errorf("invalid strategy state %q", state.CurrentState)
	}
	if state.EntryStep < 0 || state.EntryStep > 3 {
		return fmt.Errorf("strategy entry step must be between 0 and 3")
	}
	for _, value := range []float64{state.LocalHigh, state.LocalLow, state.HigherLow, state.HighestPrice, state.DrawdownPct, state.FirstEntryReferencePrice} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("strategy state values must be finite")
		}
	}
	if strategyID == strategy.StrategyDipAccumulation && state.EntryStep > 0 && (math.IsNaN(state.FirstEntryReferencePrice) || math.IsInf(state.FirstEntryReferencePrice, 0) || state.FirstEntryReferencePrice <= 0) {
		return fmt.Errorf("dip accumulation reference price must be finite and positive")
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO strategy_states_v2 (
			asset, strategy_id, state, local_high, local_low, higher_low, highest_price, drawdown_pct,
			first_entry_reference_price, first_entry_reference_at, last_dip_entry_at,
			correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
			entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(asset, strategy_id) DO UPDATE SET
			state = excluded.state, local_high = excluded.local_high, local_low = excluded.local_low,
			higher_low = excluded.higher_low, highest_price = excluded.highest_price, drawdown_pct = excluded.drawdown_pct,
			first_entry_reference_price = excluded.first_entry_reference_price, first_entry_reference_at = excluded.first_entry_reference_at,
			last_dip_entry_at = excluded.last_dip_entry_at, correction_high_at = excluded.correction_high_at,
			last_breakout_high_at = excluded.last_breakout_high_at, last_breakout_higher_low_at = excluded.last_breakout_higher_low_at,
			reentry_after = excluded.reentry_after, entry_step = excluded.entry_step, profit_taken = excluded.profit_taken,
			drawdown1_triggered = excluded.drawdown1_triggered, drawdown2_triggered = excluded.drawdown2_triggered,
			drawdown3_triggered = excluded.drawdown3_triggered, position_open = excluded.position_open, updated_at = CURRENT_TIMESTAMP`,
		asset, strategyID, state.CurrentState, state.LocalHigh, state.LocalLow, state.HigherLow, state.HighestPrice, state.DrawdownPct,
		state.FirstEntryReferencePrice, nullableTime(state.FirstEntryReferenceAt), nullableTime(state.LastDipEntryAt),
		nullableTime(state.CorrectionHighAt), nullableTime(state.LastBreakoutHighAt), nullableTime(state.LastBreakoutHigherLowAt), nullableTime(state.ReentryAfter),
		state.EntryStep, state.ProfitTaken, state.Drawdown1Triggered, state.Drawdown2Triggered, state.Drawdown3Triggered, state.PositionOpen,
	)
	if err != nil {
		return fmt.Errorf("save %s %s strategy state: %w", asset, strategyID, err)
	}
	return nil
}

// AppendStrategyEvent records an advisory strategy result idempotently. Callers
// decide which results are meaningful events; it never triggers alerts.
func (s *Store) AppendStrategyEvent(ctx context.Context, event StrategyEvent) error {
	return appendStrategyEvent(ctx, s.db, event)
}

func appendStrategyEvent(ctx context.Context, db execer, event StrategyEvent) error {
	asset, err := strategyAsset(event.Asset)
	if err != nil {
		return err
	}
	strategyID := event.StrategyID
	if strategyID == "" {
		strategyID = strategy.StrategyRecoveryBreakout
	}
	if strategyID != strategy.StrategyRecoveryBreakout && strategyID != strategy.StrategyDipAccumulation {
		return fmt.Errorf("invalid strategy event ID %q", strategyID)
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
	if event.EventKey == "" {
		event.EventKey = fmt.Sprintf("%s|%s|%s|%s|%s|%.8f", asset, strategyID, databaseTime(event.Timestamp), event.Action, event.State, event.Price)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO strategy_events(asset, strategy_id, timestamp, action, price, reason, state, event_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(event_key) DO NOTHING`,
		asset, strategyID, databaseTime(event.Timestamp), event.Action, event.Price, event.Reason, event.State, event.EventKey)
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
