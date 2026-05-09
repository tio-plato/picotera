# Request TimescaleDB Partitioning Design

## Scope

This change converts the gateway request history table from a plain PostgreSQL table into a TimescaleDB hypertable partitioned by `created_at`. The request ID remains the externally visible identifier, and `created_at` becomes part of the internal database identity so TimescaleDB can enforce uniqueness on the partitioning dimension.

The implementation keeps the public request ID shape unchanged. Go derives the request timestamp from the generated xid and writes it explicitly as `created_at` during insert. Management APIs expose strict time-window inputs so every request-history query carries a `created_at` predicate that TimescaleDB can use for chunk pruning.

## Database Design

The `request` table primary key becomes:

```sql
PRIMARY KEY (id, created_at)
```

The table is converted with:

```sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
SELECT create_hypertable('request', by_range('created_at'), migrate_data => true, if_not_exists => true);
```

The existing `request.id` column stays as text and continues to store xid strings. The schema does not add a surrogate integer key and does not rename public API fields.

Additional indexes support the current access paths while preserving the partition key:

```sql
CREATE INDEX request_created_at_id_idx ON request (created_at DESC, id DESC);
CREATE INDEX request_parent_span_created_at_idx ON request (parent_span_id, created_at DESC, id DESC)
  WHERE parent_span_id IS NOT NULL AND parent_span_id <> '';
CREATE INDEX request_span_created_at_idx ON request (span_id, created_at ASC, id ASC)
  WHERE span_id IS NOT NULL;
```

TimescaleDB is already the development PostgreSQL image in `docker-compose.yaml`, so the migration only needs to create the extension. Production environments must run a PostgreSQL 17-compatible TimescaleDB image or have the TimescaleDB extension installed before applying this migration.

## Pre-Migration Data Rewrite

Historical gateway rows may have `created_at` values from the database default rather than the timestamp encoded in the xid. Before applying the hypertable migration, run:

```bash
psql "$PICOTERA_DATABASE_URL" \
  -f specs/2026-05-08-request-timescaledb-partition/rewrite_request_created_at_from_xid.sql
```

The script validates every `request.id` as a strict lowercase xid-shaped value and rewrites `request.created_at` to the UTC second encoded in the ID. It runs in a single transaction and aborts before writing when it finds an invalid ID. After this rewrite, point lookups and updates use exact `(id, created_at)` predicates.

## Timestamp Generation

Request IDs are generated with `github.com/rs/xid`. `xid.ID.Time()` returns the timestamp encoded in the first four bytes of the ID at one-second precision. The server will introduce a helper that returns both values from one generated xid:

```go
id := xid.New()
requestID := id.String()
createdAt := id.Time().UTC()
```

`InsertRequest` will accept `created_at` as a required parameter and return the inserted timestamp for the existing artifact-key flow. The migration removes the database default from `request.created_at` so every insert supplies the partition key explicitly. This keeps `(id, created_at)` stable for later updates and detail queries.

Strict xid parsing is required when an API receives only `id` and needs a partition predicate. Invalid xid strings are rejected as bad requests. The server does not trim, case-fold, or repair request IDs.

## Query Design

All request-history SQL in `db/queries/request.sql` will take explicit `created_at` bounds.

List endpoints use a required range:

```sql
created_at >= @created_at_from AND created_at < @created_at_to
```

Cursor pagination stays ordered by `(created_at DESC, id DESC)` and remains stable inside the requested range.

Point lookups use the timestamp decoded from the xid:

```sql
WHERE id = @id AND created_at = @id_created_at
```

The one-time pre-migration rewrite removes historical subsecond drift, and new gateway inserts write the exact second from the xid.

Update queries become partition-aware:

```sql
UPDATE request
SET ...
WHERE id = @id AND created_at = @created_at
```

The gateway already holds the created timestamp returned from insert for artifact keys. That timestamp will be carried alongside the ID for every subsequent metadata and completion update.

Trace aggregation queries receive the same required time range and apply it to every base and lateral scan of `request`. Span expansion first resolves the anchor request with the decoded ID time, then returns rows with the same span inside the caller's requested time range.

## API Design

The management request-history API gains time-window query parameters. There is no dashboard-only shortcut or compatibility path.

The default dashboard behavior supplies a bounded last-24-hours window and lets the user adjust it from the request-history controls. Direct API callers must provide the required parameters for list and trace endpoints.

Point detail endpoints keep the path shape `/requests/{id}` and `/requests/{id}/spans`. The server derives the exact point lookup partition timestamp from the xid path parameter so user requests do not need to include `created_at`.

## Generated Code

After SQL changes, run:

```bash
sqlc generate
mise run openapi
pnpm --dir dashboard generate-openapi
```

The dashboard typed client must be regenerated because request-history query parameters change.

## Testing Strategy

Backend tests will cover:

- xid helper generation and strict parsing.
- request detail lookup parameter construction from an xid.
- list and trace handlers requiring valid time ranges.
- update helper methods including `created_at` in sqlc params.

Database verification will run migrations against the TimescaleDB docker service and inspect:

- `timescaledb_information.hypertables` contains `request`.
- `request` has primary key `(id, created_at)`.
- `EXPLAIN` for list/detail/trace queries shows chunk pruning when bounded by `created_at`.
