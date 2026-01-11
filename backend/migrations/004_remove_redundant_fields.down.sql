-- Add back valid_from and created_at fields
ALTER TABLE wiki_extensions_snapshots 
ADD COLUMN IF NOT EXISTS valid_from TIMESTAMP NOT NULL DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_wiki_extensions_valid_from 
ON wiki_extensions_snapshots(valid_from DESC);

-- Update valid_from to match snapshot_at for existing records
UPDATE wiki_extensions_snapshots 
SET valid_from = snapshot_at 
WHERE valid_from IS NULL;
