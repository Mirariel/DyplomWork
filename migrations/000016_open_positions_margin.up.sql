-- 000016: Ensure open_positions.margin stores real exchange margin (not just notional/leverage).
-- Column already exists in DDL (000001), this migration is a no-op placeholder
-- to mark the point where backend started writing real margin from exchange APIs.
-- No schema change needed — column `margin DECIMAL(20,8)` already exists.
SELECT 1;
