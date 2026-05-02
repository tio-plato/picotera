-- name: GetModelByName :one
SELECT * FROM model WHERE name = $1 LIMIT 1;

-- name: GetModels :many
SELECT * FROM model;

-- name: UpsertModel :one
INSERT INTO model (name, title, developer, series, disabled) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (name) DO UPDATE SET title = $2, developer = $3, series = $4, disabled = $5 RETURNING *;

-- name: DeleteModel :exec
DELETE FROM model WHERE name = $1;
