-- name: GetOverviewSummary :one
SELECT
  COALESCE(SUM(request_count), 0)::bigint AS total_requests,
  COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int);

-- name: GetOverviewCostSummary :many
SELECT
  upstream_cost_currency AS currency,
  COALESCE(SUM(upstream_cost), 0)::numeric(20, 6) AS amount
FROM request_overview_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND upstream_cost IS NOT NULL
  AND upstream_cost_currency IS NOT NULL
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
GROUP BY upstream_cost_currency
ORDER BY upstream_cost_currency;

-- name: GetOverviewTraceCount :one
SELECT COUNT(*)::bigint AS trace_count
FROM traces t
WHERE t.last_request_at >= sqlc.arg('start_at')::timestamp
  AND t.last_request_at < sqlc.arg('end_at')::timestamp
  AND (
    (
      sqlc.narg('api_key_id')::int IS NULL
      AND sqlc.narg('model')::text IS NULL
      AND sqlc.narg('upstream_model')::text IS NULL
      AND sqlc.narg('provider_id')::int IS NULL
    )
    OR EXISTS (
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
    )
  );

-- name: GetOverviewDistribution :many
WITH metric_rows AS (
  SELECT
    CASE sqlc.arg('dimension')::text
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
    END AS group_key,
    SUM(request_count)::bigint AS request_count,
    SUM(total_tokens)::bigint AS total_tokens
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  GROUP BY group_key
),
cost_rows AS (
  SELECT
    CASE sqlc.arg('dimension')::text
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
    END AS group_key,
    upstream_cost_currency AS currency,
    SUM(upstream_cost)::numeric(20, 6) AS amount
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND upstream_cost IS NOT NULL
    AND upstream_cost_currency IS NOT NULL
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  GROUP BY group_key, upstream_cost_currency
),
cost_json AS (
  SELECT
    group_key,
    jsonb_agg(jsonb_build_object('currency', currency, 'amount', amount::float8) ORDER BY currency)::jsonb AS costs
  FROM cost_rows
  GROUP BY group_key
),
trace_rows AS (
  SELECT
    groups.group_key,
    COUNT(DISTINCT t.id)::bigint AS trace_count
  FROM traces t
  JOIN LATERAL (
    SELECT DISTINCT
      CASE sqlc.arg('dimension')::text
        WHEN 'apiKey' THEN COALESCE(r.api_key_id::text, '')
        WHEN 'model' THEN COALESCE(r.model, '')
        WHEN 'upstreamModel' THEN COALESCE(r.upstream_model, '')
        WHEN 'provider' THEN COALESCE(r.provider_id::text, '')
      END AS group_key
    FROM request r
    WHERE r.parent_span_id = t.parent_span_id
      AND r.created_at >= t.first_request_at
      AND r.created_at <= t.last_request_at
      AND r.type = 1
      AND (sqlc.narg('api_key_id')::int IS NULL OR r.api_key_id = sqlc.narg('api_key_id')::int)
      AND (sqlc.narg('model')::text IS NULL OR r.model = sqlc.narg('model')::text)
      AND (sqlc.narg('upstream_model')::text IS NULL OR r.upstream_model = sqlc.narg('upstream_model')::text)
      AND (sqlc.narg('provider_id')::int IS NULL OR r.provider_id = sqlc.narg('provider_id')::int)
  ) groups ON true
  WHERE t.last_request_at >= sqlc.arg('start_at')::timestamp
    AND t.last_request_at < sqlc.arg('end_at')::timestamp
  GROUP BY groups.group_key
)
SELECT
  sqlc.arg('dimension')::text AS dimension,
  COALESCE(metric_rows.group_key, trace_rows.group_key, cost_json.group_key, '')::text AS group_key,
  COALESCE(
    CASE sqlc.arg('dimension')::text
      WHEN 'apiKey' THEN api_key.name
      WHEN 'provider' THEN provider.name
      ELSE NULL
    END,
    COALESCE(metric_rows.group_key, trace_rows.group_key, cost_json.group_key, '')
  )::text AS group_label,
  COALESCE(metric_rows.total_tokens, 0)::bigint AS total_tokens,
  COALESCE(metric_rows.request_count, 0)::bigint AS request_count,
  COALESCE(trace_rows.trace_count, 0)::bigint AS trace_count,
  COALESCE(cost_json.costs, '[]'::jsonb)::jsonb AS costs
FROM metric_rows
FULL OUTER JOIN trace_rows ON trace_rows.group_key = metric_rows.group_key
FULL OUTER JOIN cost_json ON cost_json.group_key = COALESCE(metric_rows.group_key, trace_rows.group_key)
LEFT JOIN api_key ON sqlc.arg('dimension')::text = 'apiKey'
  AND COALESCE(metric_rows.group_key, trace_rows.group_key, cost_json.group_key) <> ''
  AND api_key.id = COALESCE(metric_rows.group_key, trace_rows.group_key, cost_json.group_key)::int
