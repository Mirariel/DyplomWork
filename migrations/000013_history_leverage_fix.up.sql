-- Fix leverage values that were force-set to '1x' due to OKX returning empty lever field.
-- '0x' is now the sentinel for "leverage unknown" — the UI will show "—" for these.
-- Trades from OKX with lever = '1x' are most likely 0/unknown, not actual 1x futures.
UPDATE position_history
SET leverage = '0x'
WHERE leverage = '1x'
  AND exchange = 'okx';
