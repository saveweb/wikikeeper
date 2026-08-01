CREATE TABLE wiki_extension_sets (
    id BIGSERIAL PRIMARY KEY,
    content_hash BYTEA NOT NULL UNIQUE,
    item_count INTEGER NOT NULL CHECK (item_count >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT wiki_extension_sets_hash_length CHECK (octet_length(content_hash) = 32)
);

CREATE TABLE wiki_extension_set_items (
    set_id BIGINT NOT NULL REFERENCES wiki_extension_sets(id) ON DELETE CASCADE,
    ext_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(2048),
    version VARCHAR(255),
    license_name VARCHAR(255),
    PRIMARY KEY (set_id, ext_type, name)
);

-- Reverse extension lookups use the name prefix; set_id completes the join.
CREATE INDEX idx_wiki_extension_set_items_name_set
ON wiki_extension_set_items(name, set_id);

ALTER TABLE wiki_extensions_snapshots
ADD COLUMN extension_set_id BIGINT NULL
REFERENCES wiki_extension_sets(id);

CREATE INDEX idx_wiki_extensions_current_set
ON wiki_extensions_snapshots(extension_set_id)
WHERE valid_until IS NULL AND extension_set_id IS NOT NULL;

-- New code dual-writes legacy members until the online backfill is complete.
CREATE TABLE extension_storage_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    legacy_writes BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO extension_storage_state(singleton, legacy_writes)
VALUES (TRUE, TRUE);
