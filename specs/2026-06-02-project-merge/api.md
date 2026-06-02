# Project merge — API

All paths are mounted under the existing `/api/picotera` group.

## Operations

### `POST /api/picotera/projects/merge`

Merges source project into target project in a single transaction: rewrites every `request.project_id` from `sourceId` to `targetId`, extends the target's `paths` JSONB with the source's paths, widens `first_seen_at` / `last_seen_at` to encompass the source, then deletes the source row.

**Request body**:

```go
type MergeProjectRequest struct {
    Body struct {
        SourceID int32 `json:"sourceId"`
        TargetID int32 `json:"targetId"`
    }
}
```

**Validation**:
- `sourceId > 0`
- `targetId > 0`
- `sourceId != targetId`

Violations return `400 Bad Request`.

**Responses**:
- `200 OK` with the resulting target `ProjectView` (post-merge; same shape as `GET /projects/{id}`).
- `404 Not Found` when either project row does not exist.
- `500 Internal Server Error` on any DB failure. The transaction is rolled back; no rows are mutated.

**Example**:

```http
POST /api/picotera/projects/merge
Content-Type: application/json

{ "sourceId": 12, "targetId": 7 }
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": 7,
  "name": "picotera",
  "paths": ["/home/user/picotera", "/tmp/spike"],
  "firstSeenAt": "2026-05-09T10:00:00Z",
  "lastSeenAt":  "2026-06-02T14:30:00Z",
  "createdAt":   "2026-05-09T09:55:00Z",
  "updatedAt":   "2026-06-02T14:35:00Z",
  "autoCreated": false
}
```

## Existing operations — no changes

The merge operation is purely additive. `POST /api/picotera/projects/delete`, `PUT /api/picotera/projects`, `GET /api/picotera/projects`, `GET /api/picotera/projects/{id}` keep their current shape and semantics.

## Endpoint registration

In `pkg/server/server.go`'s `registerOperations`:

```go
huma.Register(mgmt, contract.OperationMergeProject, s.handleMergeProject)
```

After every code change, regenerate `openapi.yaml` (`mise run openapi`) and `dashboard/src/openapi-types.d.ts` (`pnpm --dir dashboard generate-openapi`).
