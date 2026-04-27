-- name: ListProviderEndpoints :many
SELECT * FROM provider_endpoint
WHERE provider_id = $1
ORDER BY endpoint_path;

-- name: UpsertProviderEndpoint :one
INSERT INTO provider_endpoint (provider_id, endpoint_path, upstream_url)
VALUES ($1, $2, $3)
ON CONFLICT (provider_id, endpoint_path) DO UPDATE SET
  upstream_url = EXCLUDED.upstream_url
RETURNING *;

-- name: DeleteProviderEndpoint :exec
DELETE FROM provider_endpoint
WHERE provider_id = $1 AND endpoint_path = $2;

-- name: GetProviderEndpoint :one
SELECT * FROM provider_endpoint
WHERE provider_id = $1 AND endpoint_path = $2;
