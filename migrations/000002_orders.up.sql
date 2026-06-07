-- ============================================================
-- Migration 002 — orders table
-- Зберігає всі ордери розміщені через API сервера.
-- ============================================================

CREATE TABLE IF NOT EXISTS orders (
    id                  BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id             BIGINT UNSIGNED     NOT NULL,
    exchange            VARCHAR(50)         NOT NULL,                 -- "binance", "okx", "bybit"
    exchange_order_id   VARCHAR(128)        DEFAULT NULL,             -- ID на стороні біржі
    symbol              VARCHAR(20)         NOT NULL,                 -- "BTC", "ETH"
    side                ENUM('buy','sell')  NOT NULL,
    type                ENUM('market','limit') NOT NULL,
    category            ENUM('spot','futures') NOT NULL DEFAULT 'spot',
    quantity            DECIMAL(20,8)       NOT NULL,
    price               DECIMAL(20,8)       NOT NULL DEFAULT 0,       -- 0 для market ордерів
    filled_qty          DECIMAL(20,8)       NOT NULL DEFAULT 0,
    avg_price           DECIMAL(20,8)       NOT NULL DEFAULT 0,
    status              VARCHAR(20)         NOT NULL DEFAULT 'new',   -- new|partial|filled|cancelled|rejected
    error_message       TEXT                DEFAULT NULL,             -- заповнюється якщо розміщення провалилось
    created_at          TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_orders_user_id           (user_id),
    INDEX idx_orders_user_status       (user_id, status),
    INDEX idx_orders_exchange_order_id (exchange_order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
