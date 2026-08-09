ALTER TABLE wikis DROP CONSTRAINT IF EXISTS wikis_status_check;
ALTER TABLE wikis
ADD CONSTRAINT wikis_status_check
CHECK (status IN ('pending', 'ok', 'error', 'offline'));
