DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats;

DO $$
BEGIN
    IF (SELECT legacy_writes FROM extension_storage_state WHERE singleton) THEN
        EXECUTE $view$
            CREATE MATERIALIZED VIEW mv_extension_stats AS
            SELECT item.name, COUNT(*) AS count
            FROM wiki_extension_items item
            JOIN wiki_extensions_snapshots snapshot ON item.snapshot_id = snapshot.id
            WHERE snapshot.valid_until IS NULL
            GROUP BY item.name
            WITH DATA
        $view$;
    ELSE
        EXECUTE $view$
            CREATE MATERIALIZED VIEW mv_extension_stats AS
            SELECT item.name, COUNT(*) AS count
            FROM wiki_extension_set_items item
            JOIN wiki_extensions_snapshots snapshot ON snapshot.extension_set_id = item.set_id
            WHERE snapshot.valid_until IS NULL
            GROUP BY item.name
            WITH DATA
        $view$;
    END IF;
END $$;

CREATE UNIQUE INDEX idx_mv_extension_stats_name ON mv_extension_stats(name);
CREATE INDEX idx_mv_extension_stats_count ON mv_extension_stats(count DESC, name);

DROP INDEX IF EXISTS idx_wikis_farm;
ALTER TABLE wikis DROP COLUMN IF EXISTS farm;
