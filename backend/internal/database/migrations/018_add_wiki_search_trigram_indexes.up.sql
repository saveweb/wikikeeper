CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_wikis_sitename_trgm
ON wikis USING GIN (LOWER(sitename) gin_trgm_ops);

CREATE INDEX idx_wikis_url_trgm
ON wikis USING GIN (LOWER(url) gin_trgm_ops);

CREATE INDEX idx_wikis_api_url_trgm
ON wikis USING GIN (LOWER(api_url) gin_trgm_ops);

CREATE INDEX idx_wikis_index_url_trgm
ON wikis USING GIN (LOWER(index_url) gin_trgm_ops);
