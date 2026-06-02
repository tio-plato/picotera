# Project merge — Design

## Overview

Adds a one-shot "merge project A into project B" operation. The source project is deleted and every request/trace previously attributed to it is re-attributed to the target. The target's `paths` array is extended with the source's `paths`; its `first_seen_at` and `last_seen_at` are widened to encompass the source's timestamps. The whole thing happens inside a single Postgres transaction so a partial merge is impossible.

## API

A single new operation:

```
POST /api/picotera/projects/merge
Content-Type: application/json

{ "sourceId": 12, "targetId": 7 }
```

Response: `200 OK` with the resulting target `ProjectView` (post-merge). Errors:
- `400 Bad Request` — `sourceId == targetId`, or either id is zero/negative.
- `404 Not Found` — source or target row missing.
- `500 Internal Server Error` — DB failure.

The current `POST /api/picotera/projects/delete` flow is unchanged; merge is a separate endpoint because it carries a target id and returns the merged target row.

## Database

The merge is broken into three sqlc queries that the handler runs inside a single `pgx` transaction. sqlc cannot express a CTE that returns two different row shapes, so we keep each step standalone.

### Step 1 — read both rows

`GetProject(sourceID)` and `GetProject(targetID)` (existing) — used for 404 detection and to confirm both rows are visible to the caller. The target row is also re-read after step 2 to build the response payload.

### Step 2 — update target fields

`db/queries/project.sql`:

```sql
-- name: MergeProjectUpdateTarget :one
UPDATE project AS p
SET paths = COALESCE((
  SELECT jsonb_agg(DISTINCT elem)
  FROM (
    SELECT jsonb_array_elements_text(p.paths) AS elem
    UNION
    SELECT jsonb_array_elements_text(src.paths) AS elem
    FROM project AS src WHERE src.id = @source_id
  ) all_paths
), p.paths),
    first_seen_at = LEAST(p.first_seen_at, (
      SELECT first_seen_at FROM project WHERE id = @source_id
    )),
    last_seen_at  = GREATEST(p.last_seen_at, (
      SELECT last_seen_at FROM project WHERE id = @source_id
    )),
    updated_at = now()
WHERE p.id = @target_id
RETURNING *;
```

`first_seen_at` and `last_seen_at` of the source being `NULL` are treated as "no opinion" by `LEAST` / `GREATEST` — `NULL` inputs collapse to the non-NULL side, matching the existing `UpsertProjectSeen` semantics.

`paths` is rebuilt as the `DISTINCT` union of target and source paths. Duplicates between source and target are dropped, so the target's `paths` array never grows with redundant entries. The `COALESCE(..., p.paths)` keeps the original target array intact when the source has zero paths.

### Step 3 — reassign request rows

```sql
-- name: MergeProjectReassignRequests :execrows
UPDATE request SET project_id = @target_id
WHERE project_id = @source_id;
```

`:execrows` returns the affected row count, which the handler logs at info level via `logx` for operator visibility.

### Step 4 — delete source

`DeleteProject(sourceID)` (existing) drops the source row. Done last so the source row is still readable by step 2's `WHERE id = @source_id` lookups.

## Trace implications

`request.project_id` is the only project reference in the request hypertable; traces are a derived view grouped by `parent_span_id`. After the merge, every span of every trace that was previously attributed to the source now reports the target. The continuous aggregates (`request_overview_hourly`, `request_speed_hourly`) and the `trace_project` LATERAL in `ListRequestTraces` all read `request.project_id` at query time, so no aggregate rebuild is needed — they'll naturally show the merged counts from the next refresh forward.

## Project router invalidation

`Server.projectExtractor` is uncached: it calls `MatchProjectByPaths` against Postgres on every request. The merge therefore needs no in-memory invalidation hook — once step 4 deletes the source row, its `paths` no longer participate in any future match.

## Server wiring

`pkg/server/handle_project.go` gains `handleMergeProject`. The handler:

