-- name: GetUserByID :one
SELECT * FROM app_user WHERE id = $1 LIMIT 1;

-- name: GetUserByIdentity :one
SELECT u.* FROM app_user u
JOIN user_identity i ON i.user_id = u.id
WHERE i.provider = $1 AND i.identity = $2
LIMIT 1;

-- name: InsertUser :one
INSERT INTO app_user (display_name, is_admin) VALUES ($1, $2) RETURNING *;

-- name: InsertUserIdentity :one
INSERT INTO user_identity (user_id, provider, identity)
VALUES ($1, $2, $3)
ON CONFLICT (provider, identity) DO NOTHING
RETURNING *;

-- name: UpdateUserAdmin :one
UPDATE app_user SET is_admin = $2, updated_at = now() WHERE id = $1 RETURNING *;
