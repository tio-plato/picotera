-- name: GetEndpointByPath :one
SELECT * FROM endpoint WHERE path = $1 LIMIT 1;

-- name: GetProvidersByEndpointAndModel :many
SELECT mpe.model_name, mpe.provider_id, mpe.endpoint_path, mpe.upstream_model_name, mpe.priority, mpe.annotations, p.name AS provider_name, p.credentials AS provider_credentials, p.priority AS provider_priority, pe.upstream_url, p.annotations AS provider_annotations
  FROM model_provider_endpoint AS mpe
  LEFT JOIN provider AS p ON mpe.provider_id = p.id
  LEFT JOIN provider_endpoint AS pe ON mpe.provider_id = pe.provider_id AND mpe.endpoint_path = pe.endpoint_path
  WHERE mpe.endpoint_path = $1 AND mpe.model_name = $2;

-- name: GetApiKeyByHash :one
SELECT * FROM api_key WHERE api_key_hash = $1 LIMIT 1;

-- name: InsertRequest :one
INSERT INTO request (
  id, span_id, parent_span_id, type, status,
  provider_id, endpoint_path, api_key_id, model, upstream_model,
  input_tokens, cache_read_tokens, output_tokens, cache_write_tokens,
  status_code, error_message, ttft_ms, time_spent_ms
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  $11, $12, $13, $14,
  $15, $16, $17, $18
)
RETURNING created_at;
