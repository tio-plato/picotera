-- name: GetModelByName :one
SELECT name, title, developer, series, disabled, pricing, annotations FROM model WHERE name = $1 LIMIT 1;

-- name: GetModels :many
SELECT name, title, developer, series, disabled, pricing, annotations FROM model;

-- name: UpsertModel :one
INSERT INTO model (name, title, developer, series, disabled, pricing, annotations) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (name) DO UPDATE SET title = $2, developer = $3, series = $4, disabled = $5, pricing = $6, annotations = $7 RETURNING *;

-- name: DeleteModel :exec
DELETE FROM model WHERE name = $1;
