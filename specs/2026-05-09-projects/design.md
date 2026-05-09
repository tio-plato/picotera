# Projects — Design

## Overview

The "projects" feature attaches a `project_id` to every gateway request whose body contains a recognizable workspace/cwd path. Operators define projects (name + list of path prefixes) via the management API. The gateway runs a fixed set of regexes against the raw request body, JSON-unescapes each capture, then matches the candidates against an in-memory cache of all configured project paths to pick the longest matching prefix. The matched `project_id` is written into the `request` row at insert time, and the project row's `first_seen_at` / `last_seen_at` are upserted in the same code path that writes the request.

## Data Model

New table:

```sql
CREATE TABLE project (
  id              SERIAL PRIMARY KEY,
  name            TEXT NOT NULL UNIQUE,
  paths           JSONB NOT NULL DEFAULT '[]'::jsonb,    -- array of strings
  first_seen_at   TIMESTAMP,                              -- nullable until first hit
  last_seen_at    TIMESTAMP,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`paths` is stored as a JSON array of strings on the project row. Two projects MAY have overlapping paths; the matcher resolves overlaps by longest-prefix-wins, then by `(project_id ASC)` as a deterministic tie-breaker. No uniqueness constraint is enforced across rows.

`request` table gains:

```sql
ALTER TABLE request ADD COLUMN project_id INTEGER;
CREATE INDEX request_project_id_created_at_idx ON request (project_id, created_at DESC, id DESC)
  WHERE project_id IS NOT NULL;
