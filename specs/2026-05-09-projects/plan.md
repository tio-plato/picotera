# Projects — Execution plan

Steps are ordered so that each step compiles and the test suite (such as it is) stays green.

## 1. Migration `020_project_table_and_request_column.sql`

`db/migrations/020_project_table_and_request_column.sql`:

```sql
-- +goose Up
CREATE TABLE project (
  id            SERIAL PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  paths         JSONB NOT NULL DEFAULT '[]'::jsonb,
  first_seen_at TIMESTAMP,
  last_seen_at  TIMESTAMP,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE request ADD COLUMN project_id INTEGER;
CREATE INDEX request_project_id_created_at_idx
  ON request (project_id, created_at DESC, id DESC)
  WHERE project_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS request_project_id_created_at_idx;
ALTER TABLE request DROP COLUMN IF EXISTS project_id;
DROP TABLE IF EXISTS project;
```

## 2. sqlc queries

`db/queries/project.sql` — full CRUD plus the synchronous timestamp upsert:

```sql
-- name: ListProjects :many
SELECT * FROM project ORDER BY name ASC;

-- name: GetProject :one
SELECT * FROM project WHERE id = $1;

-- name: GetProjectByName :one
SELECT * FROM project WHERE name = $1;

-- name: InsertProject :one
INSERT INTO project (name, paths) VALUES ($1, $2) RETURNING *;

-- name: UpdateProject :one
UPDATE project SET name = $2, paths = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteProject :exec
DELETE FROM project WHERE id = $1;

-- name: ListProjectPaths :many
-- Used by the in-memory router. Returns one row per (project_id, path).
SELECT id AS project_id, jsonb_array_elements_text(paths) AS path
FROM project
WHERE jsonb_array_length(paths) > 0;

-- name: UpsertProjectSeen :exec
UPDATE project
SET first_seen_at = LEAST(COALESCE(first_seen_at, $2::timestamp), $2::timestamp),
    last_seen_at  = GREATEST(COALESCE(last_seen_at,  $2::timestamp), $2::timestamp),
    updated_at    = now()
WHERE id = $1;
```

`db/queries/request.sql` — extend the SELECT lists in `ListRequests`, `GetRequest`, `ListRequestsBySpan` to project the new `project_id` column. Add `project_id` filter to `ListRequests`:

```sql
AND (sqlc.narg('project_id')::int IS NULL OR r.project_id = sqlc.narg('project_id'))
```

Update `InsertRequestParams` source query (`InsertRequest`) to take `project_id` and persist it. Add an `UpdateRequestProject` exec when needed in unified gateway flow.

`db/queries/trace.sql` — extend `ListRequestTraces` to include a LATERAL subquery picking `project_id` from the latest meta:

```sql
LEFT JOIN LATERAL (
  SELECT project_id
  FROM request
  WHERE parent_span_id = traces.parent_span_id
    AND created_at >= traces.first_request_at
    AND created_at <= traces.last_request_at
    AND type = 0
    AND project_id IS NOT NULL
  ORDER BY created_at DESC, id DESC
  LIMIT 1
) trace_project ON true
```

Then add `trace_project.project_id AS project_id` to the SELECT.

Run `sqlc generate`. Verify generated code in `pkg/db/`.

## 3. Contract types & operations

`pkg/contract/project.go` (new) — `ProjectView`, `ToProjectView`, request/response types and `OperationListProjects` / `OperationGetProject` / `OperationUpsertProject` / `OperationDeleteProject`. Mirror `pkg/contract/api_key.go` shape.

`pkg/contract/request.go` — extend `RequestView` with `ProjectID *int32`, `ListRequestsRequest` with `ProjectID int32 \`query:"projectId,omitempty"\``, `requestLike` and `toRequestView` to copy the new field, `ToRequestView` / `ToListRequestRowView` / `ToListRequestsBySpanRowView` to map it from the generated row, `RequestTraceView` with `ProjectID *int32`, and `ToRequestTraceView` to read the new lateral.

