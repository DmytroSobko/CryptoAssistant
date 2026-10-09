package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/ath"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

// StrategyEvaluationInput is the complete persisted input to one deterministic
// strategy evaluation. IntradayPrice is used only by the opt-in exit alert.
type StrategyEvaluationInput struct {
	Candles       []market.Candle
	Position      portfolio.Asset
	Config        strategy.Config
	State         strategy.PersistedState
	IntradayPrice float64
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
	if pending, found, err := pendingRecommendation(ctx, tx, asset, input.Config.ResolvedStrategyID(), input.Config.ATHReferencePeak); err != nil {
		return strategy.Result{}, err
	} else if found {
		if staleHistoricalPeakEntry(pending, input) && len(input.Candles) > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE strategy_recommendations SET status = 'DISMISSED', resolved_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'PENDING'`, pending.RecommendationID); err != nil {
				return strategy.Result{}, fmt.Errorf("dismiss stale historical-peak recommendation: %w", err)
			}
			// A buy recommendation advances the advisory state when it is created.
			// If an unexecuted historical-peak entry becomes invalid after changing
			// its rule, start the flat cycle over rather than treating it as bought.
			input.State = strategy.PersistedState{
				Asset: asset, StrategyID: input.Config.ResolvedStrategyID(),
				CurrentState: strategy.StateWaitingForReentry,
				ReentryAfter: input.Candles[len(input.Candles)-1].Timestamp,
			}
		} else {
			if err := tx.Commit(); err != nil {
				return strategy.Result{}, err
			}
			return pending, nil
		}
	}
	result, next := evaluate(input)
	result.RecommendationRuleFingerprint = historicalPeakEntryFingerprint(result, input.Config)
	if err := saveStrategyState(ctx, tx, next); err != nil {
		return strategy.Result{}, err
	}
	if actionableStrategyAction(result.Action) {
		if len(input.Candles) == 0 {
			return strategy.Result{}, fmt.Errorf("record %s strategy action: completed daily candle is required", asset)
		}
		event := StrategyEvent{
			Asset: asset, Timestamp: input.Candles[len(input.Candles)-1].Timestamp,
			StrategyID: input.Config.ResolvedStrategyID(),
			Action:     string(result.Action), Price: result.Price, Reason: result.Reason, State: string(result.State),
			EventKey: strategyEventKey(asset, input.Config.ResolvedStrategyID(), input.Candles[len(input.Candles)-1].Timestamp, result),
		}
		if err := appendStrategyEvent(ctx, tx, event); err != nil {
			return strategy.Result{}, err
		}
		pendingID, err := createPendingRecommendation(ctx, tx, input.Config.ResolvedStrategyID(), result)
		if err != nil {
			return strategy.Result{}, err
		}
		result.RecommendationID, result.RecommendationStatus = pendingID, "PENDING"
	}
	if err := tx.Commit(); err != nil {
		return strategy.Result{}, fmt.Errorf("commit %s strategy evaluation: %w", asset, err)
	}
	return result, nil
}

// staleHistoricalPeakEntry identifies an unexecuted historical-peak buy that
// was created under settings that no longer permit its original signal price.
// Exit recommendations remain explicit user decisions and are never removed
// automatically.
func staleHistoricalPeakEntry(pending strategy.Result, input StrategyEvaluationInput) bool {
	if input.Position.Quantity > 0 || !strings.HasPrefix(pending.Reason, "ATH entry override:") {
		return false
	}
	switch pending.Action {
	case strategy.ActionBuy, strategy.ActionBuy40, strategy.ActionBuy30, strategy.ActionBuy30Final:
	default:
		return false
	}
	if !input.Config.ATHEntryOverrideEnabled || input.Config.ATHReferencePeak <= 0 || input.Config.ATHEntryThresholdPct <= 0 {
		return true
	}
	return pending.RecommendationRuleFingerprint == "" ||
		pending.RecommendationRuleFingerprint != historicalPeakEntryFingerprint(pending, input.Config) ||
		pending.Price > input.Config.ATHReferencePeak*input.Config.ATHEntryThresholdPct/100
}

// historicalPeakEntryFingerprint changes whenever the saved configuration for
// an override-based entry changes. The reference average itself is deliberately
// excluded: it evolves with the eligible historical data and is checked against
// the pending signal price separately above.
func historicalPeakEntryFingerprint(result strategy.Result, config strategy.Config) string {
	if !strings.HasPrefix(result.Reason, "ATH entry override:") {
		return ""
	}
	config.ATHReferencePeak = 0
	payload, err := json.Marshal(config)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func createPendingRecommendation(ctx context.Context, tx *sql.Tx, strategyID strategy.StrategyID, result strategy.Result) (int64, error) {
	response, err := tx.ExecContext(ctx, `INSERT INTO strategy_recommendations(asset, strategy_id, action, action_pct, price, state, reason, next_condition, intraday_alert, rule_fingerprint) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, result.Asset, strategyID, result.Action, result.ActionPct, result.Price, result.State, result.Reason, result.NextCondition, result.IntradayAlert, result.RecommendationRuleFingerprint)
	if err != nil {
		return 0, fmt.Errorf("create pending recommendation: %w", err)
	}
	return response.LastInsertId()
}

