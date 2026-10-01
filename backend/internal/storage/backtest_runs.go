package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
	"github.com/google/uuid"
)

// BacktestRun is the stored, auditable counterpart to a pure Result.
type BacktestRun struct {
	ID          string          `json:"id"`
	CandleSetID string          `json:"candleSetId"`
	CreatedAt   time.Time       `json:"createdAt"`
	CompletedAt time.Time       `json:"completedAt"`
	Result      backtest.Result `json:"result"`
}

type BacktestRunSummary struct {
	ID                 string              `json:"id"`
	CandleSetID        string              `json:"candleSetId"`
	Asset              string              `json:"asset"`
	Start              time.Time           `json:"start"`
	End                time.Time           `json:"end"`
	StartingCashUSD    float64             `json:"startingCashUsd"`
	ReturnPct          float64             `json:"returnPct"`
	MaximumDrawdownPct float64             `json:"maximumDrawdownPct"`
	StrategyID         strategy.StrategyID `json:"strategyId"`
	CreatedAt          time.Time           `json:"createdAt"`
	CompletedAt        time.Time           `json:"completedAt"`
}

// SaveBacktestRun atomically records one complete completed simulation. It
// only writes the isolated backtest_* tables.
func (s *Store) SaveBacktestRun(ctx context.Context, candleSetID string, result backtest.Result) (BacktestRun, error) {
	if err := backtest.ValidateRequest(result.Request); err != nil {
		return BacktestRun{}, fmt.Errorf("validate backtest run: %w", err)
	}
	if result.DataFingerprint == "" {
		return BacktestRun{}, fmt.Errorf("backtest data fingerprint is required")
	}
	if err := validateBacktestResult(result); err != nil {
		return BacktestRun{}, err
	}
	configJSON, err := json.Marshal(result.Request.StrategyConfig)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("encode strategy config: %w", err)
	}
	assumptionsJSON, err := json.Marshal(result.Assumptions)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("encode assumptions: %w", err)
	}
	summaryJSON, err := json.Marshal(result.Summary)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("encode summary: %w", err)
	}
	finalStateJSON, err := json.Marshal(result.FinalState)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("encode final state: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("begin save backtest run: %w", err)
	}
	defer tx.Rollback()
	var candleSetAsset string
	if err := tx.QueryRowContext(ctx, `SELECT asset FROM backtest_candle_sets WHERE id = ?`, candleSetID).Scan(&candleSetAsset); err != nil {
		return BacktestRun{}, fmt.Errorf("get backtest candle set: %w", err)
	}
	if candleSetAsset != result.Request.Asset {
		return BacktestRun{}, fmt.Errorf("backtest candle-set asset does not match run asset")
	}
	run := BacktestRun{ID: uuid.NewString(), CandleSetID: candleSetID, Result: result}
	if _, err := tx.ExecContext(ctx, `INSERT INTO backtest_runs(id, candle_set_id, asset, start_timestamp, end_timestamp, starting_cash_usd, config_json, assumptions_json, data_fingerprint, status, completed_at, summary_json, final_state_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'COMPLETED', CURRENT_TIMESTAMP, ?, ?)`, run.ID, candleSetID, result.Request.Asset, databaseTime(result.Request.Start), databaseTime(result.Request.End), result.Request.StartingCashUSD, string(configJSON), string(assumptionsJSON), result.DataFingerprint, string(summaryJSON), string(finalStateJSON)); err != nil {
		return BacktestRun{}, fmt.Errorf("insert backtest run: %w", err)
	}
	if err := insertBacktestSignals(ctx, tx, run.ID, result.Signals); err != nil {
		return BacktestRun{}, err
	}
	if err := insertBacktestTrades(ctx, tx, run.ID, result.Trades); err != nil {
		return BacktestRun{}, err
	}
	if err := insertBacktestEquity(ctx, tx, run.ID, result.EquityCurve); err != nil {
		return BacktestRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return BacktestRun{}, fmt.Errorf("commit backtest run: %w", err)
	}
	created, completed, err := s.backtestRunTimes(ctx, run.ID)
	if err != nil {
		return BacktestRun{}, err
	}
	run.CreatedAt, run.CompletedAt = created, completed
	return run, nil
}

