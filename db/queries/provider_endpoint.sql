-- name: ListProviderEndpoints :many
SELECT * FROM provider_endpoint
WHERE provider_id = $1
ORDER BY endpoint_id;

-- name: UpsertProviderEndpoint :one
INSERT INTO provider_endpoint (provider_id, endpoint_id, upstream_url)
VALUES ($1, $2, $3)
ON CONFLICT (provider_id, endpoint_id) DO UPDATE SET
  upstream_url = EXCLUDED.upstream_url
RETURNING *;

-- name: DeleteProviderEndpoint :exec
DELETE FROM provider_endpoint
WHERE provider_id = $1 AND endpoint_id = $2;
