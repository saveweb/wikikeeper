-- Remove archive status tracking fields from wikis table

DROP INDEX IF EXISTS idx_wikis_archive_last_check_at;

ALTER TABLE wikis DROP COLUMN IF EXISTS archive_last_error_at;
ALTER TABLE wikis DROP COLUMN IF EXISTS archive_last_error;
ALTER TABLE wikis DROP COLUMN IF EXISTS archive_last_check_at;
