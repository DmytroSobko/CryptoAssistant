ALTER TABLE backtest_runs ADD COLUMN summary_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE backtest_runs ADD COLUMN final_state_json TEXT NOT NULL DEFAULT '{}';
