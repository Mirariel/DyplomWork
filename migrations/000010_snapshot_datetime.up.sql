-- Migration 010 — change snapshot_date DATE → snapshot_at DATETIME
-- Дозволяє погодинні snapshot-и для 1d графіку.

ALTER TABLE portfolio_snapshots
    ADD COLUMN snapshot_at DATETIME NOT NULL DEFAULT '2000-01-01 00:00:00' AFTER futures_pnl;

-- Копіюємо існуючі дати (встановлюємо час 00:00:00)
UPDATE portfolio_snapshots
    SET snapshot_at = CONCAT(snapshot_date, ' 00:00:00');

-- Видаляємо старий унікальний ключ і колонку
ALTER TABLE portfolio_snapshots
    DROP INDEX uq_snapshot_user_date,
    DROP COLUMN snapshot_date;

-- Новий унікальний ключ по (user_id, snapshot_at) — один запис на годину
ALTER TABLE portfolio_snapshots
    ADD UNIQUE INDEX uq_snapshot_user_at (user_id, snapshot_at);
