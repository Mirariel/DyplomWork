-- ============================================================
-- Migration 004 — bots + bot_grids tables
-- Grid trading bot: автоматичне розміщення лімітних ордерів
-- на рівнях цінової сітки.
-- ============================================================

CREATE TABLE IF NOT EXISTS bots (
    id              BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED     NOT NULL,
    exchange        VARCHAR(50)         NOT NULL,
    symbol          VARCHAR(20)         NOT NULL,
    category        ENUM('spot','futures') NOT NULL DEFAULT 'spot',
    price_low       DECIMAL(20,8)       NOT NULL,   -- нижня межа сітки
    price_high      DECIMAL(20,8)       NOT NULL,   -- верхня межа сітки
    grids           INT UNSIGNED        NOT NULL,   -- кількість інтервалів (мінімум 2)
    qty_per_grid    DECIMAL(20,8)       NOT NULL,   -- кількість активу на кожен ордер
    status          ENUM('running','stopped','error') NOT NULL DEFAULT 'stopped',
    total_profit    DECIMAL(20,8)       NOT NULL DEFAULT 0,
    error_message   TEXT                DEFAULT NULL,
    created_at      TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_bots_user    (user_id),
    INDEX idx_bots_running (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS bot_grids (
    id                  BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    bot_id              BIGINT UNSIGNED     NOT NULL,
    level               INT UNSIGNED        NOT NULL,    -- 0=найнижчий рівень
    price               DECIMAL(20,8)       NOT NULL,
    side                ENUM('buy','sell')  NOT NULL,
    exchange_order_id   VARCHAR(128)        DEFAULT NULL,
    status              ENUM('pending','open','filled','cancelled') NOT NULL DEFAULT 'pending',
    filled_at           TIMESTAMP           NULL DEFAULT NULL,
    created_at          TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_bot_grids_bot  (bot_id),
    INDEX idx_bot_grids_open (bot_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
