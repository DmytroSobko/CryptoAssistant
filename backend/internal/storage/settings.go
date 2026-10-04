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
		return strategy.Config{PullbackMinPct: 35, PivotLeft: 2, PivotRight: 2, TrendMode: "STRICT", Entry1Pct: 20, Entry2Pct: 30, Entry3Pct: 50, ProfitTrigger1Pct: 60, ProfitTakePct: 25, Drawdown1Pct: -5, Drawdown1SellPct: 25, Drawdown2Pct: -7, Drawdown2SellPct: 50, Drawdown3Pct: -10, Drawdown3SellPct: 100}
	}
	return strategy.Config{PullbackMinPct: 15, PivotLeft: 2, PivotRight: 2, TrendMode: "STRICT", Entry1Pct: 20, Entry2Pct: 30, Entry3Pct: 50, ProfitTrigger1Pct: 50, ProfitTakePct: 25, Drawdown1Pct: -5, Drawdown1SellPct: 25, Drawdown2Pct: -8, Drawdown2SellPct: 50, Drawdown3Pct: -10, Drawdown3SellPct: 100}
}

func DefaultDipAccumulationConfig(symbol string) strategy.Config {
	config := DefaultStrategyConfig(symbol)
	config.StrategyID = strategy.StrategyDipAccumulation
	config.Entry2DipFromFirstPct = 5
	config.Entry3DipFromFirstPct = 10
	if symbol == "ETH" {
		config.Entry2DipFromFirstPct = 10
		config.Entry3DipFromFirstPct = 20
	}
	config.EstimatedSellFeeBps = 10
	config.BreakEvenExitFloorEnabled = true
	return config
}

func DefaultAssetStrategySettings(symbol string) strategy.AssetStrategySettings {
	return strategy.AssetStrategySettings{
		SelectedStrategyID: strategy.StrategyRecoveryBreakout,
		RecoveryBreakout:   DefaultStrategyConfig(symbol),
		DipAccumulation:    DefaultDipAccumulationConfig(symbol),
		ATHEntryOverride:   strategy.ATHEntryOverrideSettings{Enabled: true, ThresholdPct: 60},
	}
}

