-- name: ListRequests :many
SELECT id, span_id, parent_span_id, type, status, provider_id, endpoint_path, api_key_id, model,
       upstream_model, input_tokens, cache_read_tokens, output_tokens, cache_write_tokens,
       status_code, error_message, ttft_ms, time_spent_ms, created_at
FROM request
WHERE
  (sqlc.narg('type')::int IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id'))
  AND (sqlc.narg('endpoint_path')::text IS NULL OR endpoint_path = sqlc.narg('endpoint_path'))
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model'))
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model'))
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::text)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.narg('limit')::int;

-- name: GetRequest :one
SELECT * FROM request WHERE id = $1;

-- name: ListRequestsBySpan :many
WITH anchor AS (
  SELECT request.span_id FROM request WHERE request.id = $1
)
SELECT r.id, r.span_id, r.parent_span_id, r.type, r.status, r.provider_id, r.endpoint_path,
       r.api_key_id, r.model, r.upstream_model, r.input_tokens, r.cache_read_tokens, r.output_tokens,
       r.cache_write_tokens, r.status_code, r.error_message, r.ttft_ms, r.time_spent_ms,
       r.created_at
FROM request r, anchor
WHERE r.span_id = anchor.span_id
ORDER BY r.created_at ASC, r.id ASC;

-- name: UpdateRequestOnHeader :exec
UPDATE request
SET provider_id = $2, model = $3, upstream_model = $4, endpoint_path = $5, api_key_id = $6, status = $7
WHERE id = $1;

-- name: UpdateRequestOnComplete :exec
UPDATE request
SET status_code = $2, error_message = $3, time_spent_ms = $4, status = $5,
    ttft_ms = $6, input_tokens = $7, output_tokens = $8,
    cache_read_tokens = $9, cache_write_tokens = $10
WHERE id = $1;

-- name: UpdateRequestModel :exec
UPDATE request SET model = $2 WHERE id = $1;

-- name: UpdateRequestMetrics :exec
UPDATE request
SET ttft_ms = $2, input_tokens = $3, output_tokens = $4,
    cache_read_tokens = $5, cache_write_tokens = $6
WHERE id = $1;
