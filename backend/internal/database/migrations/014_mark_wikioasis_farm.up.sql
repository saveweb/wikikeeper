UPDATE wikis
SET farm = 'wikioasis'
WHERE farm IS NULL
  AND lower(url) ~ '^https?://([^/]+\.)?wikioasis\.org([/:]|$)';