func (s *Store) GetBacktestRun(ctx context.Context, id string) (BacktestRun, error) {
	var run BacktestRun
	var asset, start, end, configJSON, assumptionsJSON, fingerprint, status, summaryJSON, finalStateJSON, created, completed string
	var startingCash float64
	err := s.db.QueryRowContext(ctx, `SELECT id, candle_set_id, asset, start_timestamp, end_timestamp, starting_cash_usd, config_json, assumptions_json, data_fingerprint, status, summary_json, final_state_json, created_at, completed_at FROM backtest_runs WHERE id = ?`, id).Scan(&run.ID, &run.CandleSetID, &asset, &start, &end, &startingCash, &configJSON, &assumptionsJSON, &fingerprint, &status, &summaryJSON, &finalStateJSON, &created, &completed)
	if err != nil {
		return BacktestRun{}, err
	}
	if status != "COMPLETED" {
		return BacktestRun{}, fmt.Errorf("backtest run %s is not completed", id)
	}
	if err := json.Unmarshal([]byte(configJSON), &run.Result.Request.StrategyConfig); err != nil {
		return BacktestRun{}, fmt.Errorf("decode backtest config: %w", err)
	}
	if err := json.Unmarshal([]byte(assumptionsJSON), &run.Result.Assumptions); err != nil {
		return BacktestRun{}, fmt.Errorf("decode backtest assumptions: %w", err)
	}
	if err := json.Unmarshal([]byte(summaryJSON), &run.Result.Summary); err != nil {
		return BacktestRun{}, fmt.Errorf("decode backtest summary: %w", err)
	}
	if err := json.Unmarshal([]byte(finalStateJSON), &run.Result.FinalState); err != nil {
		return BacktestRun{}, fmt.Errorf("decode final state: %w", err)
	}
	var errTime error
	if run.Result.Request.Start, errTime = parseDatabaseTime(start); errTime != nil {
		return BacktestRun{}, fmt.Errorf("parse backtest start: %w", errTime)
	}
	if run.Result.Request.End, errTime = parseDatabaseTime(end); errTime != nil {
		return BacktestRun{}, fmt.Errorf("parse backtest end: %w", errTime)
	}
	if run.CreatedAt, errTime = parseDatabaseTime(created); errTime != nil {
		return BacktestRun{}, fmt.Errorf("parse backtest creation time: %w", errTime)
	}
	if run.CompletedAt, errTime = parseDatabaseTime(completed); errTime != nil {
		return BacktestRun{}, fmt.Errorf("parse backtest completion time: %w", errTime)
	}
	run.Result.Request.Asset, run.Result.Request.ExecutionModel = asset, run.Result.Assumptions.ExecutionModel
	run.Result.Request.FeeBps, run.Result.Request.SlippageBps = run.Result.Assumptions.FeeBps, run.Result.Assumptions.SlippageBps
	run.Result.Request.StartingCashUSD = startingCash
	run.Result.DataFingerprint = fingerprint
	var errRows error
	if run.Result.Signals, errRows = s.backtestSignals(ctx, id); errRows != nil {
		return BacktestRun{}, errRows
	}
	if run.Result.Trades, errRows = s.backtestTrades(ctx, id); errRows != nil {
		return BacktestRun{}, errRows
	}
	if run.Result.EquityCurve, errRows = s.backtestEquity(ctx, id); errRows != nil {
		return BacktestRun{}, errRows
	}
	if err := backtest.ValidateRequest(run.Result.Request); err != nil {
		return BacktestRun{}, fmt.Errorf("stored backtest request is invalid: %w", err)
	}
	return run, nil
}

func (s *Store) ListBacktestRuns(ctx context.Context, limit int) ([]BacktestRunSummary, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, candle_set_id, asset, start_timestamp, end_timestamp, starting_cash_usd, config_json, summary_json, created_at, completed_at FROM backtest_runs WHERE status = 'COMPLETED' ORDER BY created_at DESC, id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list backtest runs: %w", err)
	}
	defer rows.Close()
	runs := make([]BacktestRunSummary, 0)
	for rows.Next() {
		var run BacktestRunSummary
		var start, end, configJSON, summaryJSON, created, completed string
		if err := rows.Scan(&run.ID, &run.CandleSetID, &run.Asset, &start, &end, &run.StartingCashUSD, &configJSON, &summaryJSON, &created, &completed); err != nil {
			return nil, fmt.Errorf("scan backtest run: %w", err)
		}
		if run.Start, err = parseDatabaseTime(start); err != nil {
			return nil, err
		}
		if run.End, err = parseDatabaseTime(end); err != nil {
			return nil, err
		}
		if run.CreatedAt, err = parseDatabaseTime(created); err != nil {
			return nil, err
		}
		if run.CompletedAt, err = parseDatabaseTime(completed); err != nil {
			return nil, err
		}
		var summary backtest.Summary
		if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
			return nil, fmt.Errorf("decode backtest summary: %w", err)
		}
		var strategyConfig strategy.Config
		if err := json.Unmarshal([]byte(configJSON), &strategyConfig); err != nil {
			return nil, fmt.Errorf("decode backtest strategy config: %w", err)
		}
		run.StrategyID = strategyConfig.ResolvedStrategyID()
		run.ReturnPct, run.MaximumDrawdownPct = summary.ReturnPct, summary.MaximumDrawdownPct
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backtest runs: %w", err)
	}
	return runs, nil
}

