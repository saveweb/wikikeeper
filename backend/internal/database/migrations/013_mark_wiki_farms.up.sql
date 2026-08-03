ALTER TABLE wikis
ADD COLUMN farm VARCHAR(50);

UPDATE wikis
SET farm = CASE
    WHEN lower(url) ~ '^https?://([^/]+\.)?(fandom\.com|wikia\.com|wikia\.org|gamepedia\.com)([/:]|$)' THEN 'fandom'
    WHEN lower(url) ~ '^https?://([^/]+\.)?miraheze\.org([/:]|$)' THEN 'miraheze'
    WHEN lower(url) ~ '^https?://([^/]+\.)?shoutwiki\.com([/:]|$)' THEN 'shoutwiki'
END
WHERE farm IS NULL;

CREATE INDEX idx_wikis_farm ON wikis(farm);

DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats;

DO $$
BEGIN
    IF (SELECT legacy_writes FROM extension_storage_state WHERE singleton) THEN
        EXECUTE $view$
            CREATE MATERIALIZED VIEW mv_extension_stats AS
            SELECT item.name,
                   COUNT(*) FILTER (WHERE wiki.farm IS NULL) AS count,
                   COUNT(*) AS all_count
            FROM wiki_extension_items item
            JOIN wiki_extensions_snapshots snapshot ON item.snapshot_id = snapshot.id
            JOIN wikis wiki ON wiki.id = snapshot.wiki_id
            WHERE snapshot.valid_until IS NULL
            GROUP BY item.name
            WITH DATA
        $view$;
    ELSE
        EXECUTE $view$
            CREATE MATERIALIZED VIEW mv_extension_stats AS
            SELECT item.name,
                   COUNT(*) FILTER (WHERE wiki.farm IS NULL) AS count,
                   COUNT(*) AS all_count
            FROM wiki_extension_set_items item
            JOIN wiki_extensions_snapshots snapshot ON snapshot.extension_set_id = item.set_id
            JOIN wikis wiki ON wiki.id = snapshot.wiki_id
            WHERE snapshot.valid_until IS NULL
            GROUP BY item.name
            WITH DATA
        $view$;
    END IF;
END $$;

CREATE UNIQUE INDEX idx_mv_extension_stats_name ON mv_extension_stats(name);
CREATE INDEX idx_mv_extension_stats_count ON mv_extension_stats(count DESC, name);
CREATE INDEX idx_mv_extension_stats_all_count ON mv_extension_stats(all_count DESC, name);

COMMENT ON MATERIALIZED VIEW mv_extension_stats IS
'Materialized view containing current extension usage counts, both excluding and including wiki farms.';
