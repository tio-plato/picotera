# Design — Dashboard static-serve from picotera binary

## Summary

Add a new internal package `pkg/server/static` that embeds the dashboard SPA via `//go:embed`. Wire it into the LLM gateway handler as a fallback: when the gateway determines the incoming request does not match any configured LLM endpoint *and* the request is a safe-to-serve method (GET/HEAD), delegate to the static handler instead of returning a `route not found` gateway error.

The LLM gateway remains the catch-all at `/`. Routing precedence:

```
1. /api/picotera/*        → Huma management API (chi router, registered first)
2. /  (gatewayHandler)    → LLM proxy if path matches a configured endpoint
3. fallback inside #2     → static SPA handler if request looks like a browser
                             navigation (GET/HEAD + Accept allows HTML)
                             AND no endpoint matched
4. otherwise              → existing 404 gateway error
```

### What "looks like a browser navigation" means

The fallback fires only when ALL of:

- Method is `GET` or `HEAD`
- `Accept` header is empty, contains `text/html`, or contains `*/*`

An API client doing `POST /v1/chat/completions` with `Accept: application/json` to a misconfigured endpoint MUST get the JSON gateway 404, not the SPA. The Accept gate prevents that even for the GET/HEAD case where a tool like `httpie` or a custom script might land on an unknown path.

## Package layout

```
pkg/server/static/
├── embed.go         # //go:embed all:dist  →  var distFS embed.FS
├── handler.go       # Handler() http.Handler with SPA fallback semantics
└── dist/
    └── index.html   # tracked placeholder ("dashboard not built yet" page)
```

`pkg/server/static/dist/` is the embed target. In dev, only the placeholder lives there. Production builds (Dockerfile / nix) will overwrite the directory with the output of `pnpm --dir dashboard build` before invoking `go build`.

### Why a tracked placeholder

`//go:embed all:dist` requires the directory to exist with at least one file at compile time. Without a tracked file, `go build` fails on a clean checkout that hasn't run the dashboard build. The placeholder:

- keeps `go build` working with no extra steps in dev
- visibly surfaces the "dashboard not bundled" state when a dev hits the URL
- is overwritten — not deleted — during production assembly, so no special build glue is needed

### .gitignore for the embed directory

Track only the placeholder. Add a **local** gitignore inside the embed directory at `pkg/server/static/dist/.gitignore`:

```
/*
!/.gitignore
!/index.html
```

Keeping the rules co-located with the directory they govern (rather than in the repo-root `.gitignore`) makes the intent obvious to anyone browsing the static package and avoids repo-wide rule sprawl. `pnpm build` outputs land in the embed dir during local testing without polluting git.

## Static handler behavior

`static.Handler()` returns an `http.Handler` that:

1. **Method gate** — reject anything that is not `GET` or `HEAD` with `405 Method Not Allowed`. (In practice the gateway will only call us for GET/HEAD; the gate is defensive.)
2. **Path resolution** — clean the URL path; map `/` → `/index.html`. Reject paths containing `..` (defense-in-depth, though `embed.FS` already disallows traversal).
3. **File lookup** —
   - If the file exists in the precomputed asset table (see Compression below) → serve it with appropriate `Content-Type`, `Cache-Control`, and the best-matching encoding for the client's `Accept-Encoding`.
   - If the file does NOT exist → serve `index.html` with `200 OK` so client-side Vue Router can resolve the path. This is the SPA fallback.
4. **Cache headers** —
   - `index.html` and the SPA fallback response: `Cache-Control: no-cache` (immediate pickup of new dashboard versions).
   - `assets/*` (Vite emits content-hashed filenames): `Cache-Control: public, max-age=31536000, immutable`.
   - Everything else (favicon, root-level files): `Cache-Control: no-cache`.

## Compression

Vite outputs immutable, content-hashed assets that don't change for the lifetime of a binary. We exploit that: **precompress every embedded asset once at process startup** and store the results in memory. Per-request work is then just header negotiation and a `bytes.Reader` write — no per-request CPU and no per-request allocation.

### Algorithms

- **gzip** — via stdlib `compress/gzip` at level 9 (best). Universally supported.
- **brotli** — via `github.com/andybalholm/brotli` at level 11 (best, slow compress but we only do it once). Pure Go, no cgo. ~15-20% smaller than gzip for text bundles.

### Asset table

At package init (or first call to `Handler()`), walk the embedded `dist/` filesystem and build an in-memory map:

```go
type asset struct {
    contentType string
    raw         []byte
    gzip        []byte // nil if not worth compressing
    brotli      []byte // nil if not worth compressing
    etag        string // strong ETag derived from raw bytes
}

var assets map[string]*asset
```

For each file:

- Read raw bytes.
- Determine content type via `mime.TypeByExtension` with a small override table for `.js`, `.css`, `.svg`, `.json`.
- Compress with gzip + brotli **only if** content type is text-y (`text/*`, `application/javascript`, `application/json`, `application/wasm`, `image/svg+xml`) AND raw size > 1024 bytes. For PNG/JPG/WOFF2 etc., compression is wasted.
- If the compressed variant is not at least 10% smaller than raw, drop it (compressing further hurts).
- ETag is `"` + first 16 hex chars of `sha256(raw)` + `"`.

### Per-request negotiation

For each request:

1. Resolve target path → `*asset` (or fall back to the SPA index).
2. Conditional GET: if `If-None-Match` matches the asset's ETag, return `304 Not Modified` with no body.
3. Parse `Accept-Encoding`. Prefer `br` if the client accepts it AND `asset.brotli != nil`. Else prefer `gzip` if accepted AND `asset.gzip != nil`. Else use raw.
4. Set headers:
   - `Content-Type: <asset.contentType>`
   - `Content-Length: <chosen body length>`
   - `Content-Encoding: br` or `gzip` if applicable
   - `Vary: Accept-Encoding` (always — even when serving raw, because the *response would differ* under a different Accept-Encoding)
   - `ETag: <asset.etag>`
   - `Cache-Control` per the rule above
