UPDATE asset_strategy_settings
SET ath_entry_override_json = json_set(ath_entry_override_json, '$.peakSpacingMonths', 3)
WHERE json_extract(ath_entry_override_json, '$.peakSpacingMonths') IS NULL;
