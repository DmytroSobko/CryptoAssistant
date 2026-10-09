CREATE TABLE strategy_recommendations (
  id INTEGER PRIMARY KEY,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  strategy_id TEXT NOT NULL CHECK (strategy_id IN ('RECOVERY_BREAKOUT', 'DIP_ACCUMULATION')),
  action TEXT NOT NULL,
  action_pct REAL NOT NULL,
  price REAL NOT NULL,
  state TEXT NOT NULL,
  reason TEXT NOT NULL,
  next_condition TEXT NOT NULL,
  intraday_alert INTEGER NOT NULL DEFAULT 0 CHECK (intraday_alert IN (0, 1)),
  status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'EXECUTED', 'DISMISSED')),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at TEXT
);
CREATE INDEX idx_strategy_recommendations_pending ON strategy_recommendations(asset, strategy_id, status, id DESC);
