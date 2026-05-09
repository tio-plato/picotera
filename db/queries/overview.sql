-- name: GetOverviewTotals :one
WITH filtered AS (
  SELECT
    cost_currency,
    request_count,
    input_tokens,
    cache_read_tokens,
    output_tokens,
    cache_write_tokens,
    cache_write_1h_tokens,
    cost
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
), totals AS (
  SELECT
    COALESCE(SUM(
      input_tokens + cache_read_tokens + output_tokens + cache_write_tokens + cache_write_1h_tokens
    ), 0)::bigint AS total_tokens,
    COALESCE(SUM(request_count), 0)::bigint AS total_requests
  FROM filtered
), cost_groups AS (
  SELECT cost_currency AS currency, SUM(cost)::float8 AS amount
  FROM filtered
  WHERE cost_currency IS NOT NULL AND cost_currency <> ''
  GROUP BY cost_currency
), cost_json AS (
  SELECT COALESCE(
    jsonb_agg(
      jsonb_build_object('currency', currency, 'amount', amount)
      ORDER BY currency
    ),
    '[]'::jsonb
  )::jsonb AS costs
  FROM cost_groups
)
SELECT
  totals.total_tokens,
  totals.total_requests,
  cost_json.costs::jsonb AS costs
FROM totals CROSS JOIN cost_json;

-- name: CountTraces :one
SELECT COUNT(*)::bigint AS trace_count
FROM traces
WHERE last_request_at >= sqlc.arg('start_at')::timestamp
  AND last_request_at < sqlc.arg('end_at')::timestamp;

-- name: CountTracesFiltered :one
SELECT COUNT(*)::bigint AS trace_count
FROM traces t
WHERE t.last_request_at >= sqlc.arg('start_at')::timestamp
  AND t.last_request_at < sqlc.arg('end_at')::timestamp
  AND EXISTS (
    SELECT 1
    FROM request r
    WHERE r.parent_span_id = t.parent_span_id
      AND r.created_at >= t.first_request_at
      AND r.created_at <= t.last_request_at
      AND r.type = 1
      AND (sqlc.narg('api_key_id')::int IS NULL OR r.api_key_id = sqlc.narg('api_key_id')::int)
      AND (sqlc.narg('model')::text IS NULL OR r.model = sqlc.narg('model')::text)
      AND (sqlc.narg('upstream_model')::text IS NULL OR r.upstream_model = sqlc.narg('upstream_model')::text)
      AND (sqlc.narg('provider_id')::int IS NULL OR r.provider_id = sqlc.narg('provider_id')::int)
  );

-- name: ListOverviewDistribution :many
SELECT
  CASE sqlc.arg('dimension')::text
    WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
    WHEN 'model' THEN COALESCE(model, '')
    WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
    WHEN 'provider' THEN COALESCE(provider_id::text, '')
    ELSE ''
  END AS key,
  SUM(
    input_tokens + cache_read_tokens + output_tokens + cache_write_tokens + cache_write_1h_tokens
  )::bigint AS total_tokens,
  SUM(request_count)::bigint AS request_count
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
GROUP BY key
ORDER BY total_tokens DESC, key ASC;

-- name: ListOverviewDistributionCosts :many
SELECT
  CASE sqlc.arg('dimension')::text
    WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
    WHEN 'model' THEN COALESCE(model, '')
    WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
    WHEN 'provider' THEN COALESCE(provider_id::text, '')
    ELSE ''
  END AS key,
  cost_currency::text AS currency,
  SUM(cost)::float8 AS amount
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  AND cost_currency IS NOT NULL
  AND cost_currency <> ''
GROUP BY key, currency
ORDER BY key ASC, currency ASC;

-- name: ListOverviewTraceCountsByDimension :many
SELECT
  CASE sqlc.arg('dimension')::text
    WHEN 'apiKey' THEN COALESCE(r.api_key_id::text, '')
    WHEN 'model' THEN COALESCE(r.model, '')
    WHEN 'upstreamModel' THEN COALESCE(r.upstream_model, '')
    WHEN 'provider' THEN COALESCE(r.provider_id::text, '')
    ELSE ''
  END AS key,
  COUNT(DISTINCT r.parent_span_id)::bigint AS trace_count
