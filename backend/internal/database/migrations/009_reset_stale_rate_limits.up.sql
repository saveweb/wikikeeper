-- Expired rate-limit outcomes are historical collection results, not an
-- indication that the provider is currently throttling WikiKeeper. Queue them
-- for a fresh check over a deterministic seven-day window so the backlog does
-- not monopolize collection batches after deployment.
UPDATE wikis
SET collection_status = 'pending',
    consecutive_failures = 0,
    next_check_at = NOW()
        + MOD(ABS(HASHTEXT(id::text)::bigint), 604800) * INTERVAL '1 second'
WHERE collection_status = 'rate_limited'
  AND (next_check_at IS NULL OR next_check_at <= NOW());
