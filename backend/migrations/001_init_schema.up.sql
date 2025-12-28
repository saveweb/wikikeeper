-- Create wikis table
CREATE TABLE IF NOT EXISTS wikis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url VARCHAR(2048) NOT NULL UNIQUE,
    api_url VARCHAR(2048),
    index_url VARCHAR(2048),
    wiki_name VARCHAR(255),

    -- MediaWiki metadata
    sitename VARCHAR(255),
    lang VARCHAR(10),
    db_type VARCHAR(50),
    db_version VARCHAR(50),
    media_wiki_version VARCHAR(50),
    max_page_id INTEGER,

    -- Status tracking
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ok', 'error', 'offline')),
    has_archive BOOLEAN NOT NULL DEFAULT false,
    api_available BOOLEAN NOT NULL DEFAULT true,

    -- Error tracking
    last_error TEXT,
    last_error_at TIMESTAMP,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_check_at TIMESTAMP,

    -- Settings
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- Create indexes for wikis
CREATE INDEX IF NOT EXISTS idx_wikis_url ON wikis(url);
CREATE INDEX IF NOT EXISTS idx_wikis_status ON wikis(status);
CREATE INDEX IF NOT EXISTS idx_wikis_has_archive ON wikis(has_archive);
CREATE INDEX IF NOT EXISTS idx_wikis_api_url ON wikis(api_url);
CREATE INDEX IF NOT EXISTS idx_wikis_created_at ON wikis(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wikis_updated_at ON wikis(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_wikis_last_check_at ON wikis(last_check_at);
CREATE INDEX IF NOT EXISTS idx_wikis_sitename ON wikis(sitename);

-- Create trigger function to update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for wikis
DROP TRIGGER IF EXISTS update_wikis_updated_at ON wikis;
CREATE TRIGGER update_wikis_updated_at BEFORE UPDATE ON wikis
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create wiki_stats table
CREATE TABLE IF NOT EXISTS wiki_stats (
    id BIGSERIAL PRIMARY KEY,
    wiki_id UUID NOT NULL REFERENCES wikis(id) ON DELETE CASCADE,
    time TIMESTAMP NOT NULL,

    -- Statistics
    pages INTEGER NOT NULL DEFAULT 0,
    articles INTEGER NOT NULL DEFAULT 0,
    edits INTEGER NOT NULL DEFAULT 0,
    images INTEGER NOT NULL DEFAULT 0,
    users INTEGER NOT NULL DEFAULT 0,
    active_users INTEGER NOT NULL DEFAULT 0,
    admins INTEGER NOT NULL DEFAULT 0,
    jobs INTEGER NOT NULL DEFAULT 0,

    -- Availability metrics
    response_time_ms INTEGER,
    http_status INTEGER
);

-- Create indexes for wiki_stats
CREATE INDEX IF NOT EXISTS idx_wiki_stats_wiki_id ON wiki_stats(wiki_id);
CREATE INDEX IF NOT EXISTS idx_wiki_stats_time ON wiki_stats(time DESC);
CREATE INDEX IF NOT EXISTS idx_wiki_stats_wiki_time ON wiki_stats(wiki_id, time DESC);

-- Create wiki_archives table
CREATE TABLE IF NOT EXISTS wiki_archives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wiki_id UUID NOT NULL REFERENCES wikis(id) ON DELETE CASCADE,
    ia_identifier VARCHAR(255) NOT NULL,

    -- Archive metadata
    added_date TIMESTAMP,
    dump_date TIMESTAMP,
    item_size BIGINT,
    uploader VARCHAR(255),
    scanner VARCHAR(255),
    upload_state VARCHAR(50),

    -- Dump content flags
    has_xml_current BOOLEAN NOT NULL DEFAULT false,
    has_xml_history BOOLEAN NOT NULL DEFAULT false,
    has_images_dump BOOLEAN NOT NULL DEFAULT false,
    has_titles_list BOOLEAN NOT NULL DEFAULT false,
    has_images_list BOOLEAN NOT NULL DEFAULT false,
    has_legacy_wikidump BOOLEAN NOT NULL DEFAULT false,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Unique constraint
    CONSTRAINT unique_wiki_archive UNIQUE (wiki_id, ia_identifier)
);

-- Create indexes for wiki_archives
CREATE INDEX IF NOT EXISTS idx_wiki_archives_wiki_id ON wiki_archives(wiki_id);
CREATE INDEX IF NOT EXISTS idx_wiki_archives_ia_identifier ON wiki_archives(ia_identifier);
CREATE INDEX IF NOT EXISTS idx_wiki_archives_dump_date ON wiki_archives(dump_date DESC);

-- Create trigger for wiki_archives
DROP TRIGGER IF EXISTS update_wiki_archives_updated_at ON wiki_archives;
CREATE TRIGGER update_wiki_archives_updated_at BEFORE UPDATE ON wiki_archives
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
