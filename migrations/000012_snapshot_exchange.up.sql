-- Migration 012 — add exchange column to portfolio_snapshots
-- Кожен рядок тепер відповідає одній біржі одного користувача в певний момент.
-- "All Exchanges" агрегується на рівні запиту (SUM + bucket by 15 min).

ALTER TABLE portfolio_snapshots
    ADD COLUMN exchange VARCHAR(50) NOT NULL DEFAULT '' AFTER user_id;

-- Оновлюємо існуючі рядки (агреговані — позначаємо як 'all')
UPDATE portfolio_snapshots SET exchange = 'all' WHERE exchange = '';

-- Індекс для швидкого вибору по (user_id, exchange, snapshot_at)
CREATE INDEX idx_snapshots_user_exchange ON portfolio_snapshots (user_id, exchange, snapshot_at);
