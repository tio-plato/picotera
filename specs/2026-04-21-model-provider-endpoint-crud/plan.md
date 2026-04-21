# Plan: Model Provider Endpoint CRUD + Generic Pagination

## Step 1: Add generic pagination types

Create `pkg/contract/pagination.go`:
- `PaginationRequest` struct with `Limit` and `Cursor` fields
- `PaginationInfo` struct with `NextCursor` and `HasMore` fields
- `PaginatedBody[T any]` generic struct with `Items []T` and `Pagination PaginationInfo`
- `PaginatedResponse[T any]` generic struct with `Body PaginatedBody[T]`
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

- `ModelProviderEndpointView` struct:
```go
type ModelProviderEndpointView struct {
    ModelName         string            `json:"modelName"`
    ProviderID        int32             `json:"providerId"`
    EndpointID        int32             `json:"endpointId"`
    UpstreamModelName string            `json:"upstreamModelName,omitempty"`
    Priority          int32             `json:"priority"`
    Annotations       map[string]string `json:"annotations"`
}
```

- `ToModelProviderEndpointView` / `FromModelProviderEndpointView` conversion functions

- Request/Response types:

```go
// List
type ListModelProviderEndpointsRequest struct {
    PaginationRequest
    ModelName  string `query:"modelName,omitempty"`
    ProviderID *int32 `query:"providerId,omitempty"`
    EndpointID *int32 `query:"endpointId,omitempty"`
}

type ListModelProviderEndpointsResponse = PaginatedResponse[ModelProviderEndpointView]

// Get
type GetModelProviderEndpointRequest struct {
    ModelName  string `path:"modelName"`
    ProviderID int32  `path:"providerId"`
    EndpointID int32  `path:"endpointId"`
}

type GetModelProviderEndpointResponse struct {
    Body ModelProviderEndpointView
}

// Upsert
type UpsertModelProviderEndpointRequest struct {
    Body ModelProviderEndpointView
}

type UpsertModelProviderEndpointResponse struct {
    Body ModelProviderEndpointView
}

// Delete
type DeleteModelProviderEndpointRequest struct {
    Body struct {
        ModelName  string `json:"modelName"`
        ProviderID int32  `json:"providerId"`
        EndpointID int32  `json:"endpointId"`
    }
}
```

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
