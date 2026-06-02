# Project merge — Execution plan

Steps are ordered so that each step compiles and the existing test suite stays green.

## 1. sqlc queries

Append to `db/queries/project.sql`:

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

-- name: MergeProjectReassignRequests :execrows
UPDATE request SET project_id = @target_id
WHERE project_id = @source_id;
```

Run `sqlc generate`. Verify the generated `pkg/db/project.sql.go` exposes `MergeProjectUpdateTarget` returning a `Project` row and `MergeProjectReassignRequests` returning `int64` (affected row count). The `Querier` interface in `pkg/db/querier.go` gets two new methods.

## 2. Contract types & operation

`pkg/contract/project.go` — add at the bottom:

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

No new types in `ProjectView` itself.

## 3. Server struct — add `db` pool

`pkg/server/server.go`:

- Add `db *pgxpool.Pool` to the `Server` struct.
- In `NewServer`, after `queries := db.New(conn)`, store `server.db = conn`. Keep the local variable named `conn` so the existing `conn.Close()` call in the error path stays unchanged.

## 4. Merge handler

`pkg/server/handle_project.go` — add `handleMergeProject`:

```go
func (s *Server) handleMergeProject(ctx context.Context, in *contract.MergeProjectRequest) (*contract.MergeProjectResponse, error) {
    src := in.Body.SourceID
    tgt := in.Body.TargetID
    if src <= 0 || tgt <= 0 {
        return nil, huma.Error400BadRequest("sourceId and targetId must be positive")
    }
    if src == tgt {
        return nil, huma.Error400BadRequest("source and target must be different projects")
    }

    tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return nil, huma.Error500InternalServerError("failed to begin transaction", err)
    }
    defer tx.Rollback(ctx)
    q := s.queries.WithTx(tx)

    if _, err := q.GetProject(ctx, src); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, huma.Error404NotFound("source project not found")
        }
        return nil, huma.Error500InternalServerError("failed to load source project", err)
    }
    if _, err := q.GetProject(ctx, tgt); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, huma.Error404NotFound("target project not found")
        }
        return nil, huma.Error500InternalServerError("failed to load target project", err)
    }

    updated, err := q.MergeProjectUpdateTarget(ctx, db.MergeProjectUpdateTargetParams{
        SourceID: src,
        TargetID: tgt,
    })
    if err != nil {
        return nil, huma.Error500InternalServerError("failed to update target project", err)
    }

    rewritten, err := q.MergeProjectReassignRequests(ctx, db.MergeProjectReassignRequestsParams{
        SourceID: src,
        TargetID: tgt,
    })
    if err != nil {
        return nil, huma.Error500InternalServerError("failed to reassign request rows", err)
    }

    if err := q.DeleteProject(ctx, src); err != nil {
        return nil, huma.Error500InternalServerError("failed to delete source project", err)
    }

    if err := tx.Commit(ctx); err != nil {
        return nil, huma.Error500InternalServerError("failed to commit transaction", err)
    }

    logx.WithContext(ctx).WithFields(logrus.Fields{
        "sourceId":           src,
        "targetId":           tgt,
        "rewrittenRequests":  rewritten,
    }).Info("merged project")

    v, err := contract.ToProjectView(&updated)
    if err != nil {
        return nil, huma.Error500InternalServerError("failed to encode project", err)
    }
    return &contract.MergeProjectResponse{Body: *v}, nil
}
```

`WithTx` returns a fresh `*Queries` bound to the transaction. We use it for every read/write inside the transaction; the `s.queries` reference is only used outside this handler.

Required imports on `handle_project.go` for this handler: `github.com/sirupsen/logrus` and `picotera/pkg/logx`. The existing file already imports `context`, `encoding/json`, `errors`, `picotera/pkg/contract`, `picotera/pkg/db`, `huma`, and `pgx`.

## 5. Register the operation

`pkg/server/server.go` — in `registerOperations`, add one line below the existing project registrations:

```go
huma.Register(mgmt, contract.OperationMergeProject, s.handleMergeProject)
```

## 6. OpenAPI regen

```
mise run openapi
pnpm --dir dashboard generate-openapi
```

Verify the diff adds `mergeProject` operation, `MergeProjectRequestBody` schema, and the request/response types. The generated `dashboard/src/openapi-types.d.ts` will pick up the new body type automatically.

## 7. Dashboard — API client

`dashboard/src/api/client.ts` — add after `deleteProject`:

```ts
export async function mergeProject(sourceId: number, targetId: number): Promise<ProjectView> {
  const { data, error } = await api.POST('/api/picotera/projects/merge', {
    body: { sourceId, targetId },
  })
  if (error) fail(error, '合并项目失败')
  return data
}
```

No new query key is needed — the merge uses the existing `projects.all` key, and `invalidateProjects` already fans out to `requests` and `requestTraces`.

## 8. Dashboard — re-export body type

`dashboard/src/api/index.ts` — add:

```ts
export type MergeProjectRequestBody = components['schemas']['MergeProjectRequestBody']
```

## 9. Dashboard — icon

`dashboard/src/ui/icons/paths.ts`:

- Import: `import { ..., IconGitMerge } from '@tabler/icons-vue'`.
- `IconName` union: add `'git-merge'`.
- `iconComponents` map: add `git-merge: IconGitMerge`.

## 10. Dashboard — merge form

`dashboard/src/components/MergeProjectForm.vue` (new). Structure:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { SidePanel, Button, Field, Select, StateText } from '@/ui'
import type { ProjectView } from '@/api'
import { invalidateProjects, listProjects, mergeProject } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ source: ProjectView; onSave?: () => void }>()
const queryClient = useQueryClient()

const projectsQuery = useQuery({ queryKey: queryKeys.projects.all, queryFn: listProjects })
const candidates = computed(() =>
  (projectsQuery.data.value ?? []).filter((p) => p.id !== props.source.id),
)

const targetId = ref(0)
const saving = ref(false)
const error = ref('')

const mergeProjectMutation = useMutation({
  mutationFn: (id: number) => mergeProject(props.source.id, id),
  onSuccess: () => {
    invalidateProjects(queryClient)
    props.onSave?.()
    emit('close')
  },
})

async function submit() {
  saving.value = true
  error.value = ''
  try {
    await mergeProjectMutation.mutateAsync(targetId.value)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '合并失败'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <SidePanel
    :title="`合并「${source.name || source.id}」`"
    kicker="合并项目"
    :subtitle="`将把「${source.name || source.id}」的所有请求和追踪合并到目标项目，源项目将被删除。`"
    @close="emit('close')"
  >
    <StateText v-if="projectsQuery.isLoading.value">加载中…</StateText>
    <StateText v-else-if="candidates.length === 0">没有其他项目可合并</StateText>
    <form v-else id="merge-project-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="目标项目">
        <Select v-model="targetId" :model-modifiers="{ number: true }" required>
          <option :value="0" disabled>请选择目标项目</option>
          <option v-for="c in candidates" :key="c.id" :value="c.id">{{ c.name }}</option>
        </Select>
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button
        type="submit"
        form="merge-project-form"
        :disabled="saving || targetId === 0"
      >
        {{ saving ? '合并中…' : '合并' }}
      </Button>
    </template>
  </SidePanel>
</template>
```

