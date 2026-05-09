# Request TimescaleDB Partitioning Plan

1. Add request ID timestamp helpers.
   - Create a small helper in `pkg/server` that generates a request ID and its `created_at` timestamp from a single `xid.New()` value.
   - Create a strict parser that converts a request ID string to the xid timestamp and returns a typed bad-request error for invalid IDs.
   - Add unit tests for generation, parsing, invalid IDs, and UTC normalization.

2. Rewrite historical `created_at` values before the schema migration.
   - Keep `rewrite_request_created_at_from_xid.sql` in this spec directory as a manual one-time script.
   - Run the script with `psql "$PICOTERA_DATABASE_URL" -f specs/2026-05-08-request-timescaledb-partition/rewrite_request_created_at_from_xid.sql`.
   - Validate every existing `request.id` with strict lowercase xid-shaped checks.
   - Update every historical `request.created_at` to the UTC second encoded in the xid.
   - Abort the transaction before writing if invalid IDs are present.

3. Migrate the `request` table to TimescaleDB.
   - Add a new goose migration after `016_request_cache_write_1h_tokens.sql`.
   - Run `CREATE EXTENSION IF NOT EXISTS timescaledb`.
   - Drop the existing `request` primary key on `id`.
   - Add `PRIMARY KEY (id, created_at)`.
   - Drop the database default from `request.created_at`.
   - Convert the table to a hypertable partitioned by `created_at` with `migrate_data => true`.
   - Add indexes for list ordering, parent trace grouping, and span expansion.
   - Implement the down migration by creating a plain replacement table, copying rows from the hypertable, dropping the hypertable, renaming the replacement table to `request`, restoring `PRIMARY KEY (id)`, and restoring the `created_at DEFAULT CURRENT_TIMESTAMP`.

4. Make inserts write `created_at` explicitly.
   - Update `db/queries/routing.sql` `InsertRequest` to include `created_at`.
   - Regenerate `pkg/db`.
   - Update all gateway meta and upstream insert call sites in `handle_gateway.go` and `handle_unified_gateway.go` to use the ID helper.
   - Keep artifact keys based on the same timestamp used for the inserted row.

5. Make request updates partition-aware.
   - Update `UpdateRequestOnHeader`, `UpdateRequestOnComplete`, `UpdateRequestModel`, and `UpdateRequestMetrics` to require `created_at`.
   - Regenerate `pkg/db`.
   - Thread `metaCreatedAt` and `upstreamCreatedAt` through all update helper calls in path-based and unified gateway handlers.
   - Update helper method signatures only where the generated sqlc params require it.

6. Add required time windows to list and trace contracts.
   - Update `pkg/contract/request.go` with `createdAtFrom` and `createdAtTo` query parameters for `ListRequestsRequest` and `ListRequestTracesRequest`.
   - Add optional paired range parameters to `ListRequestSpansRequest`.
   - Keep `GetRequestRequest` unchanged except for stricter xid validation in the handler.
   - Regenerate `openapi.yaml` and dashboard OpenAPI types after backend changes.

7. Rewrite request SQL for chunk pruning.
   - Add `created_at >= @created_at_from AND created_at < @created_at_to` to `ListRequests`.
   - Add the same range to the trace base CTE and every lateral request-table scan in `ListRequestTraces`.
   - Change `GetRequest` to use `id = @id AND created_at = @id_created_at`.
   - Change `ListRequestsBySpan` so the anchor lookup uses the xid-derived timestamp and returned rows use the requested or default span time range.
   - Preserve existing ordering and cursor semantics.

8. Update request handlers.
   - Parse and validate required list/trace time windows using strict RFC3339/RFC3339Nano parsing.
   - Decode xid timestamps for `GET /requests/{id}` and anchor span lookup.
   - Apply the fixed 24-hour lookback/lookahead default only for `/requests/{id}/spans` when callers omit the range.
   - Return `400 Bad Request` for invalid IDs, invalid timestamps, missing required windows, and non-increasing ranges.

9. Update the dashboard request-history views.
   - Read `dashboard/DESIGN_SYSTEM.md` before editing UI.
   - Add request-history time-window state with a bounded default.
   - Pass `createdAtFrom` and `createdAtTo` to request list and trace API calls.
   - Preserve existing filters and cursor pagination inside the selected window.
   - Add controls that expose the active window without changing unrelated management screens.

10. Verify end to end.
   - Run `sqlc generate`.
   - Run `go test ./pkg/server/... ./pkg/llmbridge/...`.
   - Run `mise run openapi`.
   - Run `pnpm --dir dashboard generate-openapi`.
   - Run `pnpm --dir dashboard type-check`.
   - Run the one-time rewrite script against a copy of existing data and confirm the reported rewritten row count.
   - Run migrations against the TimescaleDB docker service.
   - Confirm `request` is listed as a hypertable and `EXPLAIN` uses the created-at predicates for chunk pruning.
