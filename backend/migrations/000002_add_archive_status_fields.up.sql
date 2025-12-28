-- Add archive status tracking fields to wikis table

-- Archive last check timestamp
ALTER TABLE wikis ADD COLUMN archive_last_check_at TIMESTAMP;

-- Archive last error
ALTER TABLE wikis ADD COLUMN archive_last_error TEXT;

-- Archive last error timestamp
ALTER TABLE wikis ADD COLUMN archive_last_error_at TIMESTAMP;

-- Create index for archive_last_check_at
CREATE INDEX idx_wikis_archive_last_check_at ON wikis(archive_last_check_at);

-- Add comments
COMMENT ON COLUMN wikis.archive_last_check_at IS 'Timestamp of last archive.org check';
COMMENT ON COLUMN wikis.archive_last_error IS 'Last error message from archive.org check';
COMMENT ON COLUMN wikis.archive_last_error_at IS 'Timestamp of last archive.org error';
