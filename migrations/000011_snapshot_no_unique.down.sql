-- Відновити unique index (увага: може провалитись якщо є дублікати)
ALTER TABLE portfolio_snapshots
    ADD UNIQUE INDEX uq_snapshot_user_at (user_id, snapshot_at);
