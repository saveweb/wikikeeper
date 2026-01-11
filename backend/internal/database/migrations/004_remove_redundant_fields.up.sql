-- Remove redundant valid_from and created_at fields
DROP INDEX IF EXISTS idx_wiki_extensions_valid_from;
ALTER TABLE wiki_extensions_snapshots 
DROP COLUMN IF EXISTS valid_from,
DROP COLUMN IF EXISTS created_at;
