DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats_next;
DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats;

ALTER TABLE schema_migrations
    ALTER COLUMN applied_at TYPE TIMESTAMP
    USING applied_at AT TIME ZONE 'UTC';

ALTER TABLE wikis
    ALTER COLUMN last_error_at TYPE TIMESTAMP USING last_error_at AT TIME ZONE 'UTC',
    ALTER COLUMN last_success_at TYPE TIMESTAMP USING last_success_at AT TIME ZONE 'UTC',
    ALTER COLUMN next_check_at TYPE TIMESTAMP USING next_check_at AT TIME ZONE 'UTC',
    ALTER COLUMN archive_last_check_at TYPE TIMESTAMP USING archive_last_check_at AT TIME ZONE 'UTC',
    ALTER COLUMN archive_last_error_at TYPE TIMESTAMP USING archive_last_error_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN last_check_at TYPE TIMESTAMP USING last_check_at AT TIME ZONE 'UTC';

ALTER TABLE wiki_stats
    ALTER COLUMN time TYPE TIMESTAMP USING time AT TIME ZONE 'UTC';

ALTER TABLE wiki_archives
    ALTER COLUMN added_date TYPE TIMESTAMP USING added_date AT TIME ZONE 'UTC',
    ALTER COLUMN dump_date TYPE TIMESTAMP USING dump_date AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE wiki_extensions_snapshots
    ALTER COLUMN snapshot_at TYPE TIMESTAMP USING snapshot_at AT TIME ZONE 'UTC',
    ALTER COLUMN valid_until TYPE TIMESTAMP USING valid_until AT TIME ZONE 'UTC';

ALTER TABLE wiki_extension_items
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE provider_rate_limits
    ALTER COLUMN retry_at TYPE TIMESTAMP USING retry_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE wiki_extension_sets
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE extension_storage_state
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

DO $$
BEGIN
    IF (SELECT legacy_writes FROM extension_storage_state WHERE singleton) THEN
        EXECUTE $view$
            CREATE MATERIALIZED VIEW mv_extension_stats AS
            SELECT wei.name, COUNT(*) AS count
            FROM wiki_extension_items wei
            JOIN wiki_extensions_snapshots wes ON wei.snapshot_id = wes.id
            WHERE wes.valid_until IS NULL
            GROUP BY wei.name
            ORDER BY count DESC, wei.name ASC
            WITH DATA
        $view$;
    ELSE
        EXECUTE $view$
            CREATE MATERIALIZED VIEW mv_extension_stats AS
            SELECT item.name, COUNT(*) AS count
            FROM wiki_extension_set_items item
            JOIN wiki_extensions_snapshots snapshot
              ON snapshot.extension_set_id = item.set_id
            WHERE snapshot.valid_until IS NULL
            GROUP BY item.name
            ORDER BY count DESC, item.name ASC
            WITH DATA
        $view$;
    END IF;
END
$$;

CREATE UNIQUE INDEX idx_mv_extension_stats_name
ON mv_extension_stats(name);

CREATE INDEX idx_mv_extension_stats_count
ON mv_extension_stats(count DESC, name);

COMMENT ON MATERIALIZED VIEW mv_extension_stats IS
'Materialized view containing extension usage statistics for current snapshots only. Refresh this view when extension snapshots are updated.';
