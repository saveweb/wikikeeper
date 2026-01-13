-- Migration: Optimize extensions query performance
-- This migration adds indexes to improve performance of extensions statistics queries
-- especially when dealing with large numbers of snapshots and extension items

-- Partial index for current snapshots only
-- This index only contains IDs of currently valid snapshots, making it much smaller
CREATE INDEX IF NOT EXISTS idx_wiki_extensions_current_snapshots 
ON wiki_extensions_snapshots(id) 
WHERE valid_until IS NULL;

-- Composite index for extension items (snapshot_id, name)
-- This optimizes the JOIN and GROUP BY operations for extensions queries
CREATE INDEX IF NOT EXISTS idx_wiki_extension_items_snapshot_name 
ON wiki_extension_items(snapshot_id, name);

-- Index on name for faster GROUP BY operations
-- Note: idx_wiki_extension_items_name already exists, but we ensure it exists
CREATE INDEX IF NOT EXISTS idx_wiki_extension_items_name 
ON wiki_extension_items(name);

-- Update table statistics
ANALYZE wiki_extensions_snapshots;
ANALYZE wiki_extension_items;
