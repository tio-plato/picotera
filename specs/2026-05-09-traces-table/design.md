# Design — Traces Table

## Scope

This change materializes trace identity into a new `traces` table. The table is keyed by an internal xid string, keeps `parent_span_id` unique, and records the earliest and latest `request.created_at` values observed for that parent span. Request history can then list traces from the compact trace index, and request lookups by trace can use the stored time bounds to query the TimescaleDB `request` hypertable with chunk pruning.

## Database Design

Add a new migration `018_traces_table.sql`:

```sql
CREATE TABLE traces (
  id TEXT PRIMARY KEY,
  parent_span_id TEXT NOT NULL UNIQUE,
  first_request_at TIMESTAMP NOT NULL,
  last_request_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX traces_last_request_at_id_idx ON traces (last_request_at DESC, id DESC);
```

The database migration creates the table and indexes. It does not generate trace ids for historical request rows because trace ids must use the same xid format as the rest of the application.

Historical trace backfill runs in Go after migrations. The backfill scans existing non-empty parent spans and writes one xid-backed trace row for each missing parent span:

```sql
SELECT parent_span_id, MIN(created_at) AS first_request_at, MAX(created_at) AS last_request_at
FROM request
WHERE parent_span_id IS NOT NULL AND parent_span_id <> ''
  AND NOT EXISTS (
    SELECT 1
    FROM traces
    WHERE traces.parent_span_id = request.parent_span_id
  )
GROUP BY parent_span_id;
```

The Go backfill assigns `xid.New().String()` to `traces.id` for each returned row and inserts it with the scanned bounds. It is idempotent because `parent_span_id` is unique and the query only returns missing parent spans.

Runtime code only upserts a trace when the request row has a non-empty parent span id. The code does not trim, case-fold, or normalize parent span ids.

Add sqlc queries in `db/queries/trace.sql`:

```sql
-- name: ListTraceBackfillCandidates :many
SELECT parent_span_id, MIN(created_at), MAX(created_at)
FROM request
WHERE parent_span_id IS NOT NULL AND parent_span_id <> ''
  AND NOT EXISTS (
    SELECT 1
    FROM traces
    WHERE traces.parent_span_id = request.parent_span_id
  )
GROUP BY parent_span_id
ORDER BY MIN(created_at), parent_span_id;

-- name: UpsertTrace :one
INSERT INTO traces (id, parent_span_id, first_request_at, last_request_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (parent_span_id) DO UPDATE
SET first_request_at = LEAST(traces.first_request_at, EXCLUDED.first_request_at),
    last_request_at = GREATEST(traces.last_request_at, EXCLUDED.last_request_at),
    updated_at = CURRENT_TIMESTAMP
RETURNING id, parent_span_id, first_request_at, last_request_at;
```

The `request` table keeps `parent_span_id` as the copied external span id. No compatibility column or duplicate legacy table is introduced.

## Write Path

`Server.insertRequest` remains the single request insert helper used by the path-based gateway and unified generation routes. After `InsertRequest` succeeds and returns the persisted `created_at`, `insertRequest` calls `UpsertTrace` when `arg.ParentSpanID.Valid` is true and `arg.ParentSpanID.String` is not empty.

The runtime upsert passes `xid.New().String()` as the candidate trace id. PostgreSQL uses it only when the parent span is first seen; later conflicts on `parent_span_id` update the bounds and keep the existing trace id.

Trace upsert errors are logged and do not fail the gateway request. The request row remains the primary audit record, and the trace table can be repaired by rerunning the Go backfill if needed.

The upsert uses the actual inserted timestamp returned by `InsertRequest`, not the caller-provided timestamp. This keeps trace bounds aligned with the row stored in the hypertable.

## Query Design

`ListRequestTraces` changes from grouping all rows in `request` to scanning `traces` as the pagination base. It returns `traces.id`, `traces.parent_span_id`, `traces.first_request_at`, and `traces.last_request_at`, then uses lateral bounded scans over `request` for metrics, costs, and preview:

```sql
WHERE request.parent_span_id = traces.parent_span_id
  AND request.created_at >= traces.first_request_at
  AND request.created_at <= traces.last_request_at
```

The list remains ordered by newest trace activity:

```sql
ORDER BY traces.last_request_at DESC, traces.id DESC
```

The cursor stores `lastRequestAt` and the internal `traceId`. This avoids using `parent_span_id` as a pagination tiebreaker and gives a stable internal trace handle to the API and dashboard.

`ListRequests` gains `trace_id` as the preferred trace filter. When `trace_id` is present, SQL resolves the trace row first and filters requests by both `parent_span_id` and the stored created-at range. This gives TimescaleDB a bounded `created_at` predicate for trace request lookup:

```sql
WITH selected_trace AS (
  SELECT parent_span_id, first_request_at, last_request_at
  FROM traces
  WHERE id = sqlc.narg('trace_id')::text
)
SELECT ...
FROM request
WHERE ...
  AND (
    sqlc.narg('trace_id')::text IS NULL
    OR (
      parent_span_id = (SELECT parent_span_id FROM selected_trace)
      AND created_at >= (SELECT first_request_at FROM selected_trace)
      AND created_at <= (SELECT last_request_at FROM selected_trace)
    )
  )
```

The existing `parentSpanId` query parameter is replaced by trace id in dashboard navigation. Backend code can remove the old parent-span list filter because this plan updates all call sites to use the internal xid trace id.

## API Design

The trace list response adds:

- `id`: internal xid trace id.
- `firstRequestAt`: earliest request timestamp for this parent span.
- `lastRequestAt`: latest request timestamp for this parent span.

Trace rows keep `parentSpanId` for inspection and display.

The request list endpoint accepts `traceId` as the trace lookup parameter. The dashboard trace page links to `/requests?traceId=<id>`, and the request page displays the xid trace id filter state while showing the human-readable parent span from the returned rows.

Cursor shape for `GET /api/picotera/request-traces` becomes:

- `lastRequestAt`
- `traceId`

## Dashboard Design

`TracesView.vue` uses `row.id` for row actions and URL navigation. It still displays `parentSpanId`, first/last request time, request counts, token totals, costs, and preview.

`RequestsView.vue` reads and writes `traceId` in the route query. When the trace filter is active, API calls send `traceId` and omit the previous `parentSpanId` filter. The page keeps exact filtering: invalid xid strings are not normalized client-side; the API returns the validation error.

## Generated Code

After implementation, run:

```bash
sqlc generate
mise run openapi
pnpm --dir dashboard generate-openapi
```

No third-party library is added.