## 11. Dashboard — projects view row action

`dashboard/src/views/ProjectsView.vue`:

- Import `MergeProjectForm` and the `git-merge` icon (icon is already global).
- Add `openMerge(p: ProjectView)` function:
  ```ts
  function openMerge(p: ProjectView) {
    panel.open(MergeProjectForm, { source: p }, { key: `project:merge:${p.id}`, width: '380px' })
  }
  ```
- Add a third `IconButton` in the actions cell, between edit and delete:
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
- The existing `:selected="panel.isActive(`project:${p.id}`)"` on `<Tr>` stays as-is. The merge panel uses a different key, so the row does not light up when the merge form opens — only the merge button itself does (via its `:active` binding). The side panel title carries the source's name.

## 13. Smoke / verification

- `go build ./...` clean.
- `pnpm --dir dashboard build` clean (vue-tsc + vite).
- Start backend with docker-compose Postgres up. Create three projects: `A` (id 1), `B` (id 2), `C` (id 3) with distinct paths. Send a few requests tagged to A and C. Then:
  - `POST /api/picotera/projects/merge { sourceId: 1, targetId: 2 }`:
    - 200 with `B`'s row, paths = `[B.paths..., A.paths...]` (DISTINCT-deduped).
    - `first_seen_at` = min, `last_seen_at` = max.
    - `request.project_id` for the A-tagged requests is now 2.
    - `GET /api/picotera/projects/1` returns 404.
- `POST /api/picotera/projects/merge { sourceId: 2, targetId: 2 }` returns 400.
- `POST /api/picotera/projects/merge { sourceId: 1, targetId: 9999 }` returns 404.
- `GET /api/picotera/requests?projectId=2` returns the merged set.
- `GET /api/picotera/request-traces` for those requests shows the merged `projectId` in the LATERAL.
- Open the dashboard, click the merge button on row A, choose B, confirm; the row count drops by one and the rows previously attributed to A now show B's name in the project column of both the requests and traces views.
