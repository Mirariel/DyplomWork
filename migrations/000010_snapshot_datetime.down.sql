ALTER TABLE portfolio_snapshots
    ADD COLUMN snapshot_date DATE NULL AFTER futures_pnl;

UPDATE portfolio_snapshots
    SET snapshot_date = DATE(snapshot_at);

ALTER TABLE portfolio_snapshots
    DROP INDEX uq_snapshot_user_at,
    DROP COLUMN snapshot_at;

ALTER TABLE portfolio_snapshots
    MODIFY COLUMN snapshot_date DATE NOT NULL DEFAULT '2000-01-01',
    ADD UNIQUE INDEX uq_snapshot_user_date (user_id, snapshot_date);
