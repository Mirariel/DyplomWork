CREATE TABLE IF NOT EXISTS futures_positions (
    id              INT UNSIGNED   NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id         INT UNSIGNED   NOT NULL,
    exchange        VARCHAR(50)    NOT NULL,
    symbol          VARCHAR(30)    NOT NULL,
    side            ENUM('LONG','SHORT') NOT NULL,
    size            DECIMAL(20,8)  NOT NULL DEFAULT 0,
    entry_price     DECIMAL(20,8)  NOT NULL DEFAULT 0,
    mark_price      DECIMAL(20,8)  NOT NULL DEFAULT 0,
    unrealized_pnl  DECIMAL(20,8)  NOT NULL DEFAULT 0,
    leverage        INT UNSIGNED   NOT NULL DEFAULT 1,
    margin_type     VARCHAR(20)    NOT NULL DEFAULT 'cross',
    synced_at       TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_futures_pos (user_id, exchange, symbol, side),
    CONSTRAINT fk_futures_pos_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
