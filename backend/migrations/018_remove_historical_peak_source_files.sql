UPDATE asset_strategy_settings
SET ath_entry_override_json = json_remove(ath_entry_override_json, '$.sourceFile')
WHERE json_type(ath_entry_override_json, '$.sourceFile') IS NOT NULL;
