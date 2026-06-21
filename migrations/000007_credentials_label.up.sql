ALTER TABLE external_api_credentials
    ADD COLUMN label       VARCHAR(100) NOT NULL DEFAULT '' AFTER exchange,
    ADD COLUMN api_key_hint VARCHAR(20) NOT NULL DEFAULT '' AFTER label;
