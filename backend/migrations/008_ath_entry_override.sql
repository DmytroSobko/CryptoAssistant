ALTER TABLE asset_strategy_settings ADD COLUMN ath_entry_override_json TEXT NOT NULL DEFAULT '{"enabled":false,"thresholdPct":60}';
