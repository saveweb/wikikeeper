-- Drop indexes first
DROP INDEX IF EXISTS idx_wiki_extension_items_name;
DROP INDEX IF EXISTS idx_wiki_extension_items_type;
DROP INDEX IF EXISTS idx_wiki_extension_items_snapshot_id;
DROP INDEX IF EXISTS idx_wiki_extensions_wiki_time;
DROP INDEX IF EXISTS idx_wiki_extensions_valid_until;
DROP INDEX IF EXISTS idx_wiki_extensions_valid_from;
DROP INDEX IF EXISTS idx_wiki_extensions_snapshot_at;
DROP INDEX IF EXISTS idx_wiki_extensions_wiki_id;

-- Drop tables (order matters due to foreign key constraints)
DROP TABLE IF EXISTS wiki_extension_items;
DROP TABLE IF EXISTS wiki_extensions_snapshots;
