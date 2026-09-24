PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS portfolio (
  symbol TEXT PRIMARY KEY CHECK (symbol IN ('BTC', 'ETH')),
  quantity REAL NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  average_entry_price REAL NOT NULL DEFAULT 0 CHECK (average_entry_price >= 0),
  cash_allocated REAL NOT NULL DEFAULT 0 CHECK (cash_allocated >= 0),
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH', 'CASH')),
  type TEXT NOT NULL CHECK (type IN ('BUY', 'SELL', 'DEPOSIT', 'WITHDRAWAL')),
  quantity REAL NOT NULL DEFAULT 0,
  price REAL NOT NULL DEFAULT 0,
  timestamp TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS market_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  timestamp TEXT NOT NULL,
  price REAL NOT NULL,
  open REAL,
  high REAL,
  low REAL,
  close REAL,
  volume REAL,
  UNIQUE(asset, timestamp)
);

CREATE TABLE IF NOT EXISTS strategy_states (
  asset TEXT PRIMARY KEY CHECK (asset IN ('BTC', 'ETH')),
  state TEXT NOT NULL,
  local_high REAL,
  local_low REAL,
  higher_low REAL,
  highest_price REAL,
  drawdown_pct REAL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS strategy_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  timestamp TEXT NOT NULL,
  action TEXT NOT NULL,
  price REAL NOT NULL,
  reason TEXT NOT NULL,
  state TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_key TEXT NOT NULL UNIQUE,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  event_type TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  delivered_at TEXT
);

INSERT OR IGNORE INTO portfolio(symbol) VALUES ('BTC');
INSERT OR IGNORE INTO portfolio(symbol) VALUES ('ETH');
INSERT OR IGNORE INTO settings(key, value) VALUES ('cash_balance', '0');

