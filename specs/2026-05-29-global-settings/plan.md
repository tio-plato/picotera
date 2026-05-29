# Plan: Global Settings

## Step 1: Database Migration

Create `db/migrations/029_global_settings_table.sql`:
- Create `global_setting` table with `key` (TEXT PK), `value` (JSONB NOT NULL), `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now()).
- Down migration drops the table.

## Step 2: SQLC Queries

Create `db/queries/global_setting.sql` with:
- `ListGlobalSettings :many` — SELECT all rows ordered by key.
- `GetGlobalSetting :one` — SELECT by key.
- `UpsertGlobalSetting :one` — INSERT ON CONFLICT (key) DO UPDATE SET value, updated_at.
- `DeleteGlobalSetting :exec` — DELETE by key.

Run `sqlc generate` to produce `pkg/db/` code.

## Step 3: Contract Types

Create `pkg/contract/global_setting.go`:
- `GlobalSettingView` struct with `Key string` and `Value json.RawMessage`.
- `ToGlobalSettingView` conversion function.
- Request/response types: `ListGlobalSettingsResponse`, `GetGlobalSettingRequest`, `GetGlobalSettingResponse`, `UpsertGlobalSettingRequest`, `UpsertGlobalSettingResponse`, `DeleteGlobalSettingRequest`.
- Operation declarations: `OperationListGlobalSettings`, `OperationGetGlobalSetting`, `OperationUpsertGlobalSetting`, `OperationDeleteGlobalSetting`.

## Step 4: Server Handlers

Create `pkg/server/handle_global_setting.go`:
- `handleListGlobalSettings` — calls `ListGlobalSettings`, converts to views.
- `handleGetGlobalSetting` — calls `GetGlobalSetting`, handles pgx.ErrNoRows → 404.
- `handleUpsertGlobalSetting` — validates key non-empty, calls `UpsertGlobalSetting`.
- `handleDeleteGlobalSetting` — calls `DeleteGlobalSetting`, handles pgx.ErrNoRows → 404.

Register all four operations in `server.go` `registerOperations()`.

## Step 5: Regenerate OpenAPI & TypeScript Types

- Run `mise run openapi` to update `openapi.yaml`.
- Run `pnpm --dir dashboard generate-openapi` to update `dashboard/src/openapi-types.d.ts`.

## Step 6: Dashboard API Layer

In `dashboard/src/api/client.ts`:
- Add fetchers: `fetchGlobalSettings`, `fetchGlobalSetting`, `upsertGlobalSetting`, `deleteGlobalSetting`.
- Add invalidation helper: `invalidateGlobalSettings`.

In `dashboard/src/api/queryKeys.ts`:
- Add `globalSettings` entry to `queryKeys`.

## Step 7: Dashboard Composable

Create `dashboard/src/composables/useAppTitle.ts`:
- Fetches `GET /api/picotera/settings/app.title` via vue-query.
- Returns reactive `appTitle` ref (defaults to `"PicoTera"`).
- Watches the ref and sets `document.title`.

## Step 8: Dashboard Views

Create `dashboard/src/views/SettingsView.vue`:
- A form with an `Input` field for the application title.
- Fetches current title via `useAppTitle` or direct query.
- Saves via `upsertGlobalSetting` mutation.
- Shows success feedback on save.

## Step 9: Dashboard Router & Shell

In `dashboard/src/router/index.ts`:
- Add route: `{ path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') }`.

In `dashboard/src/App.vue`:
- Add `settings` entry to `pageMeta` map.

In `dashboard/src/components/AppSidebar.vue`:
- Import and use `useAppTitle` composable.
- Replace hardcoded "PicoTera" with reactive `appTitle`.
- Add "设置" nav item with `settings` icon.

## Step 10: Update Browser Title

In `dashboard/src/App.vue`:
- Import `useAppTitle`.
- Watch `appTitle` and set `document.title`.