func pendingRecommendation(ctx context.Context, tx *sql.Tx, asset string, strategyID strategy.StrategyID, historicalPeakAverage float64) (strategy.Result, bool, error) {
	var result strategy.Result
	var intraday int
	err := tx.QueryRowContext(ctx, `SELECT id, action, action_pct, price, state, reason, next_condition, intraday_alert, rule_fingerprint FROM strategy_recommendations WHERE asset = ? AND strategy_id = ? AND status = 'PENDING' ORDER BY id DESC LIMIT 1`, asset, strategyID).Scan(&result.RecommendationID, &result.Action, &result.ActionPct, &result.Price, &result.State, &result.Reason, &result.NextCondition, &intraday, &result.RecommendationRuleFingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return strategy.Result{}, false, nil
	}
	if err != nil {
		return strategy.Result{}, false, fmt.Errorf("load pending recommendation: %w", err)
	}
	result.Asset, result.StrategyID, result.IntradayAlert, result.RecommendationStatus = asset, strategyID, intraday != 0, "PENDING"
	result.HistoricalPeakAverage = historicalPeakAverage
	return result, true, nil
}

func (s *Store) ResolveStrategyRecommendation(ctx context.Context, id int64, status string) error {
	if status != "EXECUTED" && status != "DISMISSED" {
		return fmt.Errorf("recommendation status must be EXECUTED or DISMISSED")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recommendation resolution: %w", err)
	}
	defer tx.Rollback()
	var asset string
	var strategyID strategy.StrategyID
	var action strategy.Action
	var actionPct, price float64
	if err := tx.QueryRowContext(ctx, `SELECT asset, strategy_id, action, action_pct, price FROM strategy_recommendations WHERE id = ? AND status = 'PENDING'`, id).Scan(&asset, &strategyID, &action, &actionPct, &price); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("pending recommendation not found")
		}
		return fmt.Errorf("load pending recommendation: %w", err)
	}
	if status == "EXECUTED" {
		state, err := getStrategyStateForStrategy(ctx, tx, asset, strategyID)
		if err != nil {
			return err
		}
		if action == strategy.ActionSellProfit {
			state.ProfitTaken = true
		} else if action == strategy.ActionSellDrawdown {
			settings, err := getAssetStrategySettings(ctx, tx, asset)
			if err != nil {
				return err
			}
			config := settings.RecoveryBreakout
			if strategyID == strategy.StrategyDipAccumulation {
				config = settings.DipAccumulation
			}
			markExecutedDrawdown(&state, config, price)
		}
		if actionPct >= 100 {
			state = strategy.PersistedState{Asset: asset, StrategyID: strategyID, CurrentState: strategy.StateWaitingForReentry, ReentryAfter: time.Now().UTC()}
		}
		if err := saveStrategyState(ctx, tx, state); err != nil {
			return err
		}
	}
	response, err := tx.ExecContext(ctx, `UPDATE strategy_recommendations SET status = ?, resolved_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'PENDING'`, status, id)
	if err != nil {
		return fmt.Errorf("resolve recommendation: %w", err)
	}
	count, err := response.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("pending recommendation not found")
	}
	return tx.Commit()
}

// markExecutedDrawdown consumes exactly the levels crossed by the resolved
// sell. This is deliberately done on explicit user acknowledgement, never
// when an intraday alert is merely displayed.
func markExecutedDrawdown(state *strategy.PersistedState, config strategy.Config, price float64) {
	if state.HighestPrice <= 0 || price <= 0 {
		return
	}
	drawdown := (price/state.HighestPrice - 1) * 100
	if drawdown <= config.Drawdown3Pct {
		state.Drawdown1Triggered, state.Drawdown2Triggered, state.Drawdown3Triggered = true, true, true
		return
	}
	if drawdown <= config.Drawdown2Pct {
		state.Drawdown1Triggered, state.Drawdown2Triggered = true, true
		return
	}
	if drawdown <= config.Drawdown1Pct {
		state.Drawdown1Triggered = true
	}
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
	settings, err := getAssetStrategySettings(ctx, tx, asset)
	if err != nil {
		return StrategyEvaluationInput{}, fmt.Errorf("load %s strategy settings: %w", asset, err)
	}
	config, err := settings.ActiveConfig()
	if err != nil {
		return StrategyEvaluationInput{}, fmt.Errorf("resolve %s strategy config: %w", asset, err)
	}
	config.ATHEntryOverrideEnabled = settings.ATHEntryOverride.Enabled
	config.ATHEntryThresholdPct = settings.ATHEntryOverride.ThresholdPct
	config.ATHPeakCount = settings.ATHEntryOverride.PeakCount
	config.ATHSourceFile = settings.ATHEntryOverride.SourceFile
	if len(candles) > 0 {
		peak, selected, peakErr := ath.AverageSpacedPeaksBefore(config.ATHSourceFile, candles[len(candles)-1].Timestamp, config.ATHPeakCount)
		if peakErr != nil {
			return StrategyEvaluationInput{}, peakErr
		}
		if selected > 0 {
			config.ATHReferencePeak = peak
		}
	}
	state, err := getStrategyState(ctx, tx, asset)
	if err != nil {
		return StrategyEvaluationInput{}, err
	}
	var intradayPrice float64
	_ = tx.QueryRowContext(ctx, `SELECT price FROM market_snapshots WHERE asset = ? AND open IS NULL ORDER BY timestamp DESC LIMIT 1`, asset).Scan(&intradayPrice)
	return StrategyEvaluationInput{Candles: candles, Position: position, Config: config, State: state, IntradayPrice: intradayPrice}, nil
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

func strategyEventKey(asset string, strategyID strategy.StrategyID, timestamp time.Time, result strategy.Result) string {
	// The action, resulting state, and percentage distinguish separate stages
	// that happen to be evaluated from the same completed daily candle.
	return fmt.Sprintf("%s|%s|%s|%s|%s|%.8f", asset, strategyID, databaseTime(timestamp), result.Action, result.State, result.ActionPct)
}
