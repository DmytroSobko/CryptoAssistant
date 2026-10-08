-- Apply the BTC default minimum discount from the historical peak before a
-- normal recovery Entry 1 may be signalled. The ATH override remains separate.
UPDATE asset_strategy_settings
SET recovery_breakout_config_json = json_set(
      recovery_breakout_config_json,
      '$.recoveryEntryMinPeakDiscountPct', 10),
    dip_accumulation_config_json = json_set(
      dip_accumulation_config_json,
      '$.recoveryEntryMinPeakDiscountPct', 10),
    updated_at = CURRENT_TIMESTAMP
WHERE asset = 'BTC';
