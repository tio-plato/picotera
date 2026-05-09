# Plan

## 1. Add backend contract

1. Create `pkg/contract/overview.go`.
2. Define `OverviewRange`, `OverviewDimension`, `OverviewSeriesDimension`, `OverviewCostView`, `OverviewSummaryView`, `OverviewDistributionRowView`, `OverviewSeriesRowView`, `GetOverviewRequest`, and `GetOverviewResponse`.
3. Add `OperationGetOverview` with method `GET`, path `/overview`, operation id `getOverview`, and summary `Get dashboard overview`.
4. Implement strict enum validation helpers for `range`, `distributionDimension`, and `seriesDimension`.
5. Reject present-but-empty `model` and `upstreamModel` query parameters.

## 2. Add TimescaleDB overview aggregate

1. Create migration `db/migrations/019_request_overview_hourly_cagg.sql`.
2. Create continuous aggregate `request_overview_hourly` using `time_bucket('1 hour', created_at)`.
3. Group the aggregate by `api_key_id`, `model`, `upstream_model`, `provider_id`, and `upstream_cost_currency`.
4. Store `request_count`, total token fields, and `upstream_cost` sums in the aggregate.
5. Set `timescaledb.materialized_only = false` on the materialized view.
6. Add a continuous aggregate refresh policy with `start_offset => INTERVAL '35 days'`, `end_offset => INTERVAL '5 minutes'`, and `schedule_interval => INTERVAL '5 minutes'`.
7. Add a down migration that removes the continuous aggregate policy and drops the materialized view.

## 3. Add sqlc overview queries

1. Create `db/queries/overview.sql`.
2. Add a summary query over `request_overview_hourly` with exact filters and a half-open time range `bucket_at >= start_at AND bucket_at < end_at`.
3. Add a cost summary query over `request_overview_hourly` grouped by `upstream_cost_currency`.
4. Add a trace count query over `traces.last_request_at` with filter-aware `EXISTS` matching on `parent_span_id`.
5. Add one CASE-driven distribution query over `request_overview_hourly` with validated dimension input.
6. Add hourly series queries for tokens, cost, and requests over `request_overview_hourly`.
7. Add hourly trace series queries over `traces` and raw `request` membership only for grouped trace attribution.
8. Use `generate_series(start_at, end_at - interval '1 hour', interval '1 hour')` to produce stable hourly buckets and left join metric rows.
9. Run `sqlc generate`.

## 4. Add overview handler

1. Create `pkg/server/handle_overview.go`.
2. Convert the requested range to UTC `startAt` and `endAt`.
3. Build nullable pgtype filter params without trimming or coercion.
4. Call sqlc queries and merge their rows into the response shape.
5. Format all timestamps as RFC3339Nano UTC strings.
6. Register `OperationGetOverview` in `registerOperations()`.
7. Add focused handler/unit tests for enum validation, range calculation, and empty-string rejection.

## 5. Regenerate API types

1. Run `mise run openapi`.
2. Run `pnpm --dir dashboard generate-openapi`.
3. Confirm `openapi.yaml` and `dashboard/src/openapi-types.d.ts` include `getOverview`.

## 6. Add Unovis dependencies

1. Run `pnpm --dir dashboard add @unovis/vue@1.6.5 @unovis/ts@1.6.5`.
2. Import Unovis CSS in `dashboard/src/main.ts`.
3. Keep all chart UI wrapped in local dashboard components and semantic tokens.

## 7. Add dashboard data client

1. Add `OverviewFilters` to `dashboard/src/api/queryKeys.ts`.
2. Add `queryKeys.overview.detail(filters)`.
3. Add `getOverview(filters)` to `dashboard/src/api/client.ts`.
4. Export any generated overview types needed by the view from `dashboard/src/api/index.ts`.

## 8. Add overview route and navigation

1. Add route `{ path: '/overview', name: 'overview', component: () => import('@/views/OverviewView.vue') }`.
2. Change the root redirect from `/providers` to `/overview`.
3. Add `overview` metadata to `App.vue` with title `概览` and a concise hint.
4. Add a sidebar nav item labeled `概览` with a newly registered `chart-pie` Tabler icon.

## 9. Build `OverviewView.vue`

1. Create `dashboard/src/views/OverviewView.vue`.
2. Add a top control row with a range `SegmentedControl`.
3. Add strict exact-match filters for API key, actual model, upstream model, and provider using reference data from existing list queries.
4. Query overview data with Vue Query, `OPERATIONAL_STALE_TIME`, and the full filter object in the query key.
5. Render four bento summary boxes: total token count, total request count, total cost, and total trace count.
6. Render a two-column distribution section with token pie chart on the left and cost pie chart on the right.
7. Add a distribution dimension selector shared by both pie charts.
8. Render a two-column hourly chart section with four stacked area charts: tokens, cost, requests, and traces.
9. Add a series aggregation selector with options `不聚合`, `API Key`, `实际模型`, `上游模型`, and `渠道`.
10. Show `StateText` for loading, error, and empty chart states.
11. Use existing number, money, and timestamp formatting patterns from request and trace views.

## 10. Verify

1. Run `docker compose up -d` so TimescaleDB features are available.
2. Run `go test ./pkg/server ./pkg/llmbridge`.
3. Run `go build -o picotera ./cmd/picotera`.
4. Run `pnpm --dir dashboard type-check`.
5. Run `pnpm --dir dashboard lint`.
6. Run `pnpm --dir dashboard build`.
7. Smoke-test `/overview` with no filters, each range, each filter, each distribution dimension, and each series aggregation dimension.
8. Verify `request_overview_hourly` is used by overview queries and contains real-time rows newer than the last materialized refresh.