func insertBacktestSignals(ctx context.Context, tx *sql.Tx, runID string, signals []backtest.Signal) error {
	statement, err := tx.PrepareContext(ctx, `INSERT INTO backtest_signals(run_id, sequence, timestamp, decision_close, action, action_pct, strategy_state, reason, next_condition, order_status, break_even_floor_net_price) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare backtest signals: %w", err)
	}
	defer statement.Close()
	for _, signal := range signals {
		if _, err := statement.ExecContext(ctx, runID, signal.Sequence, databaseTime(signal.Timestamp), signal.DecisionClose, signal.Action, signal.ActionPct, signal.StrategyState, signal.Reason, signal.NextCondition, signal.OrderStatus, nullableFloat(signal.BreakEvenFloorNetPrice)); err != nil {
			return fmt.Errorf("insert backtest signal: %w", err)
		}
	}
	return nil
}

func insertBacktestTrades(ctx context.Context, tx *sql.Tx, runID string, trades []backtest.Trade) error {
	statement, err := tx.PrepareContext(ctx, `INSERT INTO backtest_trades(run_id, sequence, signal_sequence, signal_timestamp, execution_timestamp, side, action, requested_pct, status, raw_open_price, fill_price, quantity, gross_notional_usd, fee_usd, cash_after_usd, quantity_after, average_entry_after, reason, break_even_floor_net_price) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare backtest trades: %w", err)
	}
	defer statement.Close()
	for _, trade := range trades {
		if _, err := statement.ExecContext(ctx, runID, trade.Sequence, trade.SignalSequence, databaseTime(trade.SignalTimestamp), nullableTime(trade.ExecutionTimestamp), trade.Side, trade.Action, trade.RequestedPct, trade.Status, nullableFloat(trade.RawOpenPrice), nullableFloat(trade.FillPrice), trade.Quantity, trade.GrossNotionalUSD, trade.FeeUSD, trade.CashAfterUSD, trade.QuantityAfter, trade.AverageEntryAfter, trade.Reason, nullableFloat(trade.BreakEvenFloorNetPrice)); err != nil {
			return fmt.Errorf("insert backtest trade: %w", err)
		}
	}
	return nil
}

