# KV Browser — Design

## Overview

Add a management UI for inspecting and mutating entries in the KeyDB-backed KV store. The feature spans three layers: the `pkg/kv` Store interface (add entry scanning), the Huma REST API (new endpoints), and the Vue dashboard (new view + form).

---

## Backend

### 1. Store interface extension

Add `KvEntry` and `ScanEntries` to `kv.Store`:

```go
type KvEntry struct {
    Key   string
    Value string
    TTL   int64 // -1 = no expiry, >= 0 = seconds remaining
}

type ScanEntriesResult struct {
    Entries    []KvEntry
    NextCursor uint64
}

// ScanEntries returns entries (key + value + TTL) matching a Redis-style
// glob pattern. cursor=0 starts a new scan. count is a hint for batch size.
// Returns entries and the next cursor (0 = complete).
ScanEntries(ctx context.Context, pattern string, cursor uint64, count int64) (ScanEntriesResult, error)
```

**RedisStore**: single pipeline per call — `SCAN` to get keys, then `MGET` to fetch all values and a batch of `TTL` commands, all pipelined in one round-trip. Filters out keys that expired between `SCAN` and `MGET` (redis.Nil).

**MemoryStore**: iterates `cache.Items()`, applies glob filter, returns all matching entries in one shot (`NextCursor=0`).

### 2. Server wiring

The `Server` struct already creates a `kv.Store` in `NewServer` (passed to jsxEngine). Add `kvStore kv.Store` to the struct so handlers can access it.

### 3. API endpoints

All under `/api/picotera/kv`:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/kv` | List entries (cursor pagination, optional `pattern` query, default `*`) |
| `GET` | `/kv/{key}` | Get single entry (value + TTL) |
| `PUT` | `/kv/{key}` | Create or update an entry |
| `POST` | `/kv/delete` | Delete an entry |

The list endpoint calls `ScanEntries` once — the store returns key, value, and TTL together. No N+1 problem.

### 4. Contract types (`pkg/contract/kv.go`)

```go
type KvEntryView struct {
    Key   string `json:"key"`
    Value string `json:"value"`
    TTL   int64  `json:"ttl"` // -1 = no expiry, >= 0 = seconds remaining
}

type ListKvEntriesResponse struct {
    Body       []KvEntryView `json:"body"`
    NextCursor string        `json:"nextCursor,omitempty"`
}

type KvMutateBody struct {
    Value      string `json:"value"`
    TTLSeconds *int64 `json:"ttlSeconds,omitempty"` // nil = no expiry
}
```

### 5. Icon

Reuse the existing `db` icon name (already maps to `IconDatabase`) for the sidebar nav item.

---

## Frontend

### 1. Navigation

- **Sidebar**: add `{ name: 'kv', label: 'KV', icon: 'db' }` entry (after `scripts`).
- **Router**: `{ path: '/kv', name: 'kv', component: () => import('@/views/KvView.vue') }`.
- **pageMeta**: `kv: { title: 'KV 存储', hint: '今天存了些什么' }`.

### 2. KvView.vue

Layout follows the ScriptsView pattern:
- Header row: count badge + prefix search input + "新增" button.
- `DataCard` with `DataTable`: columns for Key, Value (truncated, monospace), TTL (human-readable), and action buttons (edit, delete).
- "Load more" button at the bottom when `nextCursor` is present.
- Value column shows first ~80 chars of the value; click row to edit.

TTL display:
- `-1` → Tag "永不过期"
- `>= 0` → formatted as `Xm Ys` or just `Xs`

### 3. KvForm.vue

Side panel form:
- Key: `Input` (readonly on edit, editable on create).
- Value: `CodeEditor` with JSON validation on blur (soft warning, not blocking).
- TTL: `Input` (number, seconds). Empty or 0 = no expiry.
- Save button calls PUT `/kv/{key}`.

### 4. API client layer

- `api/index.ts`: export `KvEntryView`.
- `api/queryKeys.ts`: add `kv: { all, detail(key) }`.
- `api/client.ts`: add `listKvEntries`, `getKvEntry`, `upsertKvEntry`, `deleteKvEntry`, `invalidateKv`.

---

## Data flow

```
Dashboard (KvView)
  ↓ useQuery(queryKeys.kv.all)
  ↓ listKvEntries(pattern?, cursor?)
  → GET /api/picotera/kv?pattern=*&cursor=0
    → Server.handleListKvEntries
      → kvStore.ScanEntries(ctx, pattern, cursor, 100)
        → [Redis: single pipeline — SCAN + MGET + batched TTL]
        → [Memory: iterate cache items]
      → return []KvEntryView + nextCursor

Dashboard (KvForm)
  ↓ useMutation → upsertKvEntry(key, { value, ttlSeconds? })
  → PUT /api/picotera/kv/{key}
    → kvStore.Set / kvStore.SetEx
```
