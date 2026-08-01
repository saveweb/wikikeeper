DROP TABLE IF EXISTS extension_storage_state;
DROP INDEX IF EXISTS idx_wiki_extensions_current_set;
ALTER TABLE wiki_extensions_snapshots DROP COLUMN IF EXISTS extension_set_id;
DROP TABLE IF EXISTS wiki_extension_set_items;
DROP TABLE IF EXISTS wiki_extension_sets;
