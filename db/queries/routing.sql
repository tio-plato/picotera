-- name: GetEndpointByPath :one
SELECT * FROM endpoint WHERE path = $1 LIMIT 1;

-- name: GetProvidersByEndpointAndModel :many
SELECT
  sqlc.arg('model_name')::text AS model_name,
  p.id AS provider_id,
  pe.endpoint_path,
  COALESCE(sub.pm ->> 'upstreamModelName', '')::text AS upstream_model_name,
  COALESCE((sub.pm ->> 'priority')::int, 0)::int AS priority,
  (COALESCE(sub.pm -> 'annotations', '{}'::jsonb))::jsonb AS annotations,
  p.name AS provider_name,
  p.credentials AS provider_credentials,
  p.priority AS provider_priority,
  pe.upstream_url,
  p.annotations AS provider_annotations
FROM provider AS p
JOIN provider_endpoint AS pe ON pe.provider_id = p.id
CROSS JOIN LATERAL (SELECT p.provider_models -> sqlc.arg('model_name')::text AS pm) sub
WHERE pe.endpoint_path = sqlc.arg('endpoint_path')::text
  AND p.provider_models ? sqlc.arg('model_name')::text
  AND sub.pm IS NOT NULL
  AND (
    sub.pm -> 'endpoints' IS NULL
    OR jsonb_typeof(sub.pm -> 'endpoints') <> 'array'
    OR jsonb_array_length(sub.pm -> 'endpoints') = 0
    OR sub.pm -> 'endpoints' @> to_jsonb(ARRAY[pe.endpoint_path])
  );

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
