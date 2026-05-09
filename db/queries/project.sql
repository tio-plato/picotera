-- name: ListProjects :many
SELECT * FROM project ORDER BY name ASC;

-- name: GetProject :one
SELECT * FROM project WHERE id = $1 LIMIT 1;

-- name: GetProjectByName :one
SELECT * FROM project WHERE name = $1 LIMIT 1;

-- name: InsertProject :one
INSERT INTO project (name, paths) VALUES ($1, $2) RETURNING *;

-- name: UpdateProject :one
UPDATE project SET name = $2, paths = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteProject :exec
DELETE FROM project WHERE id = $1;

-- name: ListProjectPaths :many
SELECT id AS project_id, jsonb_array_elements_text(paths) AS path
FROM project
WHERE jsonb_array_length(paths) > 0;

-- name: UpsertProjectSeen :exec
UPDATE project
SET first_seen_at = LEAST(COALESCE(first_seen_at, sqlc.arg('seen_at')::timestamp), sqlc.arg('seen_at')::timestamp),
    last_seen_at  = GREATEST(COALESCE(last_seen_at,  sqlc.arg('seen_at')::timestamp), sqlc.arg('seen_at')::timestamp),
    updated_at    = now()
WHERE id = $1;
