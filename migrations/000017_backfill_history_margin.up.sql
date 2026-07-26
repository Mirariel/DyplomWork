-- 000017: Backfill margin for existing position_history rows where margin=0.
-- margin = max_size / leverage (allocated margin under the position).
-- Skips rows with unknown leverage ("0x") to avoid division by zero.
UPDATE position_history
   SET margin = max_size / CAST(REPLACE(leverage, 'x', '') AS DECIMAL(10,2))
 WHERE margin = 0
   AND leverage != '0x';
