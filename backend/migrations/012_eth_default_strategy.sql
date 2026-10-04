-- Match the normal strategy-switch behavior: start a fresh advisory cycle
-- only when ETH is actually switching to Dip Accumulation.
DELETE FROM strategy_states_v2
WHERE asset = 'ETH' AND strategy_id = 'DIP_ACCUMULATION'
  AND EXISTS (
    SELECT 1 FROM asset_strategy_settings
    WHERE asset = 'ETH' AND selected_strategy_id <> 'DIP_ACCUMULATION'
  );

UPDATE asset_strategy_settings
SET selected_strategy_id = 'DIP_ACCUMULATION', updated_at = CURRENT_TIMESTAMP
WHERE asset = 'ETH' AND selected_strategy_id <> 'DIP_ACCUMULATION';
