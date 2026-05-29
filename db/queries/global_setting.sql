-- name: ListGlobalSettings :many
SELECT * FROM global_setting ORDER BY key;

-- name: GetGlobalSetting :one
SELECT * FROM global_setting WHERE key = $1 LIMIT 1;

-- name: UpsertGlobalSetting :one
INSERT INTO global_setting (key, value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()
RETURNING *;

-- name: DeleteGlobalSetting :execrows
DELETE FROM global_setting WHERE key = $1;
