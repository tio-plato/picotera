-- name: ListApiKeys :many
SELECT * FROM api_key WHERE user_id = $1 ORDER BY created_at DESC, id DESC;

-- name: GetApiKey :one
SELECT * FROM api_key WHERE id = $1 AND user_id = $2 LIMIT 1;

-- name: GetApiKeyByKey :one
SELECT * FROM api_key WHERE key = $1 LIMIT 1;

-- name: InsertApiKey :one
INSERT INTO api_key (name, key, disabled, annotations, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateApiKey :one
UPDATE api_key
SET name = $2, key = $3, disabled = $4, annotations = $5, updated_at = now()
WHERE id = $1 AND user_id = $6
RETURNING *;

-- name: DeleteApiKey :exec
DELETE FROM api_key WHERE id = $1 AND user_id = $2;
