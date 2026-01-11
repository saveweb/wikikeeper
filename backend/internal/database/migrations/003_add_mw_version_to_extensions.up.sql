-- Add mediawiki_version column to wiki_extensions_snapshots
ALTER TABLE wiki_extensions_snapshots 
ADD COLUMN mediawiki_version VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_wiki_extensions_mw_version 
ON wiki_extensions_snapshots(mediawiki_version);
