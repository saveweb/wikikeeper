-- Create materialized view for extension statistics
-- This provides fast access to extension usage counts without real-time aggregation

DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats CASCADE;

CREATE MATERIALIZED VIEW mv_extension_stats AS
SELECT 
    wei.name,
    COUNT(*) as count
FROM wiki_extension_items wei
JOIN wiki_extensions_snapshots wes ON wei.snapshot_id = wes.id
WHERE wes.valid_until IS NULL
GROUP BY wei.name
ORDER BY count DESC, wei.name ASC
WITH DATA;

-- Create index on the materialized view for faster lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_extension_stats_name 
ON mv_extension_stats(name);

-- Create index for sorting by count
CREATE INDEX IF NOT EXISTS idx_mv_extension_stats_count 
ON mv_extension_stats(count DESC, name);

-- Add comment
COMMENT ON MATERIALIZED VIEW mv_extension_stats IS 
'Materialized view containing extension usage statistics for current snapshots only. 
Refresh this view when extension snapshots are updated.';

-- Grant permissions (adjust as needed)
-- GRANT SELECT ON mv_extension_stats TO wikikeeper;
