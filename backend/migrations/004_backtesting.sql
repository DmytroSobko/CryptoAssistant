CREATE TABLE backtest_candle_sets (
  id TEXT PRIMARY KEY,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  source_label TEXT NOT NULL,
  source_filename TEXT NOT NULL,
  original_sha256 TEXT NOT NULL UNIQUE,
  schema_version INTEGER NOT NULL,
  candle_count INTEGER NOT NULL,
  first_timestamp TEXT NOT NULL,
  last_timestamp TEXT NOT NULL,
  imported_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE backtest_candles (
  candle_set_id TEXT NOT NULL REFERENCES backtest_candle_sets(id) ON DELETE CASCADE,
  timestamp TEXT NOT NULL,
  open REAL NOT NULL,
  high REAL NOT NULL,
  low REAL NOT NULL,
  close REAL NOT NULL,
  volume REAL,
  PRIMARY KEY (candle_set_id, timestamp)
);

CREATE TABLE backtest_runs (
  id TEXT PRIMARY KEY,
  candle_set_id TEXT NOT NULL REFERENCES backtest_candle_sets(id),
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  start_timestamp TEXT NOT NULL,
  end_timestamp TEXT NOT NULL,
  starting_cash_usd REAL NOT NULL,
  config_json TEXT NOT NULL,
  assumptions_json TEXT NOT NULL,
  data_fingerprint TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('COMPLETED', 'FAILED')),
  error_message TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at TEXT
);

CREATE TABLE backtest_signals (
  run_id TEXT NOT NULL REFERENCES backtest_runs(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL,
  timestamp TEXT NOT NULL,
  decision_close REAL NOT NULL,
  action TEXT NOT NULL,
  action_pct REAL NOT NULL,
  strategy_state TEXT NOT NULL,
  reason TEXT NOT NULL,
  next_condition TEXT NOT NULL,
  order_status TEXT NOT NULL,
  PRIMARY KEY (run_id, sequence)
);

CREATE TABLE backtest_trades (
  run_id TEXT NOT NULL REFERENCES backtest_runs(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL,
  signal_sequence INTEGER,
  signal_timestamp TEXT NOT NULL,
  execution_timestamp TEXT,
  side TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
  action TEXT NOT NULL,
  requested_pct REAL NOT NULL,
  status TEXT NOT NULL,
  raw_open_price REAL,
  fill_price REAL,
  quantity REAL NOT NULL,
  gross_notional_usd REAL NOT NULL,
  fee_usd REAL NOT NULL,
  cash_after_usd REAL NOT NULL,
  quantity_after REAL NOT NULL,
  average_entry_after REAL NOT NULL,
  reason TEXT NOT NULL,
  PRIMARY KEY (run_id, sequence)
);

CREATE TABLE backtest_equity_points (
  run_id TEXT NOT NULL REFERENCES backtest_runs(id) ON DELETE CASCADE,
  timestamp TEXT NOT NULL,
  close_price REAL NOT NULL,
  cash_usd REAL NOT NULL,
  quantity REAL NOT NULL,
  asset_value_usd REAL NOT NULL,
  equity_usd REAL NOT NULL,
  peak_equity_usd REAL NOT NULL,
  drawdown_pct REAL NOT NULL,
  PRIMARY KEY (run_id, timestamp)
);

CREATE INDEX idx_backtest_runs_created_at ON backtest_runs(created_at DESC);
CREATE INDEX idx_backtest_candles_set_timestamp ON backtest_candles(candle_set_id, timestamp);
CREATE INDEX idx_backtest_trades_run_sequence ON backtest_trades(run_id, sequence);
