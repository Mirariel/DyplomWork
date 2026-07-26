ALTER TABLE position_history
    ADD COLUMN margin DECIMAL(20,8) NOT NULL DEFAULT 0 AFTER max_size;
