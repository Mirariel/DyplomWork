DROP INDEX idx_snapshots_user_exchange ON portfolio_snapshots;
ALTER TABLE portfolio_snapshots DROP COLUMN exchange;
