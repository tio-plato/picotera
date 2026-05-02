# Plan: Add `upstream_model` to Request Log

## Step 1 — Database migration

Create `db/migrations/004_upstream_model.sql`:
```sql
ALTER TABLE request ADD COLUMN upstream_model TEXT;
```

## Step 2 — Update sqlc queries

**`db/queries/routing.sql`** — `InsertRequest`:
- Add `upstream_model` to the column list (after `model`) and add `$18` as the value placeholder.

**`db/queries/request.sql`**:
- `ListRequests`: add `upstream_model` to SELECT list; add optional `upstream_model` filter with `sqlc.narg`.
- `ListRequestsBySpan`: add `r.upstream_model` to SELECT list.
- `UpdateRequestOnHeader`: add `upstream_model = $7` to SET clause.

## Step 3 — Regenerate sqlc code

Run `sqlc generate`. This updates `pkg/db/models.go`, `request.sql.go`, `routing.sql.go`, and `querier.go` with the new `UpstreamModel pgtype.Text` field and updated param structs.

## Step 4 — Update API contract (`pkg/contract/request.go`)

- Add `UpstreamModel string \`json:"upstreamModel,omitempty"\`` to `RequestView`.
- Add `UpstreamModel pgtype.Text` to `requestLike`.
- Map the field in `toRequestView`, `ToRequestView`, `ToListRequestRowView`, `ToListRequestsBySpanRowView`.
- Add `UpstreamModel string \`query:"upstreamModel,omitempty"\`` to `ListRequestsRequest`.

## Step 5 — Update gateway handler (`pkg/server/handle_gateway.go`)

**5a. Capture original model name before `rewriteModel` hook.**

Before the `rewriteModel` hook call (currently ~line 170), save the original:
```go
originalModelName := modelName
```
After the hook, if `newModel != modelName`, update `body` and `modelName` as today, but keep `originalModelName` untouched.

**5b. Use `originalModelName` for the `model` column in all request rows.**

- **Meta request insert**: `Model: pgtype.Text{String: originalModelName, Valid: originalModelName != ""}`.
- **Upstream request insert**: `Model: pgtype.Text{String: originalModelName, Valid: originalModelName != ""}`.
- **`UpdateRequestOnHeader` for both rows**: `Model: pgtype.Text{String: originalModelName, Valid: originalModelName != ""}`.

**5c. Populate `upstream_model` column.**

- **Upstream request insert**: `UpstreamModel: pgtype.Text{String: upstreamModel, Valid: upstreamModel != ""}` — use the `upstreamModel` value from the existing 3-tier fallback chain (`dec.UpstreamModel` → MPE → `modelName`).
- **`streamSuccess` → `UpdateRequestOnHeader` for meta row**: Add `UpstreamModel: pgtype.Text{String: upstreamModel, Valid: upstreamModel != ""}`. Pass `upstreamModel` as a new parameter to `streamSuccess`.
- **`streamSuccess` → `UpdateRequestOnHeader` for upstream row**: Same `UpstreamModel` value.
- **Meta request insert** (initial): `UpstreamModel: pgtype.Text{Valid: false}` (not known yet).

## Step 6 — Update request API handler (`pkg/server/handle_requests.go`)

- In `handleListRequests`: add `UpstreamModel` filter param mapping (same pattern as `Model`).

## Step 7 — Regenerate OpenAPI spec

Run `mise run openapi` to update `openapi.yaml` with the new `upstreamModel` field and filter.

## Step 8 — Update dashboard types and UI

- Run `openapi-typescript` to regenerate `dashboard/src/api.d.ts` (or let `pnpm --dir dashboard build` catch it).
- **`dashboard/src/views/RequestsView.vue`**: Add `upstreamModel` column to the table (after the `model` column), showing the upstream model when present.

## Step 9 — Verify

- `go build ./cmd/picotera` — compiles cleanly.
- `pnpm --dir dashboard type-check` — types align.
