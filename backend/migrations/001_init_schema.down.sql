-- Drop triggers
DROP TRIGGER IF EXISTS update_wiki_archives_updated_at ON wiki_archives;
DROP TRIGGER IF EXISTS update_wikis_updated_at ON wikis;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (in correct order due to foreign keys)
DROP INDEX IF EXISTS idx_wiki_archives_dump_date;
DROP INDEX IF EXISTS idx_wiki_archives_ia_identifier;
DROP INDEX IF EXISTS idx_wiki_archives_wiki_id;
DROP TABLE IF EXISTS wiki_archives;

DROP INDEX IF EXISTS idx_wiki_stats_wiki_time;
DROP INDEX IF EXISTS idx_wiki_stats_time;
DROP INDEX IF EXISTS idx_wiki_stats_wiki_id;
DROP TABLE IF EXISTS wiki_stats;

DROP INDEX IF EXISTS idx_wikis_sitename;
DROP INDEX IF EXISTS idx_wikis_last_check_at;
DROP INDEX IF EXISTS idx_wikis_updated_at;
DROP INDEX IF EXISTS idx_wikis_created_at;
DROP INDEX IF EXISTS idx_wikis_api_url;
DROP INDEX IF EXISTS idx_wikis_has_archive;
DROP INDEX IF EXISTS idx_wikis_status;
DROP INDEX IF EXISTS idx_wikis_url;
DROP TABLE IF EXISTS wikis;
