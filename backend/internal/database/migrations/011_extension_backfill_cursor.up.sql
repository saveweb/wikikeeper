ALTER TABLE extension_storage_state
ADD COLUMN backfill_cursor BIGINT NOT NULL DEFAULT 0;
