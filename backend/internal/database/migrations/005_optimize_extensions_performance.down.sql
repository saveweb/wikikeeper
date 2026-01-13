-- Rollback performance optimization indexes

DROP INDEX IF EXISTS idx_wiki_extensions_current_snapets;
DROP INDEX IF EXISTS idx_wiki_extension_items_snapshot_name;

-- Keep idx_wiki_extension_items_name as it was created in migration 002
