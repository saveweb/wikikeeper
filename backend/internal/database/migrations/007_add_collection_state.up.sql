ALTER TABLE wikis
ADD COLUMN collection_status VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (collection_status IN ('pending', 'ok', 'rate_limited', 'error')),
ADD COLUMN last_success_at TIMESTAMP,
ADD COLUMN next_check_at TIMESTAMP,
ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0;

ALTER TABLE wikis ALTER COLUMN api_available SET DEFAULT false;

UPDATE wikis w
SET last_success_at = latest.last_success_at
FROM (
    SELECT wiki_id, MAX(time) AS last_success_at
    FROM wiki_stats
    GROUP BY wiki_id
) latest
WHERE latest.wiki_id = w.id;

UPDATE wikis
SET collection_status = CASE
        WHEN last_error ILIKE '%HTTP 429%' THEN 'rate_limited'
        WHEN last_error IS NOT NULL THEN 'error'
        WHEN last_check_at IS NOT NULL THEN 'ok'
        ELSE 'pending'
    END,
    consecutive_failures = CASE WHEN last_error IS NULL THEN 0 ELSE 1 END,
    next_check_at = CASE
        WHEN last_check_at IS NULL THEN NOW()
        ELSE last_check_at + INTERVAL '3 days'
    END;

-- A rate limit is a collector outcome, not evidence that the wiki or API is down.
UPDATE wikis
SET status = CASE WHEN last_success_at IS NULL THEN 'pending' ELSE 'ok' END,
    api_available = last_success_at IS NOT NULL
WHERE collection_status = 'rate_limited';

UPDATE wikis
SET api_available = false
WHERE last_success_at IS NULL AND status = 'pending';

CREATE INDEX idx_wikis_collection_status ON wikis(collection_status);
CREATE INDEX idx_wikis_last_success_at ON wikis(last_success_at);
CREATE INDEX idx_wikis_next_check_at ON wikis(next_check_at);
