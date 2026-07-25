ALTER TABLE smart_orders DROP COLUMN leverage, DROP COLUMN credential_id;
ALTER TABLE orders      DROP COLUMN leverage, DROP COLUMN credential_id;

ALTER TABLE external_api_credentials
    ADD UNIQUE KEY uq_creds_user_exchange (user_id, exchange);
ALTER TABLE external_api_credentials DROP INDEX idx_creds_user_id;

DROP TABLE IF EXISTS credential_group_members;
DROP TABLE IF EXISTS credential_groups;
