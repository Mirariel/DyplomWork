-- Revert: restore '0x' back to '1x' for OKX history entries
UPDATE position_history
SET leverage = '1x'
WHERE leverage = '0x'
  AND exchange = 'okx';
