-- Apply the requested profile values to existing BTC and ETH settings.
-- Saved backtest snapshots and campaign states remain historical records.
UPDATE asset_strategy_settings
SET recovery_breakout_config_json = json_set(recovery_breakout_config_json,
      '$.pullbackMinPct', 15, '$.pivotLeft', 2, '$.pivotRight', 2,
      '$.breakoutBufferPct', 0, '$.trendMode', 'STRICT',
      '$.entry1Pct', 20, '$.entry2Pct', 30, '$.entry3Pct', 50,
      '$.profitTrigger1Pct', 50, '$.profitTrigger2Pct', 0, '$.profitTakePct', 25,
      '$.drawdown1Pct', -5, '$.drawdown1SellPct', 25,
      '$.drawdown2Pct', -8, '$.drawdown2SellPct', 50,
      '$.drawdown3Pct', -10, '$.drawdown3SellPct', 100),
    dip_accumulation_config_json = json_set(dip_accumulation_config_json,
      '$.pullbackMinPct', 15, '$.pivotLeft', 2, '$.pivotRight', 2,
      '$.breakoutBufferPct', 0, '$.trendMode', 'STRICT',
      '$.entry1Pct', 20, '$.entry2Pct', 30, '$.entry3Pct', 50,
      '$.profitTrigger1Pct', 50, '$.profitTrigger2Pct', 0, '$.profitTakePct', 25,
      '$.drawdown1Pct', -5, '$.drawdown1SellPct', 25,
      '$.drawdown2Pct', -8, '$.drawdown2SellPct', 50,
      '$.drawdown3Pct', -10, '$.drawdown3SellPct', 100,
      '$.entry2DipFromFirstPct', 5, '$.entry3DipFromFirstPct', 10),
    ath_entry_override_json = '{"enabled":true,"thresholdPct":60}',
    updated_at = CURRENT_TIMESTAMP;