LEFT JOIN provider ON sqlc.arg('dimension')::text = 'provider'
  AND COALESCE(metric_rows.group_key, trace_rows.group_key, cost_json.group_key) <> ''
  AND provider.id = COALESCE(metric_rows.group_key, trace_rows.group_key, cost_json.group_key)::int
ORDER BY total_tokens DESC, request_count DESC, group_label ASC;

-- name: GetOverviewHourlyRequestSeries :many
WITH buckets AS (
  SELECT generate_series(
    sqlc.arg('start_at')::timestamp,
    sqlc.arg('end_at')::timestamp - INTERVAL '1 hour',
    INTERVAL '1 hour'
  )::timestamp AS bucket_at
),
groups AS (
  SELECT DISTINCT
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN ''
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
    END AS group_key
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
),
metric_rows AS (
  SELECT
    bucket_at,
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN ''
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
    END AS group_key,
    SUM(total_tokens)::float8 AS tokens_value,
    SUM(request_count)::float8 AS requests_value
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  GROUP BY bucket_at, group_key
)
SELECT
  buckets.bucket_at,
  COALESCE(groups.group_key, '')::text AS group_key,
  COALESCE(
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN '总量'
      WHEN 'apiKey' THEN api_key.name
      WHEN 'provider' THEN provider.name
      ELSE NULL
    END,
    COALESCE(groups.group_key, '')
  )::text AS group_label,
  COALESCE(metric_rows.tokens_value, 0)::float8 AS tokens_value,
  COALESCE(metric_rows.requests_value, 0)::float8 AS requests_value
FROM buckets
CROSS JOIN (SELECT group_key FROM groups UNION SELECT '' WHERE NOT EXISTS (SELECT 1 FROM groups)) groups
LEFT JOIN metric_rows ON metric_rows.bucket_at = buckets.bucket_at AND metric_rows.group_key = groups.group_key
LEFT JOIN api_key ON sqlc.arg('dimension')::text = 'apiKey'
  AND groups.group_key <> ''
  AND api_key.id = groups.group_key::int
LEFT JOIN provider ON sqlc.arg('dimension')::text = 'provider'
  AND groups.group_key <> ''
  AND provider.id = groups.group_key::int
ORDER BY buckets.bucket_at ASC, group_label ASC;

-- name: GetOverviewHourlyCostSeries :many
WITH buckets AS (
  SELECT generate_series(
    sqlc.arg('start_at')::timestamp,
    sqlc.arg('end_at')::timestamp - INTERVAL '1 hour',
    INTERVAL '1 hour'
  )::timestamp AS bucket_at
),
groups AS (
  SELECT DISTINCT
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN ''
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
    END AS group_key,
    upstream_cost_currency AS currency
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND upstream_cost IS NOT NULL
    AND upstream_cost_currency IS NOT NULL
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
),
metric_rows AS (
  SELECT
    bucket_at,
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN ''
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
    END AS group_key,
    upstream_cost_currency AS currency,
    SUM(upstream_cost)::float8 AS cost_value
  FROM request_overview_hourly
  WHERE bucket_at >= sqlc.arg('start_at')::timestamp
    AND bucket_at < sqlc.arg('end_at')::timestamp
    AND upstream_cost IS NOT NULL
    AND upstream_cost_currency IS NOT NULL
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  GROUP BY bucket_at, group_key, upstream_cost_currency
)
SELECT
  buckets.bucket_at,
  COALESCE(groups.group_key, '')::text AS group_key,
  COALESCE(
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN '总量'
      WHEN 'apiKey' THEN api_key.name
      WHEN 'provider' THEN provider.name
      ELSE NULL
    END,
    COALESCE(groups.group_key, '')
  )::text AS group_label,
  COALESCE(metric_rows.cost_value, 0)::float8 AS cost_value,
  COALESCE(groups.currency, '')::text AS currency
FROM buckets
CROSS JOIN (SELECT group_key, currency FROM groups UNION SELECT '', '' WHERE NOT EXISTS (SELECT 1 FROM groups)) groups
LEFT JOIN metric_rows ON metric_rows.bucket_at = buckets.bucket_at
  AND metric_rows.group_key = groups.group_key
  AND metric_rows.currency = groups.currency
LEFT JOIN api_key ON sqlc.arg('dimension')::text = 'apiKey'
  AND groups.group_key <> ''
  AND api_key.id = groups.group_key::int
LEFT JOIN provider ON sqlc.arg('dimension')::text = 'provider'
  AND groups.group_key <> ''
  AND provider.id = groups.group_key::int
ORDER BY buckets.bucket_at ASC, group_label ASC, COALESCE(groups.currency, '')::text ASC;

