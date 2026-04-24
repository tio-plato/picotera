-- name: GetEndpointByPath :one
SELECT * FROM endpoint WHERE path = $1 LIMIT 1;

-- name: GetProvidersByEndpointAndModel :many
SELECT mpe.model_name, mpe.provider_id, mpe.endpoint_path, mpe.upstream_model_name, mpe.priority, mpe.annotations, p.name AS provider_name, p.credentials AS provider_credentials, p.priority AS provider_priority, pe.upstream_url
  FROM model_provider_endpoint AS mpe
  LEFT JOIN provider AS p ON mpe.provider_id = p.id
  LEFT JOIN provider_endpoint AS pe ON mpe.provider_id = pe.provider_id AND mpe.endpoint_path = pe.endpoint_path
  WHERE mpe.endpoint_path = $1 AND mpe.model_name = $2;

-- name: GetApiKeyByHash :one
SELECT * FROM api_key WHERE api_key_hash = $1 LIMIT 1;

-- name: InsertRequest :exec
INSERT INTO request (id, provider_id, endpoint_path, model, status_code, error_message, time_spent_ms)
VALUES ($1, $2, $3, $4, $5, $6, $7);