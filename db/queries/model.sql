-- name: GetModelByName :one
SELECT name, disabled, pricing, annotations FROM model WHERE name = $1 LIMIT 1;

-- name: GetModels :many
SELECT name, disabled, pricing, annotations FROM model;

-- name: UpsertModel :one
INSERT INTO model (name, disabled, pricing, annotations) VALUES ($1, $2, $3, $4) ON CONFLICT (name) DO UPDATE SET disabled = $2, pricing = $3, annotations = $4 RETURNING *;

-- name: DeleteModel :exec
DELETE FROM model WHERE name = $1;
