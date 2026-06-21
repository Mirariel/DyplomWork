-- ============================================================
-- Migration 006 — dca_bots table
-- DCA (Dollar Cost Averaging) бот: купує фіксовану суму USD
-- кожні N годин незалежно від ціни.
-- ============================================================

CREATE TABLE IF NOT EXISTS dca_bots (
    id              BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED     NOT NULL,
    exchange        VARCHAR(50)         NOT NULL,
    symbol          VARCHAR(20)         NOT NULL,
    category        ENUM('spot','futures') NOT NULL DEFAULT 'spot',
    amount_usd      DECIMAL(20,8)       NOT NULL,           -- сума на одну покупку (в USD)
    interval_hours  INT UNSIGNED        NOT NULL,           -- інтервал між покупками (години)
    next_buy_at     TIMESTAMP           NOT NULL,           -- час наступної покупки
    status          ENUM('running','stopped') NOT NULL DEFAULT 'stopped',
    total_invested  DECIMAL(20,8)       NOT NULL DEFAULT 0, -- загальна вкладена сума
    total_quantity  DECIMAL(20,8)       NOT NULL DEFAULT 0, -- загальна куплена кількість
    orders_placed   INT UNSIGNED        NOT NULL DEFAULT 0,
    error_message   TEXT                DEFAULT NULL,
    created_at      TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_dca_user    (user_id),
    INDEX idx_dca_running (status, next_buy_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
