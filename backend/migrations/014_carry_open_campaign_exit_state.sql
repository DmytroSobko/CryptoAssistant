-- Repair open campaigns that had already taken profit under Recovery Breakout
-- before the user selected Dip Accumulation. Profit protection and completed
-- drawdown milestones belong to the position, not to the UI-selected ruleset.
INSERT INTO strategy_states_v2 (
  asset, strategy_id, state, local_high, local_low, higher_low, highest_price, drawdown_pct,
  first_entry_reference_price, first_entry_reference_at, last_dip_entry_at,
  correction_high_at, last_breakout_high_at, last_breakout_higher_low_at, reentry_after,
  entry_step, profit_taken, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, position_open, updated_at
)
SELECT
  asset, 'DIP_ACCUMULATION', 'PROFIT_PROTECTION', 0, 0, 0, highest_price, drawdown_pct,
  0, NULL, NULL, NULL, NULL, NULL, NULL,
  0, 1, drawdown1_triggered, drawdown2_triggered, drawdown3_triggered, 1, CURRENT_TIMESTAMP
FROM strategy_states_v2
WHERE strategy_id = 'RECOVERY_BREAKOUT' AND profit_taken = 1 AND position_open = 1
ON CONFLICT(asset, strategy_id) DO UPDATE SET
  state = 'PROFIT_PROTECTION',
  highest_price = CASE WHEN excluded.highest_price > strategy_states_v2.highest_price THEN excluded.highest_price ELSE strategy_states_v2.highest_price END,
  drawdown_pct = CASE WHEN excluded.highest_price > strategy_states_v2.highest_price THEN excluded.drawdown_pct ELSE strategy_states_v2.drawdown_pct END,
  first_entry_reference_price = 0,
  first_entry_reference_at = NULL,
  last_dip_entry_at = NULL,
  entry_step = 0,
  profit_taken = 1,
  drawdown1_triggered = MAX(strategy_states_v2.drawdown1_triggered, excluded.drawdown1_triggered),
  drawdown2_triggered = MAX(strategy_states_v2.drawdown2_triggered, excluded.drawdown2_triggered),
  drawdown3_triggered = MAX(strategy_states_v2.drawdown3_triggered, excluded.drawdown3_triggered),
  position_open = 1,
  updated_at = CURRENT_TIMESTAMP;
