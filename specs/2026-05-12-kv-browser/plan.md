# KV Browser — Execution Plan

## Step 1: Extend the KV Store interface

**Files:**
- `pkg/kv/store.go` — add `KvEntry` struct, `ScanEntriesResult` struct, and `ScanEntries` method to `Store` interface
- `pkg/kv/redis.go` — implement `ScanEntries` using a single pipeline: `SCAN` to get keys, then `MGET` + batched `TTL` commands pipelined together
- `pkg/kv/memory.go` — implement `ScanEntries` by iterating `cache.Items()` with glob matching; return all entries at once (`NextCursor=0`)

## Step 2: Wire KV store onto Server struct

**Files:**
- `pkg/server/server.go` — add `kvStore kv.Store` field to `Server` struct; assign it in `NewServer`

## Step 3: Add contract types

**Files:**
- `pkg/contract/kv.go` — new file: `KvEntryView`, `ListKvEntriesRequest`, `ListKvEntriesResponse`, `GetKvEntryRequest`, `GetKvEntryResponse`, `UpsertKvEntryRequest`, `UpsertKvEntryResponse`, `DeleteKvEntryRequest`, and Huma `Operation*` constants

## Step 4: Add server handlers

**Files:**
- `pkg/server/handle_kv.go` — new file: `handleListKvEntries` (calls `ScanEntries`, maps to `KvEntryView`), `handleGetKvEntry` (Get+TTL), `handleUpsertKvEntry`, `handleDeleteKvEntry`
- `pkg/server/server.go` — register the 4 new operations in `registerOperations()`

## Step 5: Regenerate OpenAPI spec

**Commands:**
- `mise run openapi` → writes `openapi.yaml`

## Step 6: Regenerate TypeScript types

**Commands:**
- `pnpm --dir dashboard generate-openapi` → updates `dashboard/src/openapi-types.d.ts`

## Step 7: Add API layer

**Files:**
- `dashboard/src/api/index.ts` — add `KvEntryView` type export
- `dashboard/src/api/queryKeys.ts` — add `kv: { all, detail(key) }` entries
- `dashboard/src/api/client.ts` — add `listKvEntries()`, `getKvEntry()`, `upsertKvEntry()`, `deleteKvEntry()`, `invalidateKv()`

## Step 8: Add KvForm.vue

**Files:**
- `dashboard/src/components/KvForm.vue` — side panel: key input (readonly on edit), value CodeEditor, TTL number input, JSON validation on blur

## Step 9: Add KvView.vue

**Files:**
- `dashboard/src/views/KvView.vue` — table with Key, Value (truncated), TTL, actions; prefix filter input; "load more" pagination; create/edit/delete flows

## Step 10: Wire routing and navigation

**Files:**
- `dashboard/src/router/index.ts` — add `/kv` route
- `dashboard/src/App.vue` — add `kv` to `pageMeta`
- `dashboard/src/components/AppSidebar.vue` — add `kv` nav item with `db` icon

## Step 11: Build check

**Commands:**
- `mise run openapi`
- `pnpm --dir dashboard generate-openapi`
- `pnpm --dir dashboard type-check`
