# Design

## Overview

Add a dashboard overview page backed by a new read-only aggregation API. The backend reads request, token, and cost metrics from a TimescaleDB continuous aggregate over the `request` hypertable, and reads trace counts from the indexed `traces` table. The dashboard renders summary metrics and charts with Unovis.

The page is mounted as `overview` at `/overview`, and the root route redirects to `/overview`. The sidebar gets a new “概览” navigation item above the existing configuration pages.

## Data Model

The overview reads from these database objects:

- `request` provides upstream request volume, token totals, cost, API key id, actual model, upstream model, provider id, endpoint path, and hourly timestamps.
- `traces` provides trace counts filtered by trace activity time.
- `request_overview_hourly` stores hourly pre-aggregated upstream request metrics by API key, actual model, upstream model, provider, and upstream cost currency.
- `api_key`, `provider`, and `model` provide labels for filters and chart legends through existing list APIs.

Add migration `019_request_overview_hourly_cagg.sql` to create the continuous aggregate and its refresh policy. The implementation keeps the existing `request_created_at_id_idx` and `traces_last_request_at_id_idx` range indexes for raw lookups and trace counting.

## TimescaleDB Aggregation

Create a continuous aggregate:

```sql
CREATE MATERIALIZED VIEW request_overview_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket('1 hour', created_at) AS bucket_at,
  api_key_id,
  model,
  upstream_model,
  provider_id,
  upstream_cost_currency,
  COUNT(*)::bigint AS request_count,
  SUM(
    COALESCE(input_tokens, 0)
    + COALESCE(cache_read_tokens, 0)
    + COALESCE(output_tokens, 0)
    + COALESCE(cache_write_tokens, 0)
    + COALESCE(cache_write_1h_tokens, 0)
  )::bigint AS total_tokens,
  SUM(COALESCE(input_tokens, 0))::bigint AS input_tokens,
  SUM(COALESCE(cache_read_tokens, 0))::bigint AS cache_read_tokens,
  SUM(COALESCE(output_tokens, 0))::bigint AS output_tokens,
  SUM(COALESCE(cache_write_tokens, 0))::bigint AS cache_write_tokens,
  SUM(COALESCE(cache_write_1h_tokens, 0))::bigint AS cache_write_1h_tokens,
  SUM(upstream_cost)::numeric(20, 6) AS upstream_cost
FROM request
WHERE type = 1
GROUP BY bucket_at, api_key_id, model, upstream_model, provider_id, upstream_cost_currency
WITH NO DATA;
```

Set real-time aggregation on the continuous aggregate so the latest unmaterialized rows are included:

```sql
ALTER MATERIALIZED VIEW request_overview_hourly
SET (timescaledb.materialized_only = false);
```

Add an automatic refresh policy:

```sql
SELECT add_continuous_aggregate_policy(
  'request_overview_hourly',
  start_offset => INTERVAL '35 days',
  end_offset => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);
```

The overview's maximum `1m` range is covered by the 35-day refresh window. Overview SQL reads request, token, request-count, and cost data from `request_overview_hourly`, not directly from `request`. The raw `request` table is used in overview queries only for filter-aware trace attribution because that operation needs exact parent-span membership.

## Time Range

The UI exposes `24h`, `1d`, `7d`, and `1m`. `24h` and `1d` both cover the last 24 hours because the user requested both labels. The backend accepts these exact enum values and rejects any other value.

Aggregation buckets are hourly for every range. The API returns UTC bucket timestamps. The dashboard formats bucket labels in the browser's local timezone.

## Filters

The API supports exact filters:

- `apiKeyId`: exact `request.api_key_id`.
- `model`: exact `request.model`; this is the actual client-facing model used by the gateway.
- `upstreamModel`: exact `request.upstream_model`.
- `providerId`: exact `request.provider_id`; the UI labels this as 渠道.

Invalid enum values, malformed integers, and empty string values are rejected by Huma validation and handler validation. The backend does not trim, case-fold, coerce empty strings, or accept near-miss values.

## Aggregation Semantics

Request metrics use upstream request rows only: `request.type = 1`. This is enforced in `request_overview_hourly` so overview queries do not repeat the raw-row filter.

Total tokens are computed as:

```sql
COALESCE(input_tokens, 0)
+ COALESCE(cache_read_tokens, 0)
+ COALESCE(output_tokens, 0)
+ COALESCE(cache_write_tokens, 0)
+ COALESCE(cache_write_1h_tokens, 0)
```

Total requests sum `request_count` over matching hourly aggregate rows.

Total costs sum `upstream_cost` grouped by `upstream_cost_currency`, because this represents actual upstream spend. Costs are returned grouped by currency; the frontend displays native currency totals and uses the existing exchange-rate composable only for display conversion where the dashboard already supports it.

Total traces count rows from `traces` whose `last_request_at` is inside the selected range. Request filters are applied to traces by `EXISTS` over matching upstream request rows sharing the trace row's `parent_span_id` and falling inside the trace window.

## Distribution Charts

The API returns distribution rows for the four supported dimensions:

- `apiKey`
- `model`
- `upstreamModel`
- `provider`

Each row contains the dimension key, label fields where available, total tokens, request count, trace count, and cost totals by currency. The dashboard uses the same selected distribution dimension for the token pie chart and cost pie chart. The default dimension is `provider`.

For null dimension values, the API returns an empty key and the dashboard displays `未设置`.

## Hourly Series

The API returns hourly bucket rows for these metrics:

- `tokens`
- `cost`
- `requests`
- `traces`

The UI shows a two-column chart grid. Each chart uses the same aggregation dimension selector with options:

- `none`
- `apiKey`
- `model`
- `upstreamModel`
- `provider`

`none` returns a single series. Other dimensions return stacked series. The API joins `request_overview_hourly` against generated hourly buckets so chart axes remain stable even when no requests match a bucket.

Trace hourly series counts traces by `traces.last_request_at` bucket. When a dimension grouping is selected, trace counts are attributed through matching upstream request rows in the same trace. If one trace has matching requests in more than one group, it contributes once to each group. The ungrouped trace chart counts each trace once.

## API Shape

Use a single `GET /api/picotera/overview` operation. It returns summary metrics, all distribution data, and the hourly series for the requested range and filters in one response. This avoids multiple coordinated dashboard requests for one screen and keeps aggregation semantics centralized.

## Dashboard Architecture

Add `dashboard/src/views/OverviewView.vue`.

The view uses Vue Query with a filter-keyed query:

- `queryKeys.overview.detail(filters)`
- `listOverview(filters)` in `dashboard/src/api/client.ts`
- `OPERATIONAL_STALE_TIME` because this page reflects live traffic

The filter controls reuse local UI primitives:

- `SegmentedControl` for range.
- `Select` controls for API key, model, upstream model, provider, distribution dimension, and series aggregation dimension.
- `DataCard`, `StateText`, `MoneyDisplay`, and existing typography tokens for bento metrics and chart containers.

## Unovis

Add `@unovis/vue` and its peer dependency `@unovis/ts` to `dashboard/package.json` using pnpm. Use Unovis Vue components for:

- donut charts for token and cost distributions.
- stacked area charts for hourly token, cost, request, and trace series.

Use dashboard semantic tokens through CSS variables for chart colors and labels. Do not introduce a third-party UI kit.

## OpenAPI Workflow

After adding the backend contract:

1. Run `mise run openapi` to regenerate `openapi.yaml`.
2. Run `pnpm --dir dashboard generate-openapi` to regenerate `dashboard/src/openapi-types.d.ts`.

The dashboard uses the generated types through the existing `dashboard/src/api/index.ts` exports.
