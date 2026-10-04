UPDATE asset_strategy_settings
SET recovery_breakout_config_json = json_set(
      recovery_breakout_config_json,
      '$.drawdown1SellPct', 33.33,
      '$.drawdown2SellPct', 50,
      '$.drawdown3SellPct', 100
    ),
    updated_at = CURRENT_TIMESTAMP
WHERE json_extract(recovery_breakout_config_json, '$.drawdown1SellPct') = 20
  AND json_extract(recovery_breakout_config_json, '$.drawdown2SellPct') = 30
  AND json_extract(recovery_breakout_config_json, '$.drawdown3SellPct') = 70;

UPDATE asset_strategy_settings
SET dip_accumulation_config_json = json_set(
      dip_accumulation_config_json,
      '$.drawdown1SellPct', 33.33,
      '$.drawdown2SellPct', 50,
      '$.drawdown3SellPct', 100
    ),
    updated_at = CURRENT_TIMESTAMP
WHERE json_extract(dip_accumulation_config_json, '$.drawdown1SellPct') = 20
  AND json_extract(dip_accumulation_config_json, '$.drawdown2SellPct') = 30
  AND json_extract(dip_accumulation_config_json, '$.drawdown3SellPct') = 70;
