-- name: GetEndpoints :many
SELECT * FROM endpoint;

-- name: UpsertEndpoint :one
INSERT INTO endpoint (name, path, model_path, credentials_resolver, endpoint_type) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (path) DO UPDATE SET model_path = $3, credentials_resolver = $4, endpoint_type = $5 RETURNING *;

-- name: DeleteEndpoint :exec
DELETE FROM endpoint WHERE path = $1;
