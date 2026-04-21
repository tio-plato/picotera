# Design: Model Provider Endpoint CRUD + Generic Pagination

## Pagination Design

Use **cursor-based pagination** with keyset pagination on the primary key. This is more efficient than offset-based for large datasets and works consistently with inserts/deletes.

### Generic Pagination Contract

Define reusable pagination types in `pkg/contract/pagination.go`:

```go
type PaginationRequest struct {
    Limit  int32 `query:"limit" example:"20" default:"20" maximum:"100" minimum:"1"`
    Cursor string `query:"cursor" example:"eyJpZCI6MX0="`
}

type PaginationInfo struct {
    NextCursor string `json:"nextCursor,omitempty"`
    HasMore    bool   `json:"hasMore"`
}
```

- `Limit`: max items per page (1-100, default 20).
- `Cursor`: opaque base64-encoded token encoding the last row's sort key. Empty cursor = first page.
- `NextCursor`: populated only when `HasMore` is true. Client passes this as `cursor` on the next request.

### Cursor Encoding

Each list endpoint encodes its primary key components into a JSON struct, then base64-encodes it. For example:

- `model_provider_endpoint`: cursor = base64(`{"modelName":"gpt-4o","providerId":1,"endpointId":2}`)
- `model`: cursor = base64(`{"name":"gpt-4o"}`)
- `provider`: cursor = base64(`{"id":5}`)

### SQL Pattern

Use keyset pagination with `sqlc.narg()` for optional cursor parameters. Two conditions handle the cursor:

```sql
WHERE
  -- filter conditions (always applied when provided)
  (@filter_model_name::bool IS NOT TRUE OR model_name = @model_name::text)
  AND (@filter_provider_id::bool IS NOT TRUE OR provider_id = @provider_id::int)
  AND (@filter_endpoint_id::bool IS NOT TRUE OR endpoint_id = @endpoint_id::int)
  -- keyset pagination (applied when cursor is provided)
  AND (
    @has_cursor::bool IS NOT TRUE
    OR (model_name, provider_id, endpoint_id) > (@cursor_model_name::text, @cursor_provider_id::int, @cursor_endpoint_id::int)
  )
ORDER BY model_name, provider_id, endpoint_id
LIMIT @limit::int
```

The handler passes `has_cursor = true` only when a cursor is provided, and decodes the cursor into the three key components. When `has_cursor = false/null`, the keyset condition is vacuously true and all rows (matching filters) are returned from the beginning.

Fetch `limit + 1` rows to determine `HasMore` without a separate count query.

## Model Provider Endpoint CRUD Design

### View Type

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

### Operations

| Operation | Method | Path | Notes |
|-----------|--------|------|-------|
| List | GET | /model-provider-endpoints | Paginated, supports filtering by `modelName`, `providerId`, `endpointId` query params |
| Get | GET | /model-provider-endpoints/{modelName}/{providerId}/{endpointId} | Get by composite primary key |
| Upsert | PUT | /model-provider-endpoints | Insert or update by composite PK |
| Delete | POST | /model-provider-endpoints/delete | Delete by composite PK in body |

### Filtering

The list endpoint supports optional filters:
- `modelName` — filter by model name (exact match)
- `providerId` — filter by provider ID
- `endpointId` — filter by endpoint ID

When filters are combined with pagination, the cursor still encodes the full composite key, and the WHERE clause adds filter conditions alongside the keyset condition.

### sqlc Queries

New file `db/queries/model_provider_endpoint.sql` with:
- `ListModelProviderEndpoints` — paginated list with optional filters
- `GetModelProviderEndpoint` — get by composite PK
- `UpsertModelProviderEndpoint` — upsert by composite PK
- `DeleteModelProviderEndpoint` — delete by composite PK

### Error Codes

Add to `pkg/errorx/errors.go`:
- `ModelProviderEndpointNotFound`
