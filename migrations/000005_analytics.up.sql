-- ============================================================
-- Migration 005 — portfolio_snapshots table
-- Щоденний snapshot вартості портфеля для PnL-графіків.
-- Записується щогодини через scheduler (ON DUPLICATE KEY UPDATE
-- забезпечує один запис на дату).
-- ============================================================

CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id      BIGINT UNSIGNED  NOT NULL,
    total_value  DECIMAL(20,8)    NOT NULL DEFAULT 0,  -- загальна вартість портфеля (USD)
    spot_value   DECIMAL(20,8)    NOT NULL DEFAULT 0,  -- вартість спот-активів
    futures_pnl  DECIMAL(20,8)    NOT NULL DEFAULT 0,  -- нереалізований PnL відкритих позицій
    snapshot_date DATE            NOT NULL,             -- дата snapshot (без часу)
    created_at   TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_snapshot_user_date (user_id, snapshot_date),
    INDEX idx_snapshots_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
