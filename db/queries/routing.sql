-- name: GetEndpointByPath :one
SELECT * FROM endpoint WHERE path = $1 LIMIT 1;

-- name: GetProvidersByEndpointAndModel :many
SELECT mpe.*, p.name AS provider_name, p.credentials AS provider_credentials
  FROM model_provider_endpoint AS mpe
  LEFT JOIN provider AS p ON mpe.provider_id = p.id
  WHERE mpe.endpoint_path = $1 AND mpe.model_name = $2;

-- name: GetApiKeyByHash :one
SELECT * FROM api_key WHERE api_key_hash = $1 LIMIT 1;

