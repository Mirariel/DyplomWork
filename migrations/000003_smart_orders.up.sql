-- ============================================================
-- Migration 003 — smart_orders table
-- Stop-Loss, Take-Profit, Trailing Stop orders.
-- Перевіряються фоновим сервісом кожні 5 секунд.
-- ============================================================

CREATE TABLE IF NOT EXISTS smart_orders (
    id                  BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id             BIGINT UNSIGNED     NOT NULL,
    exchange            VARCHAR(50)         NOT NULL,
    symbol              VARCHAR(20)         NOT NULL,
    side                ENUM('buy','sell')  NOT NULL,
    category            ENUM('spot','futures') NOT NULL DEFAULT 'spot',
    quantity            DECIMAL(20,8)       NOT NULL,

    -- Тип умовного ордеру
    type                ENUM('stop_loss','take_profit','trailing_stop') NOT NULL,

    -- stop_loss / take_profit: ціна активації
    trigger_price       DECIMAL(20,8)       NOT NULL DEFAULT 0,

    -- trailing_stop: відступ у відсотках від піку (наприклад 1.5 = 1.5%)
    callback_rate       DECIMAL(10,4)       NOT NULL DEFAULT 0,

    -- trailing_stop: найвища (sell) або найнижча (buy) відома ціна
    peak_price          DECIMAL(20,8)       NOT NULL DEFAULT 0,

    -- Стан
    status              ENUM('active','triggered','cancelled','failed') NOT NULL DEFAULT 'active',
    triggered_order_id  BIGINT UNSIGNED     DEFAULT NULL,   -- FK → orders.id
    error_message       TEXT                DEFAULT NULL,

    created_at          TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_smart_orders_user   (user_id),
    INDEX idx_smart_orders_active (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