5. Write body (skip body for HEAD).

`http.ServeContent` is NOT used — we want full control over `Content-Encoding` and to avoid stdlib's `If-Modified-Since` quirks since we're using ETag exclusively.

### Memory cost

For the current dashboard (~357KB raw across all assets), the in-memory table with gzip+brotli copies is roughly 357KB + 120KB + 100KB ≈ **600KB of heap, one-time**. Negligible. Recompression only happens on process restart.

### What about the placeholder?

The placeholder `index.html` is small and gets the same treatment — it'll have a gzip variant. No special-casing.

## Wiring into the gateway handler

The gateway handler (`pkg/server/handle_gateway.go`) currently:

1. Reads request body.
2. Inserts a `meta` request row in the DB and uploads the request artifact to S3.
3. Calls `resolveEndpoint(path)`. If it returns `pgx.ErrNoRows` (wrapped as a `gatewayError{404, "route not found"}`), the handler fails the meta request and returns a JSON 404.

The change: **lift the endpoint lookup above the meta-row insert**, and treat *all* failures to match an LLM endpoint as off-pipeline events that don't get logged in the `request` table.

```go
// New flow at the top of ServeHTTP:
endpoint, err := h.resolveEndpoint(r.Context(), r.URL.Path)
if err != nil {
    if isRouteNotFound(err) && looksLikeBrowserNav(r) {
        h.staticHandler.ServeHTTP(w, r)
        return
    }
    handleGatewayErr(w, err)
    return
}

// existing path: read body, insert meta, run hooks, retry loop, etc.
```

`looksLikeBrowserNav` returns true iff method ∈ {GET, HEAD} AND `Accept` header is empty / contains `text/html` / contains `*/*`.

### Why no meta row for unmatched paths

The `request` table records LLM gateway traffic — provider, model, tokens, TTFT. An unmatched path has none of those: every numeric column would be NULL and the row carries no analytic value. Worse, in production it would absorb whatever a public-facing port catches: scanners, fat-fingered curls, monitoring probes.

This is symmetric with the existing body-read-error path at the top of `ServeHTTP`, which also writes a JSON error directly without a meta row. The author of that path had the same instinct: errors that prevent us from knowing *which* LLM endpoint this would have been don't belong in the request log.

Real DB errors during endpoint resolution (not `pgx.ErrNoRows`, but a connection drop, etc.) are surfaced via `logx.Error` inside `resolveEndpoint` so they remain visible operationally. They are rare and warrant alerting, not a row.

### Audit trail considerations

If you later need "log every 404 hitting the gateway port," the right place is an HTTP access-log middleware on the chi router (or a reverse proxy), not the `request` table. Keep the `request` table semantically narrow: one row per attempted LLM call.

### Why the method + Accept gate matters

In every case below, **no row is inserted into the `request` table** — the gate only decides whether the response is the dashboard SPA or a JSON 404.

- `POST /chat/completions` → fails the method gate → JSON 404.
- `GET /v1/messages` from `curl` with default `Accept: */*` → Accept allows HTML (`*/*` matches), so this *does* fall through to the SPA. Harmless: an unconfigured LLM path responding with the dashboard's HTML signals to the user (when they open it in a browser) that they're at the wrong URL.
- `GET /v1/messages` from a properly-configured Anthropic SDK with `Accept: application/json` → fails the Accept gate → JSON 404 (the SDK gets the structured error it expects).
- `HEAD /v1/messages` from a healthcheck script with `Accept: */*` → falls through to SPA HEAD (200 with index.html headers, no body). Harmless; healthchecks against an unconfigured endpoint should not have been pointed there in the first place.

## Server struct change

`Server` gains a `staticHandler http.Handler` field, populated in `NewServer` with `static.Handler()`. The `gatewayHandler` reads it via its embedded `*Server`.

## Error refactor

A small helper in `pkg/server/gateway_helpers.go`:

```go
func isRouteNotFound(err error) bool {
    var gw *gatewayError
    return errors.As(err, &gw) && gw.code == errorx.RouteNotFound.Error()
}
```

To classify the exact error from `resolveEndpoint` without touching its signature.

## What does NOT change

- `mise.toml` — no new tasks. Devs run `pnpm --dir dashboard build` and `cp -r dashboard/dist/* pkg/server/static/dist/` manually if they want the built dashboard in the binary.
- Dashboard `vite.config.ts` — base remains `/`. The dev server's `/api` and `/v1` proxies to `:9898` still work.
- The Huma API mount at `/api/picotera/*`.
- The chi `Mount("/", &gatewayHandler{s})` registration.
- Database schema, migrations, sqlc queries.

## Risks / trade-offs

- **Body-read reorder risk**: we currently read the body before doing anything else. Lifting endpoint lookup above the body read is safe because `resolveEndpoint` only consumes `r.URL.Path` and `r.Context()`. No body needed.
- **Empty-dist case**: if a dev hits the URL before building the dashboard, they get a placeholder HTML page that explains how to populate it. Better than 404.
- **Cache headers on placeholder**: the placeholder is `index.html`, served as `no-cache`, so it'll be replaced immediately once a real dashboard is built and served.

## Out of scope

- Per-build asset manifest, integrity hashes, or CSP nonces.
- Auth/access control on the dashboard.
- Build automation. Dockerfile / nix build will be a separate spec.
- Zstd encoding (low browser support relative to brotli).