-- name: GetOverviewHourlyTraceSeries :many
WITH buckets AS (
  SELECT generate_series(
    sqlc.arg('start_at')::timestamp,
    sqlc.arg('end_at')::timestamp - INTERVAL '1 hour',
    INTERVAL '1 hour'
  )::timestamp AS bucket_at
),
groups AS (
  SELECT ''::text AS group_key
  WHERE sqlc.arg('dimension')::text = 'none'
  UNION
  SELECT DISTINCT
    CASE sqlc.arg('dimension')::text
      WHEN 'apiKey' THEN COALESCE(r.api_key_id::text, '')
      WHEN 'model' THEN COALESCE(r.model, '')
      WHEN 'upstreamModel' THEN COALESCE(r.upstream_model, '')
      WHEN 'provider' THEN COALESCE(r.provider_id::text, '')
    END AS group_key
  FROM traces t
  JOIN request r ON r.parent_span_id = t.parent_span_id
    AND r.created_at >= t.first_request_at
    AND r.created_at <= t.last_request_at
    AND r.type = 1
  WHERE sqlc.arg('dimension')::text <> 'none'
    AND t.last_request_at >= sqlc.arg('start_at')::timestamp
    AND t.last_request_at < sqlc.arg('end_at')::timestamp
    AND (sqlc.narg('api_key_id')::int IS NULL OR r.api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR r.model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR r.upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR r.provider_id = sqlc.narg('provider_id')::int)
),
metric_rows AS (
  SELECT
    time_bucket('1 hour', t.last_request_at)::timestamp AS bucket_at,
    ''::text AS group_key,
    COUNT(*)::float8 AS trace_value
  FROM traces t
  WHERE sqlc.arg('dimension')::text = 'none'
    AND t.last_request_at >= sqlc.arg('start_at')::timestamp
    AND t.last_request_at < sqlc.arg('end_at')::timestamp
    AND (
      (
        sqlc.narg('api_key_id')::int IS NULL
        AND sqlc.narg('model')::text IS NULL
        AND sqlc.narg('upstream_model')::text IS NULL
        AND sqlc.narg('provider_id')::int IS NULL
      )
      OR EXISTS (
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
      )
    )
  GROUP BY bucket_at
  UNION ALL
  SELECT
    time_bucket('1 hour', t.last_request_at)::timestamp AS bucket_at,
    groups.group_key,
    COUNT(DISTINCT t.id)::float8 AS trace_value
  FROM traces t
  JOIN LATERAL (
    SELECT DISTINCT
      CASE sqlc.arg('dimension')::text
        WHEN 'apiKey' THEN COALESCE(r.api_key_id::text, '')
        WHEN 'model' THEN COALESCE(r.model, '')
        WHEN 'upstreamModel' THEN COALESCE(r.upstream_model, '')
        WHEN 'provider' THEN COALESCE(r.provider_id::text, '')
      END AS group_key
    FROM request r
    WHERE r.parent_span_id = t.parent_span_id
      AND r.created_at >= t.first_request_at
      AND r.created_at <= t.last_request_at
      AND r.type = 1
      AND (sqlc.narg('api_key_id')::int IS NULL OR r.api_key_id = sqlc.narg('api_key_id')::int)
      AND (sqlc.narg('model')::text IS NULL OR r.model = sqlc.narg('model')::text)
      AND (sqlc.narg('upstream_model')::text IS NULL OR r.upstream_model = sqlc.narg('upstream_model')::text)
      AND (sqlc.narg('provider_id')::int IS NULL OR r.provider_id = sqlc.narg('provider_id')::int)
  ) groups ON true
  WHERE sqlc.arg('dimension')::text <> 'none'
    AND t.last_request_at >= sqlc.arg('start_at')::timestamp
    AND t.last_request_at < sqlc.arg('end_at')::timestamp
  GROUP BY bucket_at, groups.group_key
)
SELECT
  buckets.bucket_at,
  COALESCE(groups.group_key, '')::text AS group_key,
  COALESCE(
    CASE sqlc.arg('dimension')::text
      WHEN 'none' THEN '总量'
      WHEN 'apiKey' THEN api_key.name
      WHEN 'provider' THEN provider.name
      ELSE NULL
    END,
    COALESCE(groups.group_key, '')
  )::text AS group_label,
  COALESCE(metric_rows.trace_value, 0)::float8 AS trace_value
FROM buckets
CROSS JOIN (SELECT group_key FROM groups UNION SELECT '' WHERE NOT EXISTS (SELECT 1 FROM groups)) groups
LEFT JOIN metric_rows ON metric_rows.bucket_at = buckets.bucket_at AND metric_rows.group_key = groups.group_key
LEFT JOIN api_key ON sqlc.arg('dimension')::text = 'apiKey'
  AND groups.group_key <> ''
  AND api_key.id = groups.group_key::int
LEFT JOIN provider ON sqlc.arg('dimension')::text = 'provider'
  AND groups.group_key <> ''
  AND provider.id = groups.group_key::int
ORDER BY buckets.bucket_at ASC, group_label ASC;
