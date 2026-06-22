CREATE TABLE IF NOT EXISTS spot_trades (
    id          INT UNSIGNED   NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     INT UNSIGNED   NOT NULL,
    exchange    VARCHAR(50)    NOT NULL,
    symbol      VARCHAR(30)    NOT NULL,
    side        ENUM('buy','sell') NOT NULL,
    quantity    DECIMAL(20,8)  NOT NULL DEFAULT 0,
    price       DECIMAL(20,8)  NOT NULL DEFAULT 0,
    fee         DECIMAL(20,8)  NOT NULL DEFAULT 0,
    fee_asset   VARCHAR(20)    NOT NULL DEFAULT '',
    traded_at   BIGINT         NOT NULL DEFAULT 0,
    UNIQUE KEY uq_spot_trade (user_id, exchange, symbol, side, traded_at, quantity),
    CONSTRAINT fk_spot_trades_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
