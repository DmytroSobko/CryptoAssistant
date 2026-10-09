-- Live drawdown alerts use the refreshed display quote while entries and
-- high-water marks continue to use completed UTC daily candles.
UPDATE asset_strategy_settings
SET recovery_breakout_config_json = json_set(recovery_breakout_config_json, '$.intradayDrawdownAlertsEnabled', json('true')),
    dip_accumulation_config_json = json_set(dip_accumulation_config_json, '$.intradayDrawdownAlertsEnabled', json('true')),
    updated_at = CURRENT_TIMESTAMP;
