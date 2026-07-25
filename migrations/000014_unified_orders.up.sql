-- ============================================================
-- Migration 014 — Unified orders: credential groups, credential_id, leverage
-- ============================================================

-- 1. Credential groups — групи API ключів
CREATE TABLE IF NOT EXISTS credential_groups (
    id            BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id       INT UNSIGNED     NOT NULL,
    name          VARCHAR(100)     NOT NULL,
    created_at    TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_group_name (user_id, name),
    CONSTRAINT fk_cred_group_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Зв'язок група ↔ credential
CREATE TABLE IF NOT EXISTS credential_group_members (
    id            BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
    group_id      BIGINT UNSIGNED  NOT NULL,
    credential_id INT UNSIGNED     NOT NULL,
    UNIQUE KEY uq_group_cred (group_id, credential_id),
    CONSTRAINT fk_cgm_group FOREIGN KEY (group_id)      REFERENCES credential_groups(id)         ON DELETE CASCADE,
    CONSTRAINT fk_cgm_cred  FOREIGN KEY (credential_id) REFERENCES external_api_credentials(id)  ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Дозволяємо кілька API ключів на одну біржу
--    Спочатку додаємо простий індекс на user_id (FK потребує індекс),
--    потім прибираємо unique constraint.
ALTER TABLE external_api_credentials ADD INDEX idx_creds_user_id (user_id);
ALTER TABLE external_api_credentials DROP INDEX uq_creds_user_exchange;

-- 4. Додаємо credential_id + leverage до orders
ALTER TABLE orders
    ADD COLUMN credential_id INT UNSIGNED NULL AFTER user_id,
    ADD COLUMN leverage      VARCHAR(10) NOT NULL DEFAULT '' AFTER category;

-- 5. Додаємо credential_id + leverage до smart_orders
ALTER TABLE smart_orders
    ADD COLUMN credential_id INT UNSIGNED NULL AFTER user_id,
    ADD COLUMN leverage      VARCHAR(10) NOT NULL DEFAULT '' AFTER category;
