# Plan: ProviderEndpoint CRUD

## Step 1 — SQL queries (`db/queries/provider_endpoint.sql`)

Create three queries:

```sql
-- name: ListProviderEndpoints :many
SELECT * FROM provider_endpoint
WHERE provider_id = $1
ORDER BY endpoint_id;

-- name: UpsertProviderEndpoint :one
INSERT INTO provider_endpoint (provider_id, endpoint_id, upstream_url)
VALUES ($1, $2, $3)
ON CONFLICT (provider_id, endpoint_id) DO UPDATE SET
  upstream_url = EXCLUDED.upstream_url
RETURNING *;

-- name: DeleteProviderEndpoint :exec
DELETE FROM provider_endpoint
WHERE provider_id = $1 AND endpoint_id = $2;
```

## Step 2 — Generate sqlc code

Run `sqlc generate` to produce Go types and methods in `pkg/db/`.

## Step 3 — Error code (`pkg/errorx/errors.go`)

Add:
```go
var ProviderEndpointNotFound = ErrorCode("PROVIDER_ENDPOINT_NOT_FOUND")
```

## Step 4 — Contract types (`pkg/contract/provider_endpoint.go`)

Create with:
- `ProviderEndpointView` struct — `ProviderId`, `EndpointId`, `UpstreamUrl`
- `ToProviderEndpointView()` — convert db model to view
- `FromProviderEndpointView()` — convert view to sqlc params
- `ListProviderEndpointsRequest` — with `ProviderID` query param
- `ListProviderEndpointsResponse` — with `Body []ProviderEndpointView`
- `UpsertProviderEndpointRequest` / `UpsertProviderEndpointResponse`
- `DeleteProviderEndpointRequest`
- Three `huma.Operation` definitions

## Step 5 — Server handler (`pkg/server/handle_provider_endpoint.go`)

Implement:
- `handleListProviderEndpoints` — query by provider_id, convert to views, return array
- `handleUpsertProviderEndpoint` — convert view to params, upsert, return view
- `handleDeleteProviderEndpoint` — delete by composite key

## Step 6 — Register operations (`pkg/server/server.go`)

Add three `huma.Register` calls in `registerOperations()`.

## Step 7 — Verify

Run `go build ./cmd/picotera` to confirm compilation.