## 4. Project router

`pkg/server/project_router.go` (new). Mirrors `endpoint_router.go`:

```go
type projectRouter struct {
    queries *db.Queries
    mu      sync.RWMutex
    entries []projectEntry        // sorted: longest path first
    loaded  bool
}

type projectEntry struct {
    path      string
    projectID int32
}
```

Methods: `Match(candidates []string) (int32, bool)`, `Invalidate()`, internal `load(ctx)` reading from `ListProjectPaths`. Sort: `len(path) desc`, ties `projectID asc`.

## 5. Project extractor

`pkg/server/project_extractor.go` (new):

- Package-level `var projectExtractRegexps = []*regexp.Regexp{...}` compiled with `regexp.MustCompile` for the three patterns.
- `type projectExtractor struct { router *projectRouter }`.
- `func (e *projectExtractor) Extract(body []byte) (int32, bool)`:
  1. For each regex, run `FindAllSubmatch(body, -1)`.
  2. For each capture group 1, build `[]byte("\"")+capture+[]byte("\"")` and `json.Unmarshal` into `string`. Skip on error or empty result.
  3. Dedup the candidate slice (small N, slice scan is fine).
  4. Return `e.router.Match(candidates)`.

## 6. Server wiring

`pkg/server/server.go`:

- Add `projectRouter *projectRouter` and `projectExtractor *projectExtractor` to `Server`.
- Construct both in `NewServer` after `endpointRouter`.
- Register the four project operations in `registerOperations`.

## 7. Project handlers

`pkg/server/handle_project.go` (new). Mirrors `handle_api_key.go`:

- `handleListProjects` — iterates `ListProjects` rows, returns `[]ProjectView`.
- `handleGetProject` — `GetProject(id)`, 404 on `pgx.ErrNoRows`.
- `handleUpsertProject` —
  1. Validate `name != ""` and every path entry non-empty.
  2. `body.id == 0` → `InsertProject`. `body.id != 0` → `UpdateProject`.
  3. Map `pgconn.PgError.Code == "23505"` → `huma.Error409Conflict("name already exists")`.
  4. On success, `s.projectRouter.Invalidate()`.
- `handleDeleteProject` — `DeleteProject`, then `Invalidate`.

## 8. Gateway integration

`pkg/server/handle_gateway.go`:

- After `body, err := io.ReadAll(r.Body)`: `projectID, projectMatched := h.projectExtractor.Extract(body)`. Build `pgtype.Int4{Int32: projectID, Valid: projectMatched}`.
- Pass `ProjectID` into the meta `InsertRequestParams` and into every upstream `InsertRequestParams` inside the retry loop.
- After meta insert, when `projectMatched`: fire-and-forget `go func() { _ = h.queries.UpsertProjectSeen(bgCtx, db.UpsertProjectSeenParams{ID: projectID, SeenAt: pgtype.Timestamp{Time: metaCreatedAt, Valid: true}}) }()`. Errors logged at warn via `logx`.

`pkg/server/handle_unified_gateway.go` — repeat the same three changes (extract once after `io.ReadAll`, propagate to meta insert and every upstream insert in the loop, async upsert).

`db/queries/request.sql` — `InsertRequestParams` now carries `ProjectID`. Update `InsertRequest` to `INSERT INTO request (..., project_id) VALUES (..., $N)` and run `sqlc generate`.

## 9. OpenAPI regen

```
mise run openapi
pnpm --dir dashboard generate-openapi
```

Verify diff covers `ProjectView`, the four project operations, `RequestView.projectId`, `RequestTraceView.projectId`, and `ListRequests.projectId` query.

## 10. Dashboard data layer

`dashboard/src/api/queryKeys.ts`:
- Add `projects: { all: ['projects'], detail: (id: number) => ['projects', id] }`.
- Extend `RequestsFilters` with `projectId?: number`.

