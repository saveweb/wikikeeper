DROP INDEX IF EXISTS idx_wikis_next_check_at;
DROP INDEX IF EXISTS idx_wikis_last_success_at;
DROP INDEX IF EXISTS idx_wikis_collection_status;

ALTER TABLE wikis ALTER COLUMN api_available SET DEFAULT true;

ALTER TABLE wikis
DROP COLUMN IF EXISTS consecutive_failures,
DROP COLUMN IF EXISTS next_check_at,
DROP COLUMN IF EXISTS last_success_at,
DROP COLUMN IF EXISTS collection_status;
