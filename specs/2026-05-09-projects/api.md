# Projects — API

All paths are mounted under the existing `/api/picotera` group.

## Types

```go
type ProjectView struct {
    ID            int32    `json:"id"`
    Name          string   `json:"name"`
    Paths         []string `json:"paths"`
    FirstSeenAt   string   `json:"firstSeenAt,omitempty"` // RFC3339, omitted when NULL
    LastSeenAt    string   `json:"lastSeenAt,omitempty"`
    CreatedAt     string   `json:"createdAt"`
    UpdatedAt     string   `json:"updatedAt"`
}

type ProjectMutateBody struct {
    ID    int32    `json:"id,omitempty"`     // 0 = create, otherwise update
    Name  string   `json:"name"`             // required
    Paths []string `json:"paths"`            // may be empty; empty paths string entries are rejected
}
```

`paths` strings are stored verbatim — no trimming, case-folding, or normalization (per project working conventions). Empty-string entries in `paths` are rejected with 400.

## Operations

### `GET /api/picotera/projects`

Lists all projects.

**Response**: `200 OK`, body `[]ProjectView` ordered by `name ASC`.

### `GET /api/picotera/projects/{id}`

**Response**: `200 OK` with `ProjectView`. `404` when no row matches.

### `PUT /api/picotera/projects`

Upsert. When `body.id == 0`, inserts and returns the created row. When `body.id != 0`, updates the matched row.

**Validation**:
- `name` required, non-empty.
- `paths` may be empty; if non-empty, every entry must be a non-empty string.
- `name` must be unique across the table — `409 Conflict` on collision.

**Response**: `200 OK` with the resulting `ProjectView`.

**Side effect**: invalidates `Server.projectRouter`.

### `POST /api/picotera/projects/delete`

Body: `{ "id": <int32> }`. Returns `200 OK` empty body.

**Side effect**: invalidates `Server.projectRouter`. `request.project_id` rows pointing at the deleted project become orphaned (`NULL` in the FK sense — there's no FK constraint, so no cascade is needed). The dashboard renders orphan ids as `—` since `projectsMap.get(id)` will be undefined.

## Existing operations — additions

### `GET /api/picotera/requests`

New optional query parameter:

- `projectId int32` — when present and non-zero, filters to requests whose `project_id` equals the given value.

`RequestView` gains `projectId *int32` (matches existing `*int32` pointer pattern for nullable ids).

### `GET /api/picotera/request-traces`

`RequestTraceView` gains `projectId *int32` populated from the most-recent meta row inside the trace window. No new query parameter.

## Endpoint registration

In `pkg/server/server.go`'s `registerOperations`:

```go
huma.Register(mgmt, contract.OperationListProjects,   s.handleListProjects)
huma.Register(mgmt, contract.OperationGetProject,     s.handleGetProject)
huma.Register(mgmt, contract.OperationUpsertProject,  s.handleUpsertProject)
huma.Register(mgmt, contract.OperationDeleteProject,  s.handleDeleteProject)
```

After every code change, regenerate `openapi.yaml` (`mise run openapi`) and `dashboard/src/openapi-types.d.ts` (`pnpm --dir dashboard generate-openapi`).