FROM request r
WHERE r.created_at >= sqlc.arg('start_at')::timestamp
  AND r.created_at < sqlc.arg('end_at')::timestamp
  AND r.type = 1
  AND r.parent_span_id IS NOT NULL
  AND r.parent_span_id <> ''
  AND (sqlc.narg('api_key_id')::int IS NULL OR r.api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR r.model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR r.upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR r.provider_id = sqlc.narg('provider_id')::int)
GROUP BY key;

-- name: ListOverviewSeriesMetrics :many
SELECT
  bucket_at::timestamp AS bucket_at,
  CASE sqlc.arg('dimension')::text
    WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
    WHEN 'model' THEN COALESCE(model, '')
    WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
    WHEN 'provider' THEN COALESCE(provider_id::text, '')
    ELSE ''
  END AS group_key,
  COALESCE(cost_currency, '')::text AS currency,
  SUM(input_tokens + cache_read_tokens + output_tokens + cache_write_tokens + cache_write_1h_tokens)::bigint AS tokens,
  SUM(request_count)::bigint AS requests,
  SUM(cost)::float8 AS cost
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
GROUP BY bucket_at, group_key, currency
ORDER BY bucket_at ASC, group_key ASC, currency ASC;

-- name: ListOverviewSeriesTraces :many
SELECT
  time_bucket(INTERVAL '1 hour', r.created_at)::timestamp AS bucket_at,
  CASE sqlc.arg('dimension')::text
    WHEN 'apiKey' THEN COALESCE(r.api_key_id::text, '')
    WHEN 'model' THEN COALESCE(r.model, '')
    WHEN 'upstreamModel' THEN COALESCE(r.upstream_model, '')
    WHEN 'provider' THEN COALESCE(r.provider_id::text, '')
    ELSE ''
  END AS group_key,
  COUNT(DISTINCT r.parent_span_id)::bigint AS trace_count
FROM request r
WHERE r.created_at >= sqlc.arg('start_at')::timestamp
  AND r.created_at < sqlc.arg('end_at')::timestamp
  AND r.type = 1
  AND r.parent_span_id IS NOT NULL
  AND r.parent_span_id <> ''
  AND (sqlc.narg('api_key_id')::int IS NULL OR r.api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR r.model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR r.upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR r.provider_id = sqlc.narg('provider_id')::int)
GROUP BY bucket_at, group_key
ORDER BY bucket_at ASC, group_key ASC;

-- name: GetOverviewTokenBreakdown :one
SELECT
  COALESCE(SUM(input_tokens), 0)::bigint           AS input_tokens,
  COALESCE(SUM(cache_read_tokens), 0)::bigint      AS cache_read_tokens,
  COALESCE(SUM(cache_write_tokens), 0)::bigint     AS cache_write_tokens,
  COALESCE(SUM(cache_write_1h_tokens), 0)::bigint  AS cache_write_1h_tokens,
  COALESCE(SUM(output_tokens), 0)::bigint          AS output_tokens
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int);

-- name: ListOverviewBreakdownTokens :many
SELECT
  COALESCE(api_key_id, 0)::int          AS api_key_id,
  COALESCE(model, '')::text             AS model,
  COALESCE(upstream_model, '')::text    AS upstream_model,
  COALESCE(provider_id, 0)::int         AS provider_id,
  SUM(
    input_tokens + cache_read_tokens + output_tokens + cache_write_tokens + cache_write_1h_tokens
  )::bigint AS total_tokens
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
GROUP BY 1, 2, 3, 4
HAVING SUM(
  input_tokens + cache_read_tokens + output_tokens + cache_write_tokens + cache_write_1h_tokens
) > 0;

-- name: ListOverviewBreakdownCosts :many
SELECT
  COALESCE(api_key_id, 0)::int          AS api_key_id,
  COALESCE(model, '')::text             AS model,
  COALESCE(upstream_model, '')::text    AS upstream_model,
  COALESCE(provider_id, 0)::int         AS provider_id,
  cost_currency::text                    AS currency,
  SUM(cost)::float8                      AS amount
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  AND cost_currency IS NOT NULL
  AND cost_currency <> ''
GROUP BY 1, 2, 3, 4, 5;
