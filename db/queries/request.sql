-- name: ListRequests :many
SELECT id, span_id, parent_span_id, provider_id, endpoint_path, api_key_id, model,
       input_tokens, cache_read_tokens, output_tokens, cache_write_tokens,
       status_code, error_message, ttft_ms, time_spent_ms, created_at
FROM request
WHERE
  (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id'))
  AND (sqlc.narg('endpoint_path')::text IS NULL OR endpoint_path = sqlc.narg('endpoint_path'))
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model'))
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::text)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.narg('limit')::int;

-- name: GetRequest :one
SELECT * FROM request WHERE id = $1;
