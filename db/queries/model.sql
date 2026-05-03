-- name: GetModelByName :one
SELECT * FROM model WHERE name = $1 LIMIT 1;

-- name: GetModels :many
SELECT * FROM model;

-- name: UpsertModel :one
INSERT INTO model (name, title, developer, series, disabled, pricing) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (name) DO UPDATE SET title = $2, developer = $3, series = $4, disabled = $5, pricing = $6 RETURNING *;

-- name: DeleteModel :exec
DELETE FROM model WHERE name = $1;