```

`project_id` is nullable; existing rows stay NULL (no backfill — historical request bodies are not re-parsed).

## Project router (in-memory cache)

`pkg/server/project_router.go` mirrors `endpoint_router.go`:

- Lazy-loaded on first `Match`, refreshed on every project mutation via `Invalidate()`.
- Holds a flat slice of `{path string; projectID int32}` entries sorted by `len(path)` descending, ties broken by `projectID ASC`.
- `Match([]string) (int32, bool)` takes the candidate list emitted by the regex pipeline. It walks entries (longest path first) and returns the first entry whose `path` is a prefix of any candidate. Because entries are sorted longest-first, the first hit is automatically the longest-match. Returns `(0, false)` when no entry matches.
- Server holds `*projectRouter` alongside `*endpointRouter`. Mutation handlers (`upsertProject`, `deleteProject`) MUST call `Invalidate()`.

The router only loads projects whose `paths` array is non-empty — projects with zero paths cannot match anything anyway, and skipping them keeps the comparison loop tighter.

Prefix semantics: `path` matches `candidate` iff `strings.HasPrefix(candidate, path)`. We deliberately do NOT normalize separators or case — operators control both sides of the match by explicitly listing path prefixes.

## Extraction pipeline

`pkg/server/project_extractor.go`:

1. A package-level `[]*regexp.Regexp` is compiled once at init time from the three patterns:
   - `Workspace root folder: (.*?)\n`
   - `Primary working directory: (.*?)\n`
   - `<cwd>(.*?)</cwd>`
   `regexp.Compile` is used (not `MustCompile`) so a future bad pattern still fails the build via the smoke test.
2. For each regex, `FindAllSubmatch(body, -1)` collects every capture group 1.
3. Each capture is wrapped as `"<bytes>"` and unmarshalled with `json.Unmarshal` into a Go string. Failures (invalid JSON escape) drop that single capture but do not abort the request.
4. Empty post-decode strings are dropped.
5. The deduplicated list of decoded candidates is handed to `projectRouter.Match`.

The extractor returns `(projectID int32, matched bool)`. When `matched == false` the gateway writes `project_id = NULL`.

## Gateway integration

In `handle_gateway.go` and `handle_unified_gateway.go`, after the request body is read and **before** `insertRequest` is called for the meta row:

```go
projectID, projectMatched := h.projectExtractor.Extract(body)
```

The match result feeds:

- `InsertRequestParams.ProjectID = pgtype.Int4{Int32: projectID, Valid: projectMatched}` for the meta row, and is propagated to upstream rows in the retry loop (every span in the trace shares the same project).
- A **non-blocking** `UpsertProjectSeen(projectID, metaCreatedAt)` call when matched, guarded by `bgCtx`. The query is a single `UPDATE project SET first_seen_at = LEAST(...), last_seen_at = GREATEST(...), updated_at = now() WHERE id = $1` (no INSERT half — the project row already exists). Same pattern as `UpsertTrace` minus the upsert, since project rows are created via the management API and never auto-created by the gateway.

Failures of `UpsertProjectSeen` log at warn and are swallowed — they must not affect request handling.

## Trace view aggregation

Traces are derived in SQL from grouped `request` rows; project doesn't go onto the `traces` table directly. `ListRequestTraces` is extended:

- A new LATERAL subquery picks the `project_id` from the most-recent meta row in the trace window (mirrors `user_message_preview`'s LATERAL).
- `RequestTraceView` gains `projectId *int32`.

## API surface

Five operations under `/api/picotera/projects`:

- `GET    /projects`                — list all
- `GET    /projects/{id}`           — get by id
- `PUT    /projects`                — upsert (creates when id absent, updates when present)
- `POST   /projects/delete`         — delete by id (matches the existing pattern: api-keys/delete, scripts/delete, etc.)

Plus filter knobs on existing list endpoints:

- `GET /requests?projectId=…`
- `GET /request-traces` keeps current shape but each row gains `projectId`.

Detailed request/response shapes live in `api.md`.

## Dashboard

New view `ProjectsView.vue` + `ProjectForm.vue`:

- Sidebar entry between "密钥" and "脚本" (icon `folder`).
- Table columns: 名称 / 路径数 / 首次出现 / 最近出现 / 操作.
- Form fields: 名称 (required, unique) / 路径列表 (use the existing `ModelListEditor` pattern — array of free-text strings with add/remove buttons).

`RequestsView.vue` and `TracesView.vue` get a "项目" column:

- Inserted between 用户消息 and 渠道.
- Cell renders `projectsMap[row.projectId]?.name` or `—`.
- Header carries a `ColumnFilter` sourced from the projects list.
- For requests, filter binds to `filters.projectId` (new key on the filters reactive). For traces, clicking a project cell pushes `/requests?projectId=<id>`.

A new composable `useProjectsMap.ts` mirrors `useProvidersMap.ts` to give `(projects, projectLabel(id))`.

`useApi`-side: `client.ts` gains `listProjects / upsertProject / deleteProject`, plus an `invalidateProjects(client)` helper. `queryKeys.projects = { all: ['projects'], detail: id => ['projects', id] }`.

`RequestsFilters` type adds optional `projectId?: number`. The map currently used for url-driven filters in `RequestsView.vue` extends to round-trip `projectId` through the route query (matches the existing `traceId` round-trip).

## Caching & ordering subtleties

- The matcher is a synchronous in-memory walk; with O(N) projects×paths it is fine at the scales we care about. We are explicitly trading scan cost for simpler invalidation semantics — same trade as the endpoint router.
- A request body large enough to make `regexp.FindAllSubmatch` expensive would have already cost more in upstream forwarding; we don't bound body size separately for the extractor.
- The in-memory cache is loaded under a write lock on first miss exactly like `endpointRouter` — concurrent first-call requests serialize on the load and then proceed.

## Migration ordering

Migration `020_project_table_and_request_column.sql` adds the `project` table AND `request.project_id` together (one goose step, single backfill-free up). Down drops the column then the table.

Goose runs migrations on every startup. No backfill helper is needed — historical requests retain `project_id = NULL`.

## CLAUDE.md update

`Architecture → Database Schema` line that lists tables changes from "Nine tables" to "Ten tables" and adds `project` to the enumeration. A new bullet under `Key Patterns` documents the project router invalidation rule:

> **Project matching**: extracts candidate paths from request bodies via the fixed regex set in `pkg/server/project_extractor.go`, looks them up in `Server.projectRouter` (in-memory longest-prefix cache, mirrors `endpointRouter`). Any mutation of the `project` table MUST call `Server.projectRouter.Invalidate()`. `first_seen_at` / `last_seen_at` are updated synchronously when a request matches.

Sidebar enumeration in `dashboard/src/components/AppSidebar.vue` and `pageMeta` map in `dashboard/src/App.vue` get a `projects` entry.

## Out of scope

- Auto-creating projects from observed paths.
- Cross-project path uniqueness validation.
- Backfilling `project_id` for historical requests.
- Filter UI on the overview / charts pages.