// GetStrategyConfig resolves the active strategy for an asset. Existing
// callers continue to use this compatibility method while the UI migrates to
// the full profile endpoint in the next phase.
func (s *Store) GetStrategyConfig(ctx context.Context, symbol string) (strategy.Config, error) {
	return getStrategyConfig(ctx, s.db, symbol)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getStrategyConfig(ctx context.Context, db queryRower, symbol string) (strategy.Config, error) {
	settings, err := getAssetStrategySettings(ctx, db, symbol)
	if err != nil {
		return strategy.Config{}, err
	}
	config, err := settings.ActiveConfig()
	if err != nil {
		return strategy.Config{}, fmt.Errorf("resolve %s active strategy: %w", symbol, err)
	}
	return config, nil
}

func (s *Store) GetAssetStrategySettings(ctx context.Context, symbol string) (strategy.AssetStrategySettings, error) {
	return getAssetStrategySettings(ctx, s.db, symbol)
}

func getAssetStrategySettings(ctx context.Context, db queryRower, symbol string) (strategy.AssetStrategySettings, error) {
	asset, err := strategyAsset(symbol)
	if err != nil {
		return strategy.AssetStrategySettings{}, err
	}
	var selected, recoveryJSON, dipJSON, overrideJSON string
	err = db.QueryRowContext(ctx, `SELECT selected_strategy_id, recovery_breakout_config_json, dip_accumulation_config_json, ath_entry_override_json FROM asset_strategy_settings WHERE asset = ?`, asset).Scan(&selected, &recoveryJSON, &dipJSON, &overrideJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultAssetStrategySettings(asset).Normalized()
	}
	if err != nil {
		return strategy.AssetStrategySettings{}, fmt.Errorf("get %s strategy settings: %w", asset, err)
	}
	settings := strategy.AssetStrategySettings{SelectedStrategyID: strategy.StrategyID(selected)}
	if err := json.Unmarshal([]byte(recoveryJSON), &settings.RecoveryBreakout); err != nil {
		return strategy.AssetStrategySettings{}, fmt.Errorf("decode %s recovery breakout settings: %w", asset, err)
	}
	if err := json.Unmarshal([]byte(dipJSON), &settings.DipAccumulation); err != nil {
		return strategy.AssetStrategySettings{}, fmt.Errorf("decode %s dip accumulation settings: %w", asset, err)
	}
	if err := json.Unmarshal([]byte(overrideJSON), &settings.ATHEntryOverride); err != nil {
		return strategy.AssetStrategySettings{}, fmt.Errorf("decode %s ATH entry override settings: %w", asset, err)
	}
	normalized, err := settings.Normalized()
	if err != nil {
		return strategy.AssetStrategySettings{}, fmt.Errorf("validate %s strategy settings: %w", asset, err)
	}
	return normalized, nil
}

func (s *Store) SaveAssetStrategySettings(ctx context.Context, symbol string, settings strategy.AssetStrategySettings) error {
	asset, err := strategyAsset(symbol)
	if err != nil {
		return err
	}
	normalized, err := settings.Normalized()
	if err != nil {
		return err
	}
	previous, err := s.GetAssetStrategySettings(ctx, asset)
	if err != nil {
		return err
	}
	recoveryJSON, err := json.Marshal(normalized.RecoveryBreakout)
	if err != nil {
		return fmt.Errorf("encode recovery breakout settings: %w", err)
	}
	dipJSON, err := json.Marshal(normalized.DipAccumulation)
	if err != nil {
		return fmt.Errorf("encode dip accumulation settings: %w", err)
	}
	overrideJSON, err := json.Marshal(normalized.ATHEntryOverride)
	if err != nil {
		return fmt.Errorf("encode ATH entry override settings: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO asset_strategy_settings(asset, selected_strategy_id, recovery_breakout_config_json, dip_accumulation_config_json, ath_entry_override_json, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(asset) DO UPDATE SET selected_strategy_id = excluded.selected_strategy_id, recovery_breakout_config_json = excluded.recovery_breakout_config_json, dip_accumulation_config_json = excluded.dip_accumulation_config_json, ath_entry_override_json = excluded.ath_entry_override_json, updated_at = CURRENT_TIMESTAMP`, asset, normalized.SelectedStrategyID, string(recoveryJSON), string(dipJSON), string(overrideJSON)); err != nil {
		return fmt.Errorf("save %s strategy settings: %w", asset, err)
	}
	if previous.SelectedStrategyID != normalized.SelectedStrategyID {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM strategy_states_v2 WHERE asset = ? AND strategy_id = ?`, asset, normalized.SelectedStrategyID); err != nil {
			return fmt.Errorf("reset %s %s strategy state: %w", asset, normalized.SelectedStrategyID, err)
		}
	}
	return nil
}

// SaveStrategyConfig keeps the original configuration API usable until the
// settings screen gains a profile selector. A missing strategy ID means the
// legacy Recovery Breakout configuration, never an accidental Dip overwrite.
func (s *Store) SaveStrategyConfig(ctx context.Context, symbol string, config strategy.Config) error {
	asset, err := strategyAsset(symbol)
	if err != nil {
		return err
	}
	strategyID := config.ResolvedStrategyID()
	if config.StrategyID == "" {
		strategyID = strategy.StrategyRecoveryBreakout
	}
	config.StrategyID = strategyID
	if err := strategy.ValidateConfig(config); err != nil {
		return err
	}
	settings, err := s.GetAssetStrategySettings(ctx, asset)
	if err != nil {
		return err
	}
	switch strategyID {
	case strategy.StrategyRecoveryBreakout:
		settings.RecoveryBreakout = config
	case strategy.StrategyDipAccumulation:
		settings.DipAccumulation = config
	default:
		return fmt.Errorf("unsupported strategy ID %q", strategyID)
	}
	return s.SaveAssetStrategySettings(ctx, asset, settings)
}
