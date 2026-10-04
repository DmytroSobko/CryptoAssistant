package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDrawdownExitDefaultsMigrationUpdatesExistingDefaultProfiles(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.DB().Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"001_initial.sql",
		"002_strategy_state_engine_fields.sql",
		"003_strategy_event_idempotency.sql",
		"004_backtesting.sql",
		"005_backtest_run_result_state.sql",
		"006_backtest_break_even_floor.sql",
		"007_per_asset_strategy_profiles.sql",
		"008_ath_entry_override.sql",
	} {
		contents, err := os.ReadFile(filepath.Join("../../migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().Exec(string(contents)); err != nil {
			t.Fatalf("apply existing %s: %v", name, err)
		}
		if _, err := store.DB().Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
			t.Fatal(err)
		}
	}

	// Reproduce a database created with the former 20/30/70 defaults.
	if _, err := store.DB().Exec(`
		UPDATE asset_strategy_settings
		SET recovery_breakout_config_json = json_set(recovery_breakout_config_json, '$.drawdown1SellPct', 20, '$.drawdown2SellPct', 30, '$.drawdown3SellPct', 70),
		    dip_accumulation_config_json = json_set(dip_accumulation_config_json, '$.drawdown1SellPct', 20, '$.drawdown2SellPct', 30, '$.drawdown3SellPct', 70)
		WHERE asset = 'BTC'`); err != nil {
		t.Fatal(err)
	}
	// A customized ladder must remain untouched.
	if _, err := store.DB().Exec(`
		UPDATE asset_strategy_settings
		SET recovery_breakout_config_json = json_set(recovery_breakout_config_json, '$.drawdown1SellPct', 25, '$.drawdown2SellPct', 50, '$.drawdown3SellPct', 100)
		WHERE asset = 'ETH'`); err != nil {
		t.Fatal(err)
	}

	migration, err := os.ReadFile("../../migrations/009_drawdown_exit_defaults.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(string(migration)); err != nil {
		t.Fatalf("upgrade database: %v", err)
	}

	btc, err := store.GetAssetStrategySettings(context.Background(), "BTC")
	if err != nil {
		t.Fatal(err)
	}
	for name, config := range map[string]struct{ first, second, third float64 }{
		"recovery": {btc.RecoveryBreakout.Drawdown1SellPct, btc.RecoveryBreakout.Drawdown2SellPct, btc.RecoveryBreakout.Drawdown3SellPct},
		"dip":      {btc.DipAccumulation.Drawdown1SellPct, btc.DipAccumulation.Drawdown2SellPct, btc.DipAccumulation.Drawdown3SellPct},
	} {
		if config.first != 33.33 || config.second != 50 || config.third != 100 {
			t.Fatalf("%s drawdown exits = %+v; want 33.33/50/100", name, config)
		}
	}

	eth, err := store.GetAssetStrategySettings(context.Background(), "ETH")
	if err != nil {
		t.Fatal(err)
	}
	if eth.RecoveryBreakout.Drawdown1SellPct != 25 || eth.RecoveryBreakout.Drawdown2SellPct != 50 || eth.RecoveryBreakout.Drawdown3SellPct != 100 {
		t.Fatalf("customized ETH recovery exits were overwritten: %+v", eth.RecoveryBreakout)
	}
}
