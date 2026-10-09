UPDATE asset_strategy_settings
SET ath_entry_override_json = json_set(ath_entry_override_json,
  '$.sourceFile', '.backtestdata/btc-usd-daily-10y-2016-09-30-to-2026-10-07.csv',
  '$.peakCount', 3)
WHERE asset = 'BTC';

UPDATE asset_strategy_settings
SET ath_entry_override_json = json_set(ath_entry_override_json,
  '$.sourceFile', '.backtestdata/eth-usd-daily-10y-2016-09-30-to-2026-10-07.csv',
  '$.peakCount', 3)
WHERE asset = 'ETH';
