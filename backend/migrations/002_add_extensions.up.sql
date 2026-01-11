-- Create wiki_extensions_snapshots table
CREATE TABLE IF NOT EXISTS wiki_extensions_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wiki_id UUID NOT NULL REFERENCES wikis(id) ON DELETE CASCADE,

    -- Snapshot timestamps
    snapshot_at TIMESTAMP NOT NULL,
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP NULL,  -- NULL means currently valid snapshot

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Unique constraint: one wiki cannot have multiple snapshots at the same time
    CONSTRAINT unique_wiki_snapshot UNIQUE (wiki_id, snapshot_at)
);

-- Create indexes for wiki_extensions_snapshots
CREATE INDEX IF NOT EXISTS idx_wiki_extensions_wiki_id ON wiki_extensions_snapshots(wiki_id);
CREATE INDEX IF NOT EXISTS idx_wiki_extensions_snapshot_at ON wiki_extensions_snapshots(snapshot_at DESC);
CREATE INDEX IF NOT EXISTS idx_wiki_extensions_valid_from ON wiki_extensions_snapshots(valid_from DESC);
CREATE INDEX IF NOT EXISTS idx_wiki_extensions_valid_until ON wiki_extensions_snapshots(valid_until);
CREATE INDEX IF NOT EXISTS idx_wiki_extensions_wiki_time ON wiki_extensions_snapshots(wiki_id, snapshot_at DESC);

-- Create wiki_extension_items table
CREATE TABLE IF NOT EXISTS wiki_extension_items (
    id BIGSERIAL PRIMARY KEY,
    snapshot_id UUID NOT NULL REFERENCES wiki_extensions_snapshots(id) ON DELETE CASCADE,

    -- Extension basic information
    ext_type VARCHAR(50) NOT NULL,  -- 'extension' or 'skin'
    name VARCHAR(255) NOT NULL,
    url VARCHAR(2048),
    version VARCHAR(255),
    license_name VARCHAR(255),

    -- Creation time
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Unique constraint: no duplicate extensions within the same snapshot
    CONSTRAINT unique_snapshot_ext UNIQUE (snapshot_id, ext_type, name)
);

-- Create indexes for wiki_extension_items
CREATE INDEX IF NOT EXISTS idx_wiki_extension_items_snapshot_id ON wiki_extension_items(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_wiki_extension_items_type ON wiki_extension_items(ext_type);
CREATE INDEX IF NOT EXISTS idx_wiki_extension_items_name ON wiki_extension_items(name);
