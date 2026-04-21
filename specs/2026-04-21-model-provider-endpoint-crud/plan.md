# Plan: Model Provider Endpoint CRUD + Generic Pagination

## Step 1: Add generic pagination types

Create `pkg/contract/pagination.go`:
- `PaginationRequest` struct with `Limit` and `Cursor` fields
- `PaginationInfo` struct with `NextCursor` and `HasMore` fields
- `EncodeCursor(values ...any)` helper — JSON-encodes key values then base64-encodes
- `DecodeCursor(cursor string, targets ...any)` helper — base64-decodes then JSON-decodes into targets

## Step 2: Add sqlc queries for model_provider_endpoint

Create `db/queries/model_provider_endpoint.sql`:
- `GetModelProviderEndpoint` — SELECT by composite PK (model_name, provider_id, endpoint_id)
- `ListModelProviderEndpoints` — paginated SELECT using `sqlc.narg()` for optional cursor and filter parameters. Keyset pagination via `WHERE (model_name, provider_id, endpoint_id) > (cursor_values)` condition gated by `@has_cursor::bool`. Optional filters (modelName, providerId, endpointId) gated by `@filter_*::bool`. Fetch limit+1 rows to determine HasMore.
- `UpsertModelProviderEndpoint` — INSERT ... ON CONFLICT (model_name, provider_id, endpoint_id) DO UPDATE ... RETURNING *
- `DeleteModelProviderEndpoint` — DELETE by composite PK

Run `sqlc generate` to regenerate `pkg/db/`.

## Step 3: Add error codes

Add `ModelProviderEndpointNotFound` to `pkg/errorx/errors.go`.

## Step 4: Add contract types and operations

Create `pkg/contract/model_provider_endpoint.go`:
- `ModelProviderEndpointView` struct
- `ToModelProviderEndpointView` / `FromModelProviderEndpointView` conversion functions
- Request/Response types for each operation (List, Get, Upsert, Delete)
- `ListModelProviderEndpointsRequest` embeds `PaginationRequest` plus filter fields (modelName, providerId, endpointId)
- `ListModelProviderEndpointsResponse` contains `Items []ModelProviderEndpointView` and `PaginationInfo`
- Operation variables (OperationListModelProviderEndpoints, OperationGetModelProviderEndpoint, OperationUpsertModelProviderEndpoint, OperationDeleteModelProviderEndpoint)

## Step 5: Add server handlers

Create `pkg/server/handle_model_provider_endpoint.go`:
- `handleListModelProviderEndpoints` — parse cursor, call query, build pagination response
- `handleGetModelProviderEndpoint` — get by composite PK, 404 if not found
- `handleUpsertModelProviderEndpoint` — upsert and return view
- `handleDeleteModelProviderEndpoint` — delete by composite PK

Register all four operations in `server.go` `registerOperations()`.

## Step 6: Verify build

Run `go build -o picotera ./cmd/picotera` to ensure everything compiles.
