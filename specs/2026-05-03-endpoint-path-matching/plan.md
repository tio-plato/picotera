# Plan — Endpoint Path Matching with Variables

## Step 1 — Router + cache skeleton

Create `pkg/server/endpoint_router.go`:

- `compiledEndpoint{ endpoint db.Endpoint; re *regexp.Regexp; varNames []string; literalLen int }`.
- `compilePattern(path string) (*regexp.Regexp, []string, int, error)`:
  - Scan `path` for `{name}` tokens using a regex like `\{([A-Za-z_][A-Za-z0-9_]*)\}`.
  - Reject duplicate names.
  - Build the compiled pattern: `^` + alternating `regexp.QuoteMeta(literal)` + `(.+?)` + `$`.
  - `literalLen` = sum of `len(literal)` segments.
- `type endpointRouter struct { queries *db.Queries; mu sync.RWMutex; entries []compiledEndpoint; loaded bool }`.
- `newEndpointRouter(q *db.Queries) *endpointRouter`.
- `(*endpointRouter).Match(ctx, path) (db.Endpoint, map[string]string, bool, error)`:
  - Fast path: `RLock`; if `loaded`, iterate, return match + extracted vars.
  - Slow path: upgrade to `Lock`; recheck; call `load(ctx)`; unlock; retry under `RLock`.
  - Vars map is `nil` when `len(varNames)==0`.
- `(*endpointRouter).Invalidate()`: `Lock`; `entries=nil; loaded=false`; `Unlock`.
- `load(ctx)`: call `queries.GetEndpoints`, compile each, sort by `literalLen` desc then `path` asc, swap in.
- Top-of-file doc comment: "Endpoint matching is cached in memory. Invalidate via `Invalidate()` on any mutation of the `endpoint` table. Do not bypass this router; `GetEndpointByPath` is only for exact-path validation (e.g. `handle_provider_endpoint.go`)."

## Step 2 — Wire the router into `Server`

In `pkg/server/server.go`:

- Add `endpointRouter *endpointRouter` to `Server`.
- In `NewServer`, after `queries := db.New(conn)`, construct `server.endpointRouter = newEndpointRouter(queries)` (before `registerOperations`/`registerEndpoints`).

## Step 3 — Update `resolveEndpoint`

In `pkg/server/gateway_helpers.go`:

- Change signature to `(db.Endpoint, map[string]string, error)`.
- Replace the `GetEndpointByPath` call with `s.endpointRouter.Match(ctx, path)`.
- On `ok == false`: return the existing `RouteNotFound` `gatewayError`.
- On compile/load error: log as today and return the `InternalError` `gatewayError`.

## Step 4 — Thread `pathVars` through the gateway

In `pkg/server/handle_gateway.go`:

- Capture `pathVars` from `resolveEndpoint`.
- Pass `pathVars` to `extractModel(body, endpoint.ModelPath, pathVars)` at step 5.
- Pass `pathVars` to `buildUpstreamRequest` at the retry loop (new arg between `upstreamURL` and `upstreamModel`, or as a trailing arg — prefer trailing to minimize diff).
- In `serializeClientRequest`, include `pathVars` in the returned `RequestShape`.

## Step 5 — `extractModel` branch

In `pkg/server/gateway_helpers.go`:

- New signature `extractModel(body []byte, modelPath string, pathVars map[string]string) (string, error)`.
- Add a package-level `var pathVarRe = regexp.MustCompile(\`^\{([A-Za-z_][A-Za-z0-9_]*)\}$\`)`.
- If `modelPath` matches `pathVarRe`: look up `pathVars[name]`; if empty, return `ModelNotFound` gateway error ("model variable %q not set").
- Otherwise keep current gjson behavior.

## Step 6 — Upstream URL substitution

In `pkg/server/gateway_helpers.go`:

- Add a helper `substitutePathVars(url string, vars map[string]string) (string, error)` that replaces each `{name}` with `vars[name]` and returns an error if any `{...}` token remains after substitution.
- Extend `buildUpstreamRequest` to call this helper on `upstreamURL` before `http.NewRequestWithContext`. Surface substitution errors as the `berr` value; the retry loop already records them as failed attempts.

## Step 7 — JS RequestShape field

In `pkg/jsx/types.go`:

- Add `PathVars map[string]string \`json:"pathVars,omitempty"\`` to `RequestShape`.

Update `serializeClientRequest` in `pkg/server/gateway_helpers.go` to set it.

Add a short comment on the new field explaining that it is populated from the endpoint path match.

## Step 8 — Cache invalidation on CRUD

In `pkg/server/handle_endpoint.go`:

- After a successful `handleUpsertEndpoint`, call `s.endpointRouter.Invalidate()` with an inline comment: `// endpoint router caches compiled paths; invalidate so the next request reloads.`
- Same for `handleDeleteEndpoint`.

No other endpoint writers exist; `handle_provider_endpoint.go` only touches `provider_endpoint`, which does not affect routing.

## Step 9 — Documentation

Update `CLAUDE.md`:

- Under "Key Patterns", add a bullet:
  > **Endpoint matching**: request paths are matched against `endpoint.path` patterns (which may contain `{name}` placeholders matching any non-empty string, including `/`). The matcher is an in-memory cache (`pkg/server/endpoint_router.go`) loaded lazily from `GetEndpoints` and sorted by literal-character specificity. Any mutation of the `endpoint` table **must** call `Server.endpointRouter.Invalidate()`. Do not reintroduce `GetEndpointByPath` for gateway routing — it only remains for exact-path validation in `handle_provider_endpoint.go`.

## Step 10 — Manual verification

No Go tests exist yet in this repo; verification is manual.

1. `go build ./...` must succeed.
2. Start `docker compose up -d` and `mise run server`.
3. Upsert an endpoint with path `/v1beta/models/{model}:generateContent`, `modelPath = "{model}"`, and a provider with `upstream_url = https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent`.
4. POST to `/v1beta/models/gemini-2.5-pro:generateContent` — confirm the gateway resolves the endpoint, extracts `model=gemini-2.5-pro` from the path, and substitutes the upstream URL.
5. POST to the same path with an unknown trailing literal — confirm `404 ROUTE_NOT_FOUND`.
6. Edit the endpoint through the dashboard; confirm subsequent requests reflect the change (cache invalidated).