func insertBacktestEquity(ctx context.Context, tx *sql.Tx, runID string, points []backtest.EquityPoint) error {
	statement, err := tx.PrepareContext(ctx, `INSERT INTO backtest_equity_points(run_id, timestamp, close_price, cash_usd, quantity, asset_value_usd, equity_usd, peak_equity_usd, drawdown_pct) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare backtest equity points: %w", err)
	}
	defer statement.Close()
	for _, point := range points {
		if _, err := statement.ExecContext(ctx, runID, databaseTime(point.Timestamp), point.ClosePrice, point.CashUSD, point.Quantity, point.AssetValueUSD, point.EquityUSD, point.PeakEquityUSD, point.DrawdownPct); err != nil {
			return fmt.Errorf("insert backtest equity point: %w", err)
		}
	}
	return nil
}

func (s *Store) backtestRunTimes(ctx context.Context, id string) (time.Time, time.Time, error) {
	var created, completed string
	if err := s.db.QueryRowContext(ctx, `SELECT created_at, completed_at FROM backtest_runs WHERE id = ?`, id).Scan(&created, &completed); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("read backtest run times: %w", err)
	}
	createdTime, err := parseDatabaseTime(created)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	completedTime, err := parseDatabaseTime(completed)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return createdTime, completedTime, nil
}

func validateBacktestResult(result backtest.Result) error {
	if len(result.EquityCurve) == 0 {
		return fmt.Errorf("backtest result must include equity points")
	}
	for _, value := range []float64{result.Summary.EndingEquityUSD, result.Summary.ReturnPct, result.Summary.MaximumDrawdownPct} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("backtest result summary must be finite")
		}
	}
	for _, signal := range result.Signals {
		if signal.Sequence < 1 || signal.Timestamp.IsZero() || signal.Action == "" || signal.StrategyState == "" || signal.Reason == "" || signal.NextCondition == "" || signal.OrderStatus == "" {
			return fmt.Errorf("backtest signal is incomplete")
		}
		if signal.BreakEvenFloorNetPrice < 0 || math.IsNaN(signal.BreakEvenFloorNetPrice) || math.IsInf(signal.BreakEvenFloorNetPrice, 0) {
			return fmt.Errorf("backtest signal break-even floor must be finite and non-negative")
		}
	}
	for _, trade := range result.Trades {
		if trade.Sequence < 1 || trade.SignalTimestamp.IsZero() || trade.Side == "" || trade.Action == "" || trade.Status == "" || trade.Reason == "" {
			return fmt.Errorf("backtest trade is incomplete")
		}
		if trade.BreakEvenFloorNetPrice < 0 || math.IsNaN(trade.BreakEvenFloorNetPrice) || math.IsInf(trade.BreakEvenFloorNetPrice, 0) {
			return fmt.Errorf("backtest trade break-even floor must be finite and non-negative")
		}
	}
	return nil
}

func nullableFloat(value float64) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *Store) backtestSignals(ctx context.Context, runID string) ([]backtest.Signal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence, timestamp, decision_close, action, action_pct, strategy_state, reason, next_condition, order_status, break_even_floor_net_price FROM backtest_signals WHERE run_id = ? ORDER BY sequence ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("get backtest signals: %w", err)
	}
	defer rows.Close()
	values := make([]backtest.Signal, 0)
	for rows.Next() {
		var value backtest.Signal
		var timestamp string
		var floor sql.NullFloat64
		if err := rows.Scan(&value.Sequence, &timestamp, &value.DecisionClose, &value.Action, &value.ActionPct, &value.StrategyState, &value.Reason, &value.NextCondition, &value.OrderStatus, &floor); err != nil {
			return nil, fmt.Errorf("scan backtest signal: %w", err)
		}
		if value.Timestamp, err = parseDatabaseTime(timestamp); err != nil {
			return nil, err
		}
		if floor.Valid {
			value.BreakEvenFloorNetPrice = floor.Float64
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) backtestTrades(ctx context.Context, runID string) ([]backtest.Trade, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence, signal_sequence, signal_timestamp, execution_timestamp, side, action, requested_pct, status, raw_open_price, fill_price, quantity, gross_notional_usd, fee_usd, cash_after_usd, quantity_after, average_entry_after, reason, break_even_floor_net_price FROM backtest_trades WHERE run_id = ? ORDER BY sequence ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("get backtest trades: %w", err)
	}
	defer rows.Close()
	var values []backtest.Trade
	for rows.Next() {
		var value backtest.Trade
		var signalTimestamp string
		var executionTimestamp sql.NullString
		var rawOpenValue, fillValue, floor sql.NullFloat64
		if err := rows.Scan(&value.Sequence, &value.SignalSequence, &signalTimestamp, &executionTimestamp, &value.Side, &value.Action, &value.RequestedPct, &value.Status, &rawOpenValue, &fillValue, &value.Quantity, &value.GrossNotionalUSD, &value.FeeUSD, &value.CashAfterUSD, &value.QuantityAfter, &value.AverageEntryAfter, &value.Reason, &floor); err != nil {
			return nil, fmt.Errorf("scan backtest trade: %w", err)
		}
		if value.SignalTimestamp, err = parseDatabaseTime(signalTimestamp); err != nil {
			return nil, err
		}
		if executionTimestamp.Valid {
			if value.ExecutionTimestamp, err = parseDatabaseTime(executionTimestamp.String); err != nil {
				return nil, err
			}
		}
		if rawOpenValue.Valid {
			value.RawOpenPrice = rawOpenValue.Float64
		}
		if fillValue.Valid {
			value.FillPrice = fillValue.Float64
		}
		if floor.Valid {
			value.BreakEvenFloorNetPrice = floor.Float64
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) backtestEquity(ctx context.Context, runID string) ([]backtest.EquityPoint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT timestamp, close_price, cash_usd, quantity, asset_value_usd, equity_usd, peak_equity_usd, drawdown_pct FROM backtest_equity_points WHERE run_id = ? ORDER BY timestamp ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("get backtest equity: %w", err)
	}
	defer rows.Close()
	values := make([]backtest.EquityPoint, 0)
	for rows.Next() {
		var value backtest.EquityPoint
		var timestamp string
		if err := rows.Scan(&timestamp, &value.ClosePrice, &value.CashUSD, &value.Quantity, &value.AssetValueUSD, &value.EquityUSD, &value.PeakEquityUSD, &value.DrawdownPct); err != nil {
			return nil, fmt.Errorf("scan backtest equity point: %w", err)
		}
		if value.Timestamp, err = parseDatabaseTime(timestamp); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
