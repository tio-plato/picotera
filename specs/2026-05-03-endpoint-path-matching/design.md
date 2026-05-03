# Design — Endpoint Path Matching with Variables

## Goal

Turn `endpoint.path` from an exact-string key into a pattern that can contain `{name}` placeholders (e.g. `/v1beta/models/{model}:generateContent`). Expose the matched variables to model extraction, upstream URL rewriting, and JS hooks. Back the matcher with a lazily-loaded in-memory cache that the endpoint CRUD handlers invalidate on write.

## Pattern Semantics

- `{name}` placeholders match any non-empty string, **including `/`**. Compiled to the regex fragment `(.+?)` (non-greedy) so trailing literal characters (`:generateContent`) still bind.
- The compiled regex is anchored at both ends (`^...$`).
- Literal characters are `regexp.QuoteMeta`'d.
- A path with no `{}` compiles to a literal-only regex and behaves identically to the old exact-match flow.
- Duplicate variable names in one path are rejected at compile time.
- Variable names must match `[A-Za-z_][A-Za-z0-9_]*` (kept simple).

### Tie-breaking: "most-specific wins"

When multiple compiled patterns match an incoming request path, pick the one with the largest **literal length** — the count of characters in the path that are outside `{}` tokens. An exact-string endpoint (literal length == len(path)) always beats a patterned one that could also match. Ties (same literal length, rare) are broken by lexicographic order of the raw path for determinism.

## In-Memory Router + Cache

New file: `pkg/server/endpoint_router.go`.

```go
type compiledEndpoint struct {
    endpoint   db.Endpoint
    re         *regexp.Regexp
    varNames   []string
    literalLen int
}

type endpointRouter struct {
    queries *db.Queries
    mu      sync.RWMutex
    entries []compiledEndpoint // pre-sorted by literalLen desc, then path asc
    loaded  bool
}
```

Methods:

- `Match(ctx, path) (db.Endpoint, map[string]string, bool, error)` — ensures loaded (under write lock if not), then iterates `entries` under a read lock; returns the first match. `pathVars` is `nil` when the endpoint has no variables.
- `Invalidate()` — flips `loaded=false` and drops `entries` under the write lock. The next `Match` triggers a reload.
- `load(ctx)` (private) — calls `queries.GetEndpoints`, compiles each, sorts, swaps into place.

Concurrency notes:

- Read path: one `RLock` covering both the loaded check (fast path) and the iteration.
- Reload path: double-checked `Lock` → `load` → downgrade. A failed load leaves `loaded=false`, so the next call retries. The error is propagated to the caller; the gateway surfaces it as `500 INTERNAL_ERROR`.
- Cache state is **process-local**. Multi-replica deployments will see up to one request of staleness per replica after a CRUD write until each replica's next call triggers a reload; this is acceptable for the admin-edit use case.

### Invalidation contract

The cache is invalidated explicitly from the server's own endpoint CRUD handlers — **never** from outside the process. The contract is written in:

- A top-of-file doc comment on `endpoint_router.go`.
- Inline comments at the two call sites (`handleUpsertEndpoint`, `handleDeleteEndpoint`).
- A new paragraph in `CLAUDE.md` under "Key Patterns".

If a future contributor adds another writer to the `endpoint` table, they must call `Server.endpointRouter.Invalidate()` at the same site.

## Wiring Changes

### `Server` struct (`pkg/server/server.go`)

Add `endpointRouter *endpointRouter`; construct it in `NewServer` with the shared `*db.Queries`.

### `resolveEndpoint` (`pkg/server/gateway_helpers.go`)

Signature becomes:

```go
func (s *Server) resolveEndpoint(ctx context.Context, path string) (db.Endpoint, map[string]string, error)
```

Internally delegates to `s.endpointRouter.Match`. Returns the same `gatewayError` codes as today (`RouteNotFound` for misses, `InternalError` for DB/compile errors). The `isRouteNotFound` + `looksLikeBrowserNav` fall-through to the dashboard SPA is preserved.

### Model extraction (`extractModel`)

New signature:

```go
func extractModel(body []byte, modelPath string, pathVars map[string]string) (string, error)
```

Behavior: if `modelPath` has the explicit `{name}` form (matches `^\{([A-Za-z_][A-Za-z0-9_]*)\}$`), look up `name` in `pathVars` and require a non-empty value — no body fallback, since the operator explicitly asked for a path variable. Otherwise, evaluate as gjson against the body (current behavior).

This keeps the "priority path variables" intent while making the config readable: a Gemini-style endpoint sets `modelPath = "{model}"`; an OpenAI-style endpoint keeps `modelPath = "model"`.

### Upstream URL substitution (`buildUpstreamRequest`)

Accepts `pathVars` and, before constructing the request, replaces every `{name}` occurrence in `upstreamURL` with `pathVars[name]` verbatim (no re-encoding; the value came from the raw client URL path and is already percent-encoded as the client intended).

After substitution, if any `{...}` token remains, return an error so misconfiguration surfaces as a retryable upstream failure rather than a silent 404 from the provider.

If the endpoint has no variables and the upstream URL has no `{...}`, this is a no-op.

### JS hook exposure

`jsx.RequestShape` gains `PathVars map[string]string \`json:"pathVars,omitempty"\``. `serializeClientRequest` fills it from the resolved `pathVars`. All four hooks that see `RequestShape` (`sortProviders`, `rewriteModel`, `beforeRequest`, `rewriteRequest`) get it transparently.

No change to `Endpoint` context shape (the JS side still sees the raw `db.Endpoint` struct).

## Out of Scope

- Exposing `pathVars` over the management REST API (they're a per-request artifact, not a configurable resource).
- Re-encoding variable values for the upstream URL. If this becomes a problem we'll revisit with a targeted flag.
- Cross-replica cache invalidation (LISTEN/NOTIFY, pub/sub). Not needed at current scale.
- Dashboard UI changes. `EndpointForm.vue` already accepts arbitrary path strings; operators can type `{model}` and it round-trips through the existing CRUD.

## Third-Party Libraries

None. Uses only `regexp` and `sync` from the standard library.