`dashboard/src/api/client.ts`:
- `listProjects() / getProject(id) / upsertProject(body) / deleteProject(id)` — same shape as the api-keys helpers.
- `invalidateProjects(client)` and add a call from `invalidateProjects` into `invalidateRequests`-style fan-out where appropriate (touching projects affects requests/traces label lookups).

`dashboard/src/composables/useProjectsMap.ts` (new) — clone of `useProvidersMap.ts`.

## 11. Dashboard views

`dashboard/src/views/ProjectsView.vue` (new) — list + create/edit via side panel. Use `DataTable` + `Th/Td/Tr` + `IconButton`s, mirror `ScriptsView.vue`.

`dashboard/src/components/ProjectForm.vue` (new) — name input + repeating path input list (model after `ModelListEditor` for the array of strings; rejects empty entries).

`dashboard/src/router/index.ts` — add `{ path: '/projects', name: 'projects', component: () => import('@/views/ProjectsView.vue') }`.

`dashboard/src/components/AppSidebar.vue` — insert `{ name: 'projects', label: '项目', icon: 'folder' }` between `apiKeys` and `scripts`. If `folder` is not in `src/ui/icons/paths.ts`, add the @tabler/icons-vue `folder` glyph.

`dashboard/src/App.vue` — `pageMeta` adds `projects: { title: '项目', hint: '今天蹬到哪里去了' }`.

## 12. Dashboard requests/traces columns

`dashboard/src/views/RequestsView.vue`:
- `filters` reactive gains `projectId: 0`.
- Sync `projectId` through the URL query (mirror existing `traceId` round-trip).
- Add `projectId` to `requestFilters` computed.
- New column key `projectId` between `userMessagePreview` and `providerId`.
- `<template #header-projectId>` renders a `ColumnFilter` using `projectOptions` derived from `useProjectsMap`.
- `<template #cell-projectId>` renders the project name (or `—`).
- Watch `route.query.projectId` to sync the filter.

`dashboard/src/views/TracesView.vue`:
- New column key `projectId` between `userMessagePreview` and `id`.
- `<template #cell-projectId>` renders the name. Click handler pushes to `/requests?projectId=<id>` (use `event.stopPropagation` so it overrides the row-click → `traceId` navigation).

## 13. CLAUDE.md

Update three sections:

1. **Architecture → Database Schema**: change "Nine tables" to "Ten tables" and add `project` to the enumeration. Add a bullet noting `project_id` is on the `request` hypertable.
2. **Architecture → Key Patterns**: add a new bullet:
   > **Project matching**: every gateway request body is scanned by `pkg/server/project_extractor.go` (three fixed regexes: `Workspace root folder:`, `Primary working directory:`, `<cwd>…</cwd>`). Captures are JSON-unescaped and looked up in `Server.projectRouter` (in-memory longest-prefix cache, mirrors `endpointRouter`). The matched `project_id` is written onto the `request` row and triggers a synchronous `UpsertProjectSeen` updating `project.first_seen_at` / `project.last_seen_at`. Any mutation of the `project` table MUST call `Server.projectRouter.Invalidate()`.
3. **Dashboard → Dashboard Layout**: add `ProjectsView` to the views enumeration and `ProjectForm` to the components enumeration.

## 14. Smoke / verification

- `go build ./...` clean.
- `pnpm --dir dashboard build` clean (vue-tsc + vite).
- Start backend with docker-compose Postgres up. Create one project via `PUT /projects` with paths `["/home/oott123/Work/Projects/picotera"]`. Send a request whose body contains `Workspace root folder: /home/oott123/Work/Projects/picotera/dashboard\n`. Verify:
  - `request.project_id` matches the project row's id.
  - `project.first_seen_at` and `project.last_seen_at` are populated.
- Send a request whose body contains no recognizable marker → `request.project_id` is NULL.
- Hit `GET /requests?projectId=<id>` and confirm filtered results.
- Open the dashboard, confirm the new sidebar entry, list page, form, and that the new column renders + filters in both `RequestsView` and `TracesView`.
