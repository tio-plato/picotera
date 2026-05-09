# Plan — Traces Table

## 1. Database Migration

1. Add `db/migrations/018_traces_table.sql`.
2. Create `traces` with `id TEXT PRIMARY KEY`, `parent_span_id TEXT NOT NULL UNIQUE`, `first_request_at TIMESTAMP NOT NULL`, `last_request_at TIMESTAMP NOT NULL`, and `updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`.
3. Add `traces_last_request_at_id_idx` on `(last_request_at DESC, id DESC)`.
4. Add a Go backfill path that scans existing non-empty `request.parent_span_id` values with `MIN(created_at)` and `MAX(created_at)`, assigns each missing trace an `xid.New().String()` id, and inserts it into `traces`.
5. Add a down migration that drops `traces`.

## 2. sqlc Queries

1. Add `ListTraceBackfillCandidates`.
2. Add `UpsertTrace` accepting an xid string candidate id and returning `id`, `parent_span_id`, `first_request_at`, and `last_request_at`.
3. Change `ListRequestTraces` so `traces` is the base relation.
4. Include `traces.id` and `traces.first_request_at` in the trace list row.
5. Keep metrics, token totals, costs, and user preview from bounded lateral scans over `request`.
6. Change trace cursor params from `cursor_parent_span_id` to `cursor_trace_id`.
7. Change `ListRequests` to accept `trace_id` and use the selected trace's `parent_span_id`, `first_request_at`, and `last_request_at` as the bounded request filter.
8. Remove the `parent_span_id` list filter from `ListRequests` after dashboard call sites use `traceId`.
9. Run `sqlc generate`.

## 3. Backend Write Path

1. Add a helper that calls `UpsertTrace` when `InsertRequestParams.ParentSpanID` is valid and non-empty.
2. Call that helper from `Server.insertRequest` after `InsertRequest` succeeds.
3. Generate the candidate trace id with `xid.New().String()` before calling `UpsertTrace`.
4. Use the persisted `created_at` returned by `InsertRequest` as the trace timestamp.
5. Log trace upsert failures without failing the gateway request.
6. Keep the path-based gateway and unified gateway unchanged at call sites because both already route through `insertRequest`.

## 4. Backend Contracts and Handlers

1. Add `ID string` and `FirstRequestAt string` to `RequestTraceView`.
2. Update `ToRequestTraceView` to map the new sqlc row fields.
3. Replace `ListRequestsRequest.ParentSpanID` with `TraceID string` using query name `traceId`.
4. Update `handleListRequests` to pass `TraceID` into `ListRequests`.
5. Update `handleListRequestTraces` cursor decoding and encoding to use `traceId`.
6. Validate `traceId` with strict xid parsing and return a bad request for invalid xid strings.
7. Return an empty request list for unknown trace ids through the SQL filter.

## 5. Dashboard

1. Regenerate OpenAPI types after backend changes.
2. Update `TracesView.vue` to display trace id and first request time, and to navigate with `{ name: 'requests', query: { traceId: String(row.id) } }`.
3. Update `RequestsView.vue` filters to read `traceId` from route query and send `traceId` to the API.
4. Remove the parent-span URL filter state from request-list navigation.
5. Keep parent span visible in request rows and trace rows for inspection.

## 6. Verification

1. Run `go test ./pkg/server ./pkg/llmbridge`.
2. Run migrations against the local TimescaleDB service.
3. Verify `traces` backfill creates one row per non-empty `request.parent_span_id`.
4. Verify historical backfill assigns xid string ids to traces.
5. Verify a new gateway request with `parent_span_id` inserts or updates one trace row.
6. Run `mise run openapi`.
7. Run `pnpm --dir dashboard generate-openapi`.
8. Run `pnpm --dir dashboard type-check`.
9. Run `pnpm --dir dashboard build`.
