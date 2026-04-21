-- name: GetModelProviderEndpoint :one
SELECT * FROM model_provider_endpoint
WHERE model_name = $1 AND provider_id = $2 AND endpoint_id = $3;

-- name: ListModelProviderEndpoints :many
SELECT * FROM model_provider_endpoint
WHERE
  (sqlc.narg('model_name')::text IS NULL OR model_name = sqlc.narg('model_name'))
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id'))
  AND (sqlc.narg('endpoint_id')::int IS NULL OR endpoint_id = sqlc.narg('endpoint_id'))
  AND (
    sqlc.narg('cursor_model_name')::text IS NULL
    OR (model_name, provider_id, endpoint_id) > (sqlc.narg('cursor_model_name'), sqlc.narg('cursor_provider_id')::int, sqlc.narg('cursor_endpoint_id')::int)
  )
ORDER BY model_name, provider_id, endpoint_id
LIMIT sqlc.narg('limit')::int;

-- name: UpsertModelProviderEndpoint :one
INSERT INTO model_provider_endpoint (model_name, provider_id, endpoint_id, upstream_model_name, priority, annotations)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (model_name, provider_id, endpoint_id) DO UPDATE SET
  upstream_model_name = EXCLUDED.upstream_model_name,
  priority = EXCLUDED.priority,
  annotations = EXCLUDED.annotations
RETURNING *;

-- name: DeleteModelProviderEndpoint :exec
DELETE FROM model_provider_endpoint
WHERE model_name = $1 AND provider_id = $2 AND endpoint_id = $3;
