-- name: ListRequests :many
SELECT id, span_id, parent_span_id, type, status, provider_id, endpoint_path, api_key_id, model,
       upstream_model, input_tokens, cache_read_tokens, output_tokens, cache_write_tokens, cache_write_1h_tokens,
       status_code, error_message, ttft_ms, time_spent_ms, created_at,
       model_cost, model_cost_currency, upstream_cost, upstream_cost_currency,
       user_message_preview
FROM request
WHERE
  (sqlc.narg('type')::int IS NULL OR type = sqlc.narg('type'))
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id'))
  AND (sqlc.narg('endpoint_path')::text IS NULL OR endpoint_path = sqlc.narg('endpoint_path'))
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model'))
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model'))
  AND (sqlc.narg('parent_span_id')::text IS NULL OR parent_span_id = sqlc.narg('parent_span_id'))
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::text)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.narg('limit')::int;

-- name: ListRequestTraces :many
WITH trace_base AS (
  SELECT
    parent_span_id,
    COUNT(*) FILTER (WHERE type = 0)::bigint AS meta_request_count,
    COUNT(*) FILTER (WHERE type = 1)::bigint AS upstream_request_count,
    COALESCE(SUM(
      COALESCE(input_tokens, 0)
      + COALESCE(cache_read_tokens, 0)
      + COALESCE(output_tokens, 0)
      + COALESCE(cache_write_tokens, 0)
      + COALESCE(cache_write_1h_tokens, 0)
    ) FILTER (WHERE type = 1), 0)::bigint AS total_tokens,
    COALESCE(SUM(COALESCE(input_tokens, 0)) FILTER (WHERE type = 1), 0)::bigint AS input_tokens,
    COALESCE(SUM(COALESCE(cache_read_tokens, 0)) FILTER (WHERE type = 1), 0)::bigint AS cache_read_tokens,
    COALESCE(SUM(COALESCE(output_tokens, 0)) FILTER (WHERE type = 1), 0)::bigint AS output_tokens,
    COALESCE(SUM(COALESCE(cache_write_tokens, 0)) FILTER (WHERE type = 1), 0)::bigint AS cache_write_tokens,
    COALESCE(SUM(COALESCE(cache_write_1h_tokens, 0)) FILTER (WHERE type = 1), 0)::bigint AS cache_write_1h_tokens,
    MAX(created_at)::timestamp AS last_request_at
  FROM request
  WHERE parent_span_id IS NOT NULL AND parent_span_id <> ''
  GROUP BY parent_span_id
)
SELECT
  trace_base.parent_span_id,
  trace_base.meta_request_count,
  trace_base.upstream_request_count,
  trace_base.total_tokens,
  trace_base.input_tokens,
  trace_base.cache_read_tokens,
  trace_base.output_tokens,
  trace_base.cache_write_tokens,
  trace_base.cache_write_1h_tokens,
  COALESCE(model_costs.costs, '[]'::jsonb)::jsonb AS model_costs,
  COALESCE(upstream_costs.costs, '[]'::jsonb)::jsonb AS upstream_costs,
  trace_base.last_request_at,
  preview.user_message_preview
FROM trace_base
LEFT JOIN LATERAL (
  SELECT jsonb_agg(
    jsonb_build_object('currency', grouped.currency, 'amount', grouped.amount)
    ORDER BY grouped.currency
  ) AS costs
  FROM (
    SELECT model_cost_currency AS currency, SUM(model_cost)::float8 AS amount
    FROM request
    WHERE parent_span_id = trace_base.parent_span_id
      AND type = 1
      AND model_cost IS NOT NULL
      AND model_cost_currency IS NOT NULL
    GROUP BY model_cost_currency
  ) grouped
) model_costs ON true
LEFT JOIN LATERAL (
  SELECT jsonb_agg(
    jsonb_build_object('currency', grouped.currency, 'amount', grouped.amount)
    ORDER BY grouped.currency
  ) AS costs
  FROM (
    SELECT upstream_cost_currency AS currency, SUM(upstream_cost)::float8 AS amount
    FROM request
    WHERE parent_span_id = trace_base.parent_span_id
      AND type = 1
      AND upstream_cost IS NOT NULL
      AND upstream_cost_currency IS NOT NULL
    GROUP BY upstream_cost_currency
  ) grouped
) upstream_costs ON true
LEFT JOIN LATERAL (
  SELECT user_message_preview
  FROM request
  WHERE parent_span_id = trace_base.parent_span_id
    AND type = 0
    AND user_message_preview IS NOT NULL
  ORDER BY created_at DESC, id DESC
  LIMIT 1
) preview ON true
WHERE
  sqlc.narg('cursor_last_request_at')::timestamp IS NULL
  OR (trace_base.last_request_at, trace_base.parent_span_id) < (
    sqlc.narg('cursor_last_request_at')::timestamp,
    sqlc.narg('cursor_parent_span_id')::text
  )
ORDER BY trace_base.last_request_at DESC, trace_base.parent_span_id DESC
LIMIT sqlc.narg('limit')::int;

-- name: GetRequest :one
SELECT * FROM request WHERE id = $1;

-- name: ListRequestsBySpan :many
WITH anchor AS (
  SELECT request.span_id FROM request WHERE request.id = $1
)
SELECT r.id, r.span_id, r.parent_span_id, r.type, r.status, r.provider_id, r.endpoint_path,
       r.api_key_id, r.model, r.upstream_model, r.input_tokens, r.cache_read_tokens, r.output_tokens,
       r.cache_write_tokens, r.cache_write_1h_tokens, r.status_code, r.error_message, r.ttft_ms, r.time_spent_ms,
       r.created_at,
       r.model_cost, r.model_cost_currency, r.upstream_cost, r.upstream_cost_currency,
       r.user_message_preview
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
    cache_read_tokens = $9, cache_write_tokens = $10,
    cache_write_1h_tokens = $11,
    model_cost = $12, model_cost_currency = $13,
    upstream_cost = $14, upstream_cost_currency = $15
WHERE id = $1;

-- name: UpdateRequestModel :exec
UPDATE request SET model = $2 WHERE id = $1;

-- name: UpdateRequestMetrics :exec
UPDATE request
SET ttft_ms = $2, input_tokens = $3, output_tokens = $4,
    cache_read_tokens = $5, cache_write_tokens = $6, cache_write_1h_tokens = $7
WHERE id = $1;