1. Validates `body.SourceID > 0 && body.TargetID > 0 && body.SourceID != body.TargetID` (400 on failure).
2. Opens a transaction via `s.db.BeginTx(ctx, pgx.TxOptions{})`. The `Server` struct gains a `db *pgxpool.Pool` field, populated in `NewServer` from the same pool used to build `db.New(conn)`. (The existing `queries *db.Queries` is rebuilt per-call via `s.queries.WithTx(tx)` thanks to sqlc's generated helper.)
3. Inside the transaction, using the tx-bound `*Queries`:
   - `GetProject(sourceID)` and `GetProject(targetID)` for the response payload and 404 checks.
   - `MergeProjectUpdateTarget(sourceID, targetID)` — the row returned is the post-merge target.
   - `MergeProjectReassignRequests(sourceID, targetID)` — count logged via `logx`.
   - `DeleteProject(sourceID)`.
4. Commit; if any step errors, the deferred `tx.Rollback` fires and the whole operation is undone.
5. Map `pgx.ErrNoRows` from the initial `GetProject` calls to 404; the unique-violation check is irrelevant here.

The `Server.db` addition is justified — we already hold the pool via `conn` in `NewServer`, we just need a way to begin a tx that spans sqlc-generated queries against the same `*Queries` instance. sqlc generates `(*Queries).WithTx(pgx.Tx) *Queries` for exactly this case (see `pkg/db/db.go`); we hand the bound queries to each step. No new dependencies.

## Contract types

`pkg/contract/project.go` gains:

```go
type MergeProjectRequest struct {
    Body struct {
        SourceID int32 `json:"sourceId"`
        TargetID int32 `json:"targetId"`
    }
}

type MergeProjectResponse struct{ Body ProjectView }

var OperationMergeProject = huma.Operation{
    OperationID: "mergeProject",
    Method:      http.MethodPost,
    Path:        "/projects/merge",
    Summary:     "Merge one project into another",
}
```

`ProjectView` itself is unchanged — the response is just the target row after the merge, projected through the existing `ToProjectView` helper.

## Frontend

### API client

`dashboard/src/api/client.ts` gains `mergeProject(sourceId, targetId)`:

```ts
export async function mergeProject(sourceId: number, targetId: number): Promise<ProjectView> {
  const { data, error } = await api.POST('/api/picotera/projects/merge', {
    body: { sourceId, targetId },
  })
  if (error) fail(error, '合并项目失败')
  return data
}
```

`invalidateProjects(queryClient)` is called by the mutation's `onSuccess` — same as the existing delete path.

### Merge form

`dashboard/src/components/MergeProjectForm.vue` (new). Mirrors `ProjectForm.vue`'s shape but is a one-field form:

- `<SidePanel>` titled `合并「{source.name}」` / kicker `合并项目`. Subtitle carries the warning text.
- A `<Field>` with a `<Select>` (existing UI primitive) bound to a `ProjectView[]` of all projects except the source. The default selected value is `0` (a placeholder option "请选择目标项目"). The target list is loaded via the same `useQuery({ queryKey: queryKeys.projects.all, queryFn: listProjects })` pattern used elsewhere; if the list is empty, render `<StateText>` saying "没有其他项目可合并".
- Footer: cancel + "合并" button. The button is disabled until `targetId > 0`. Submit calls `mergeProjectMutation`.

`useMutation` for the merge:

```ts
const mergeProjectMutation = useMutation({
  mutationFn: (targetId: number) => mergeProject(props.source.id, targetId),
  onSuccess: () => {
    invalidateProjects(queryClient)
    props.onSave?.()
    emit('close')
  },
})
```

The 400 self-merge case is enforced both in the form (target options exclude source) and the backend (defense in depth). The 404 case is shown in the form's error slot via `ApiRequestError.message`.

### ProjectsView row action

`ProjectsView.vue` gains a third `IconButton` in the action cell, placed between edit and delete:

```html
<IconButton
  :active="panel.isActive(`project:merge:${p.id}`)"
  title="合并"
  aria-label="合并"
  @click="openMerge(p)"
>
  <Icon name="git-merge" :size="13" />
</IconButton>
```

`openMerge(p)` opens the side panel:

```ts
function openMerge(p: ProjectView) {
  panel.open(MergeProjectForm, { source: p }, { key: `project:merge:${p.id}`, width: '380px' })
}
```

The merge key (`project:merge:${id}`) is distinct from the edit key (`project:${id}`), so the row does not light up while the merge panel is open — only the button itself does. The existing `:selected` binding on `<Tr>` is left alone.

### Icon

`dashboard/src/ui/icons/paths.ts` gains `git-merge` mapped to `IconGitMerge` from `@tabler/icons-vue` (the package already includes it). No new npm dependency.

### openapi regeneration

After backend changes, run `mise run openapi && pnpm --dir dashboard generate-openapi`. The generated `dashboard/src/openapi-types.d.ts` will expose `MergeProjectRequestBody`; we re-export it from `dashboard/src/api/index.ts` (mirroring the existing `UpsertProjectRequestBody` re-export).

## Out of scope

- Cross-dashboard undo / audit log. The merge is destructive and one-way.
- Merging more than two projects at a time.
- Schema validation that source has zero `auto_created=true` records pointing at it (we don't reference this anywhere yet, so no constraint is needed).
