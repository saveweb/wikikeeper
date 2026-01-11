-- Remove mediawiki_version column from wiki_extensions_snapshots
DROP INDEX IF EXISTS idx_wiki_extensions_mw_version;
ALTER TABLE wiki_extensions_snapshots 
DROP COLUMN IF EXISTS mediawiki_version;
