-- +goose Up
ALTER TABLE provider
  ADD COLUMN models_endpoint_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN models_endpoint_resolver INTEGER NOT NULL DEFAULT 0;

WITH first_binding AS (
  SELECT DISTINCT ON (pe.provider_id)
    pe.provider_id,
    pe.upstream_url,
    COALESCE(NULLIF(pe.credentials_resolver, 0), e.credentials_resolver) AS resolver
  FROM provider_endpoint pe
  JOIN endpoint e ON e.path = pe.endpoint_path
  WHERE e.endpoint_type = 6
  ORDER BY pe.provider_id, pe.endpoint_path
)
UPDATE provider p
SET models_endpoint_url = fb.upstream_url,
    models_endpoint_resolver = fb.resolver
FROM first_binding fb
WHERE p.id = fb.provider_id;

DELETE FROM provider_endpoint
WHERE endpoint_path IN (SELECT path FROM endpoint WHERE endpoint_type = 6);
DELETE FROM endpoint WHERE endpoint_type = 6;

-- +goose Down
ALTER TABLE provider
  DROP COLUMN models_endpoint_url,
  DROP COLUMN models_endpoint_resolver;
