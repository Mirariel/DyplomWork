-- ============================================================
-- TradeTracker — initial schema
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    id         INT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    username   VARCHAR(64)      NOT NULL,
    email      VARCHAR(255)     NOT NULL,
    password   VARCHAR(255)     NOT NULL,
    created_at TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_users_email    (email),
    UNIQUE KEY uq_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS assets (
    id               INT UNSIGNED     NOT NULL AUTO_INCREMENT PRIMARY KEY,
    symbol           VARCHAR(20)      NOT NULL,
    name             VARCHAR(100)     NOT NULL,
    type             ENUM('crypto','commodity','stock') NOT NULL DEFAULT 'crypto',
    current_price    DECIMAL(20,8)    NOT NULL DEFAULT 0,
    price_updated_at TIMESTAMP        NULL DEFAULT NULL,
    created_at       TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_assets_symbol (symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Топ монети і commodities — заповнюються одразу
INSERT IGNORE INTO assets (symbol, name, type) VALUES
('BTC',   'Bitcoin',        'crypto'),
('ETH',   'Ethereum',       'crypto'),
('BNB',   'Binance Coin',   'crypto'),
('SOL',   'Solana',         'crypto'),
('XRP',   'Ripple',         'crypto'),
('ADA',   'Cardano',        'crypto'),
('AVAX',  'Avalanche',      'crypto'),
('DOGE',  'Dogecoin',       'crypto'),
('DOT',   'Polkadot',       'crypto'),
('MATIC', 'Polygon',        'crypto'),
('LINK',  'Chainlink',      'crypto'),
('UNI',   'Uniswap',        'crypto'),
('SUI',   'Sui',            'crypto'),
('TIA',   'Celestia',       'crypto'),
('LTC',   'Litecoin',       'crypto'),
('BCH',   'Bitcoin Cash',   'crypto'),
('APT',   'Aptos',          'crypto'),
('ARB',   'Arbitrum',       'crypto'),
('OP',    'Optimism',       'crypto'),
('PEPE',  'Pepe',           'crypto'),
('TON',   'Toncoin',        'crypto'),
('WIF',   'dogwifhat',      'crypto'),
('JUP',   'Jupiter',        'crypto'),
('NEAR',  'NEAR Protocol',  'crypto'),
('FIL',   'Filecoin',       'crypto'),
('ATOM',  'Cosmos',         'crypto'),
('INJ',   'Injective',      'crypto'),
('USDT',  'Tether',         'crypto'),
('USDC',  'USD Coin',       'crypto'),
('XAU',   'Gold',           'commodity'),
('XAG',   'Silver',         'commodity'),
('WTI',   'Crude Oil WTI',  'commodity'),
('BRENT', 'Brent Oil',      'commodity'),
('NG',    'Natural Gas',    'commodity');

-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS external_api_credentials (
    id                   INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id              INT UNSIGNED NOT NULL,
    exchange             VARCHAR(50)  NOT NULL,
    api_key_encrypted    TEXT         NOT NULL,
    api_secret_encrypted TEXT         NOT NULL,
    passphrase_encrypted TEXT         NULL,
    is_active            TINYINT(1)   NOT NULL DEFAULT 1,
    last_sync_at         TIMESTAMP    NULL DEFAULT NULL,
    last_sync_error      TEXT         NULL DEFAULT NULL,
    created_at           TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_creds_user_exchange (user_id, exchange),
    CONSTRAINT fk_creds_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS user_portfolios (
    id            INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id       INT UNSIGNED  NOT NULL,
    asset_id      INT UNSIGNED  NOT NULL,
    exchange      VARCHAR(50)   NOT NULL,
    quantity      DECIMAL(20,8) NOT NULL DEFAULT 0,
    avg_buy_price DECIMAL(20,8) NOT NULL DEFAULT 0,
    manually_set  TINYINT(1)    NOT NULL DEFAULT 0,
    updated_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_portfolio_user_asset_exchange (user_id, asset_id, exchange),
    CONSTRAINT fk_portfolio_user  FOREIGN KEY (user_id)  REFERENCES users(id)  ON DELETE CASCADE,
    CONSTRAINT fk_portfolio_asset FOREIGN KEY (asset_id) REFERENCES assets(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS open_positions (
    id          INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     INT UNSIGNED  NOT NULL,
    asset_id    INT UNSIGNED  NOT NULL,
    symbol      VARCHAR(30)   NOT NULL,
    exchange    VARCHAR(50)   NOT NULL,
    side        ENUM('LONG','SHORT') NOT NULL,
    margin_mode VARCHAR(20)   NOT NULL DEFAULT 'cross',
    entry_price DECIMAL(20,8) NOT NULL DEFAULT 0,
    mark_price  DECIMAL(20,8) NOT NULL DEFAULT 0,
    quantity    DECIMAL(20,8) NOT NULL DEFAULT 0,
    pnl         DECIMAL(20,8) NOT NULL DEFAULT 0,
    leverage    VARCHAR(10)   NOT NULL DEFAULT '1x',
    liq_price   DECIMAL(20,8) NOT NULL DEFAULT 0,
    margin      DECIMAL(20,8) NOT NULL DEFAULT 0,
    trade_size  DECIMAL(20,8) NOT NULL DEFAULT 0,
    comment     TEXT          NULL DEFAULT NULL,
    updated_at  TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_position (user_id, symbol, side, exchange),
    CONSTRAINT fk_position_user  FOREIGN KEY (user_id)  REFERENCES users(id)  ON DELETE CASCADE,
    CONSTRAINT fk_position_asset FOREIGN KEY (asset_id) REFERENCES assets(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS position_history (
    id           INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id      INT UNSIGNED  NOT NULL,
    symbol       VARCHAR(30)   NOT NULL,
    side         ENUM('LONG','SHORT') NOT NULL,
    margin_mode  VARCHAR(20)   NOT NULL DEFAULT 'isolated',
    leverage     VARCHAR(10)   NOT NULL DEFAULT '1x',
    entry_price  DECIMAL(20,8) NOT NULL DEFAULT 0,
    exit_price   DECIMAL(20,8) NOT NULL DEFAULT 0,
    quantity     DECIMAL(20,8) NOT NULL DEFAULT 0,
    realized_pnl DECIMAL(20,8) NOT NULL DEFAULT 0,
    fee          DECIMAL(20,8) NOT NULL DEFAULT 0,
    max_size     DECIMAL(20,8) NOT NULL DEFAULT 0,
    opened_at    DATETIME      NULL,
    closed_at    DATETIME      NULL,
    exchange     VARCHAR(50)   NOT NULL,
    comment      TEXT          NULL DEFAULT NULL,
    created_at   TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_pos_hist (user_id, symbol, closed_at, realized_pnl),
    CONSTRAINT fk_history_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS transactions (
    id         INT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id    INT UNSIGNED  NOT NULL,
    asset_id   INT UNSIGNED  NOT NULL,
    type       ENUM('buy','sell','deposit','withdrawal','sync_import') NOT NULL,
    quantity   DECIMAL(20,8) NOT NULL,
    price      DECIMAL(20,8) NOT NULL,
    fee        DECIMAL(20,8) NOT NULL DEFAULT 0,
    note       VARCHAR(255)  NULL,
    created_at TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_tx_user  FOREIGN KEY (user_id)  REFERENCES users(id)  ON DELETE CASCADE,
    CONSTRAINT fk_tx_asset FOREIGN KEY (asset_id) REFERENCES assets(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
