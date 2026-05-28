# Plan: Decode Speed Box Plot

## Step 1: SQL Query

Add `GetOverviewSpeedBoxplot` to `db/queries/overview.sql`:

```sql
-- name: GetOverviewSpeedBoxplot :many
WITH speeds AS (
  SELECT
    CASE sqlc.arg('dimension')::text
      WHEN 'model' THEN COALESCE(model, '')
      WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
      WHEN 'provider' THEN COALESCE(provider_id::text, '')
      WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
      WHEN 'project' THEN COALESCE(project_id::text, '')
      ELSE ''
    END AS group_key,
    output_tokens::float8 / ((time_spent_ms - ttft_ms)::float8 / 1000.0) AS decode_speed
  FROM request
  WHERE type = 1
    AND status = 2
    AND created_at >= sqlc.arg('start_at')::timestamp
    AND created_at < sqlc.arg('end_at')::timestamp
    AND output_tokens >= 50
    AND ttft_ms IS NOT NULL
    AND time_spent_ms IS NOT NULL
    AND (time_spent_ms - ttft_ms) >= 500
    AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
    AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
    AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
    AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
    AND (sqlc.narg('project_id')::int IS NULL OR project_id = sqlc.narg('project_id')::int)
)
SELECT
  group_key,
  MIN(decode_speed)::float8 AS min_speed,
  percentile_cont(0.25) WITHIN GROUP (ORDER BY decode_speed)::float8 AS p25_speed,
  percentile_cont(0.5) WITHIN GROUP (ORDER BY decode_speed)::float8 AS median_speed,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY decode_speed)::float8 AS p95_speed,
  MAX(decode_speed)::float8 AS max_speed,
  COUNT(*)::bigint AS request_count
FROM speeds
GROUP BY group_key
ORDER BY group_key ASC;
```

Then run `sqlc generate` to regenerate `pkg/db/`.

## Step 2: Contract Types & Operation

Add to `pkg/contract/overview.go`:

- `OverviewSpeedBoxplotItemView` struct: `Key`, `Label`, `Min`, `P25`, `Median`, `P95`, `Max`, `Count`
- `OverviewSpeedBoxplotView` struct: `Window`, `Dimension`, `Items`
- `GetOverviewSpeedBoxplotRequest` struct: embeds `OverviewCommonRequest` + `Dimension` field (same enum as series)
- `GetOverviewSpeedBoxplotResponse` struct: `Body OverviewSpeedBoxplotView`
- `OperationGetOverviewSpeedBoxplot` variable: `GET /overview/speed-boxplot`

## Step 3: Handler

Add `handleGetOverviewSpeedBoxplot` to `pkg/server/handle_overview.go`:

1. Parse time window via `overviewWindow()`
2. Call `s.queries.GetOverviewSpeedBoxplot()` with dimension + filters
3. Map rows to `OverviewSpeedBoxplotItemView` slice
4. Return response with window metadata

## Step 4: Register Operation

Add `huma.Register(mgmt, contract.OperationGetOverviewSpeedBoxplot, s.handleGetOverviewSpeedBoxplot)` in `registerOperations()` in `pkg/server/server.go`.

## Step 5: Regenerate OpenAPI & TypeScript Types

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

## Step 6: Dashboard API Client

In `dashboard/src/api/client.ts`:
- Add `getOverviewSpeedBoxplot(filters, dimension)` fetcher function
- Add `queryKeys.overview.speedBoxplot(filters, dimension)` key

## Step 7: Dashboard Component

Update `OverviewSpeedTimeline.vue` to consume the new endpoint instead of re-using the speed series data. Replace the current pseudo-boxplot (min/max only) with a proper boxplot showing P25/P95 box and median line.

Alternatively, create a new component if the props interface changes significantly.

Update `OverviewView.vue`:
- Add a `useQuery` for the boxplot endpoint, keyed by `speedDimension` and `overviewFilters`
- Pass the boxplot data to the updated timeline component
- Remove the current `seriesDecodeSpeed` → `OverviewSpeedTimeline` binding
