CREATE TABLE market_price_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  timestamp TEXT NOT NULL,
  price REAL NOT NULL CHECK (price > 0)
);

CREATE INDEX market_price_history_asset_timestamp_idx
  ON market_price_history(asset, timestamp DESC, id DESC);

INSERT OR IGNORE INTO settings(key, value) VALUES ('price_history_limit_BTC', '3000');
INSERT OR IGNORE INTO settings(key, value) VALUES ('price_history_limit_ETH', '3000');
