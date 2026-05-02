-- name: GetProviderByID :one
SELECT * FROM provider WHERE id = $1 LIMIT 1;

-- name: GetProviders :many
SELECT * FROM provider;

-- name: CreateProvider :one
INSERT INTO provider (name, credentials, priority, provider_models, annotations, disabled) VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: UpdateProvider :one
UPDATE provider
  SET
    name = CASE WHEN @set_name::bool THEN @name::text ELSE name END,
    credentials = CASE WHEN @set_credentials::bool THEN @credentials::text ELSE credentials END,
    priority = CASE WHEN @set_priority::bool THEN @priority::int ELSE priority END,
    provider_models = CASE WHEN @set_provider_models::bool THEN @provider_models::jsonb ELSE provider_models END,
    annotations = CASE WHEN @set_annotations::bool THEN @annotations::jsonb ELSE annotations END,
    disabled = CASE WHEN @set_disabled::bool THEN @disabled::bool ELSE disabled END
  WHERE id = @id::int
  RETURNING *;

-- name: DeleteProvider :exec
DELETE FROM provider WHERE id = $1;
