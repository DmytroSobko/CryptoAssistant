CREATE TABLE asset_strategy_settings (
  asset TEXT PRIMARY KEY CHECK (asset IN ('BTC', 'ETH')),
  selected_strategy_id TEXT NOT NULL CHECK (selected_strategy_id IN ('RECOVERY_BREAKOUT', 'DIP_ACCUMULATION')),
  recovery_breakout_config_json TEXT NOT NULL,
  dip_accumulation_config_json TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO asset_strategy_settings(asset, selected_strategy_id, recovery_breakout_config_json, dip_accumulation_config_json)
VALUES
  ('BTC', 'RECOVERY_BREAKOUT',
   COALESCE((SELECT value FROM settings WHERE key = 'strategy_config_BTC'), '{"strategyId":"RECOVERY_BREAKOUT","pullbackMinPct":10,"pivotLeft":2,"pivotRight":2,"breakoutBufferPct":0,"trendMode":"STRICT","entry1Pct":40,"entry2Pct":30,"entry3Pct":30,"profitTrigger1Pct":10,"profitTrigger2Pct":20,"profitTakePct":25,"drawdown1Pct":-10,"drawdown1SellPct":20,"drawdown2Pct":-15,"drawdown2SellPct":30,"drawdown3Pct":-20,"drawdown3SellPct":70}'),
   '{"strategyId":"DIP_ACCUMULATION","pullbackMinPct":10,"pivotLeft":2,"pivotRight":2,"breakoutBufferPct":0,"trendMode":"STRICT","entry1Pct":40,"entry2Pct":30,"entry3Pct":30,"profitTrigger1Pct":10,"profitTrigger2Pct":20,"profitTakePct":25,"drawdown1Pct":-10,"drawdown1SellPct":20,"drawdown2Pct":-15,"drawdown2SellPct":30,"drawdown3Pct":-20,"drawdown3SellPct":70,"entry2DipFromFirstPct":10,"entry3DipFromFirstPct":20,"estimatedSellFeeBps":10,"breakEvenExitFloorEnabled":true}'),
  ('ETH', 'RECOVERY_BREAKOUT',
   COALESCE((SELECT value FROM settings WHERE key = 'strategy_config_ETH'), '{"strategyId":"RECOVERY_BREAKOUT","pullbackMinPct":15,"pivotLeft":2,"pivotRight":2,"breakoutBufferPct":0,"trendMode":"STRICT","entry1Pct":40,"entry2Pct":30,"entry3Pct":30,"profitTrigger1Pct":15,"profitTrigger2Pct":20,"profitTakePct":25,"drawdown1Pct":-12,"drawdown1SellPct":20,"drawdown2Pct":-18,"drawdown2SellPct":30,"drawdown3Pct":-25,"drawdown3SellPct":70}'),
   '{"strategyId":"DIP_ACCUMULATION","pullbackMinPct":15,"pivotLeft":2,"pivotRight":2,"breakoutBufferPct":0,"trendMode":"STRICT","entry1Pct":40,"entry2Pct":30,"entry3Pct":30,"profitTrigger1Pct":15,"profitTrigger2Pct":20,"profitTakePct":25,"drawdown1Pct":-12,"drawdown1SellPct":20,"drawdown2Pct":-18,"drawdown2SellPct":30,"drawdown3Pct":-25,"drawdown3SellPct":70,"entry2DipFromFirstPct":10,"entry3DipFromFirstPct":20,"estimatedSellFeeBps":10,"breakEvenExitFloorEnabled":true}');

CREATE TABLE strategy_states_v2 (
  asset TEXT NOT NULL CHECK (asset IN ('BTC', 'ETH')),
  strategy_id TEXT NOT NULL CHECK (strategy_id IN ('RECOVERY_BREAKOUT', 'DIP_ACCUMULATION')),
  state TEXT NOT NULL,
  local_high REAL,
  local_low REAL,
  higher_low REAL,
  highest_price REAL,
  drawdown_pct REAL,
  first_entry_reference_price REAL,
  first_entry_reference_at TEXT,
  last_dip_entry_at TEXT,
  correction_high_at TEXT,
  last_breakout_high_at TEXT,
  last_breakout_higher_low_at TEXT,
  reentry_after TEXT,
  entry_step INTEGER NOT NULL DEFAULT 0 CHECK (entry_step BETWEEN 0 AND 3),
  profit_taken INTEGER NOT NULL DEFAULT 0 CHECK (profit_taken IN (0, 1)),
  drawdown1_triggered INTEGER NOT NULL DEFAULT 0 CHECK (drawdown1_triggered IN (0, 1)),
  drawdown2_triggered INTEGER NOT NULL DEFAULT 0 CHECK (drawdown2_triggered IN (0, 1)),
  drawdown3_triggered INTEGER NOT NULL DEFAULT 0 CHECK (drawdown3_triggered IN (0, 1)),
  position_open INTEGER NOT NULL DEFAULT 0 CHECK (position_open IN (0, 1)),
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (asset, strategy_id)
);

INSERT OR IGNORE INTO strategy_states_v2(
  asset, strategy_id, state, local_high, local_low, higher_low, highest_price, drawdown_pct,
  correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
  entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open, updated_at
)
SELECT
  asset, 'RECOVERY_BREAKOUT', state, local_high, local_low, higher_low, highest_price, drawdown_pct,
  correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
  entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open, updated_at
FROM strategy_states;

ALTER TABLE strategy_events ADD COLUMN strategy_id TEXT NOT NULL DEFAULT 'RECOVERY_BREAKOUT' CHECK (strategy_id IN ('RECOVERY_BREAKOUT', 'DIP_ACCUMULATION'));

CREATE INDEX idx_strategy_events_asset_strategy_timestamp ON strategy_events(asset, strategy_id, timestamp DESC);
