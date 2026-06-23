-- Migration 011 — remove unique constraint on portfolio_snapshots(user_id, snapshot_at)
-- Тепер кожен sync завжди робить INSERT нового рядка з точним timestamp,
-- замість UPSERT по годині. Це дозволяє будувати реальний графік з динамікою.

ALTER TABLE portfolio_snapshots
    DROP INDEX uq_snapshot_user_at;
