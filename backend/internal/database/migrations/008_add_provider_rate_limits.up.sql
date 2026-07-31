CREATE TABLE provider_rate_limits (
    provider VARCHAR(255) PRIMARY KEY,
    retry_at TIMESTAMP NOT NULL,
    consecutive_rate_limits INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_provider_rate_limits_retry_at ON provider_rate_limits(retry_at);
