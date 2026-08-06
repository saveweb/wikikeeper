UPDATE wikis
SET farm = 'wikigg'
WHERE farm IS NULL
  AND lower(url) ~ '^https?://([^/]+\.)?wiki\.gg([/:]|$)';
