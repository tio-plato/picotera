# Plan — Dashboard static-serve from picotera binary

## Step 1 — Create the static package skeleton

Create `pkg/server/static/`:

- `pkg/server/static/dist/index.html` — placeholder HTML page. Minimal valid HTML, body explains:
  > Dashboard not built into this binary yet. Run `pnpm --dir dashboard build && cp -r dashboard/dist/* pkg/server/static/dist/`, then rebuild.
  No CSS or JS. Self-contained.
- `pkg/server/static/dist/.gitignore` — see Step 6.
- `pkg/server/static/embed.go` — package declaration + `//go:embed all:dist` directive + `var distFS embed.FS`.
- `pkg/server/static/handler.go` — exports `Handler() http.Handler` implementing the spec'd behavior.
- `pkg/server/static/assets.go` — asset table + precompression logic.

## Step 2a — Add brotli dependency

Add `github.com/andybalholm/brotli` to `go.mod`:

```bash
go get github.com/andybalholm/brotli
```

Verify it's pure-Go (no cgo) — the package is.

## Step 2b — Build the asset table

In `pkg/server/static/assets.go`:

1. `type asset struct { contentType string; raw, gzip, brotli []byte; etag string }`.
2. `var assets map[string]*asset` — populated by `init()` or `sync.Once`.
3. Build function:
   - `subFS, _ := fs.Sub(distFS, "dist")`.
   - `fs.WalkDir(subFS, ".", ...)`. For each regular file:
     - Read bytes via `fs.ReadFile`.
     - Determine content type. Use a small override map to force `.js → text/javascript; charset=utf-8`, `.css → text/css; charset=utf-8`, `.svg → image/svg+xml`, `.json → application/json`. Fall back to `mime.TypeByExtension`. If still empty, `application/octet-stream`.
     - Compute ETag: SHA-256 of raw, hex-encode first 16 chars, wrap in `"`...`"`.
     - Decide if compressible: content type starts with `text/` OR is one of `application/javascript`, `application/json`, `application/wasm`, `image/svg+xml`. AND raw size > 1024.
     - If compressible: gzip at level 9 → `gz`. brotli at level 11 → `br`. Drop each variant if it's not at least 10% smaller than raw.
     - Insert into the map keyed on the file path with leading `/` (e.g. `/index.html`, `/assets/index-abc.js`).
4. Expose `lookup(target string) *asset` and `indexAsset() *asset` (returns the precomputed `/index.html` for SPA fallback).

## Step 2c — Implement `static.Handler()`

In `pkg/server/static/handler.go`:

1. `Handler()` returns an `http.HandlerFunc`:
   - Reject methods other than GET/HEAD with `405 Method Not Allowed` and `Allow: GET, HEAD`.
   - Compute target: `path.Clean(r.URL.Path)`; if `/` or empty, use `/index.html`. Reject paths with `..` (defense-in-depth).
   - `a := lookup(target)`. If nil, `a = indexAsset()` (SPA fallback) and force `Cache-Control: no-cache` for the response regardless of path.
   - Conditional GET: if `If-None-Match` matches `a.etag`, write `304 Not Modified` (no body) and return.
   - Pick encoding via `negotiateEncoding(r.Header.Get("Accept-Encoding"), a)`:
     - Returns one of `("", raw)`, `("gzip", a.gzip)`, `("br", a.brotli)`. Prefer `br > gzip > raw` when client allows. Respect `q=0` to exclude an encoding.
   - Set headers: `Content-Type`, `Content-Length`, `ETag`, `Vary: Accept-Encoding`, `Cache-Control` (immutable for `/assets/*`, no-cache otherwise), and `Content-Encoding` if applicable.
   - For HEAD: only write headers, no body.
   - For GET: write the chosen body buffer.

## Step 2d — Cache header rule

In `handler.go`, helper `cacheControl(target string, isFallback bool) string`:

- Fallback (SPA index served for unknown path) → `no-cache`.
- Target starts with `/assets/` → `public, max-age=31536000, immutable`.
- Otherwise → `no-cache`.

## Step 3 — Wire the handler into `Server`

In `pkg/server/server.go`:

1. Add `staticHandler http.Handler` field to `Server` struct.
2. In `NewServer`, populate it: `staticHandler: static.Handler()`. Add the import.
3. Leave `NewHuma()` unchanged — it's for OpenAPI generation only and doesn't need the static handler.

## Step 4 — Add the SPA fallback in the gateway

In `pkg/server/handle_gateway.go`, restructure the top of `gatewayHandler.ServeHTTP`:

1. **Move endpoint resolution to before body read and meta-row insert.** New top of `ServeHTTP`:

   ```go
   endpoint, err := h.resolveEndpoint(r.Context(), r.URL.Path)
   if err != nil {
       if isRouteNotFound(err) && looksLikeBrowserNav(r) {
           h.staticHandler.ServeHTTP(w, r)
           return
       }
       handleGatewayErr(w, err)
       return
   }
   ```

2. **Do NOT log unmatched paths in the `request` table.** Failures to match an LLM endpoint write a JSON error directly. This is symmetric with the existing body-read-error path, which also skips the meta row. The `request` table stays semantically narrow: one row per attempted LLM call.

3. Add a `logx.Error` call inside `resolveEndpoint` for the non-`pgx.ErrNoRows` branch (real DB errors). They are rare, off-pipeline, and need to remain operationally visible since they no longer produce a row.

4. After the early-return block, the rest of `ServeHTTP` continues as today: read body, insert meta with the now-known endpoint, etc.

### `looksLikeBrowserNav` helper

In `pkg/server/gateway_helpers.go`:

```go
func looksLikeBrowserNav(r *http.Request) bool {
    if r.Method != http.MethodGet && r.Method != http.MethodHead {
        return false
    }
    accept := r.Header.Get("Accept")
    if accept == "" {
        return true
    }
    // Accept is a comma-separated list of media-range; we just substring-match
    // against `text/html` and `*/*`. We don't bother with q=0 exclusion — the
    // Accept gate is a coarse classifier, not RFC-7231 negotiation.
    lower := strings.ToLower(accept)
    return strings.Contains(lower, "text/html") || strings.Contains(lower, "*/*")
}
```

## Step 5 — Helper: `isRouteNotFound`

In `pkg/server/gateway_helpers.go`, add:

```go
func isRouteNotFound(err error) bool {
    var gw *gatewayError
    return errors.As(err, &gw) && gw.code == errorx.RouteNotFound.Error()
}
```

## Step 6 — Local `.gitignore` inside the embed dir

Create `pkg/server/static/dist/.gitignore` (NOT the repo-root `.gitignore`):

```
/*
!/.gitignore
!/index.html
```

This lets `cp -r dashboard/dist/* pkg/server/static/dist/` work locally without staging hashed assets, and keeps the rule co-located with the directory it governs.

## Step 7 — Build + smoke test

1. `go build -o picotera ./cmd/picotera` — must succeed with only the placeholder in `dist/`.
2. Boot the binary against the dev Postgres.
3. `curl -i http://localhost:9898/` — expect `200 OK`, HTML body of placeholder, `Cache-Control: no-cache`, `ETag` set, `Vary: Accept-Encoding`.
4. `curl -i http://localhost:9898/some/spa/route` — expect `200 OK`, same placeholder body (SPA fallback).
5. `curl -i -X POST http://localhost:9898/some/spa/route` — expect existing JSON 404 gateway error (NOT the placeholder), confirming the method gate works.
6. `curl -i -H 'Accept: application/json' http://localhost:9898/some/spa/route` — expect existing JSON 404 gateway error (NOT the placeholder), confirming the Accept gate works.
7. `curl -i -H 'Accept-Encoding: gzip' http://localhost:9898/` — expect `Content-Encoding: gzip` and a smaller body. Verify by piping through `gunzip`.
8. `curl -i -H 'Accept-Encoding: br' http://localhost:9898/` — expect `Content-Encoding: br`.
9. `curl -i -H 'If-None-Match: <etag from step 3>' http://localhost:9898/` — expect `304 Not Modified` with no body.
10. `curl -i http://localhost:9898/api/picotera/providers` — expect normal Huma response, confirming API mount is unaffected.
11. Configure an LLM endpoint in DB, send a real `POST /v1/chat/completions` against it — expect normal proxy behavior (regression check).

## Step 8 — Build dashboard locally and re-test

1. `pnpm --dir dashboard build`
2. `find pkg/server/static/dist -mindepth 1 ! -name index.html ! -name .gitignore -delete && cp -r dashboard/dist/. pkg/server/static/dist/` — preserves the placeholder + local `.gitignore`, copies dashboard assets in. (After this `index.html` is overwritten by the real dashboard one, which is fine.)
3. `go build -o picotera ./cmd/picotera`
4. Boot binary, open `http://localhost:9898/` in a browser, confirm dashboard loads.
5. Click into any Vue route (e.g. `/providers`), reload the page — expect the route to resolve via SPA fallback (NOT 404).
6. In devtools network tab, confirm:
   - `/assets/*` responses have `Cache-Control: public, max-age=31536000, immutable`.
   - `Content-Encoding: br` (Chromium/Firefox both support brotli).
   - `Content-Length` is markedly smaller than the raw asset size on disk.
7. After verification, run `git status`. Only `pkg/server/static/dist/index.html` (placeholder, restored from real build) should appear if anything; the `.gitignore` should exclude `assets/`.

## Step 9 — Update documentation

Append a short "Bundling the dashboard" section to `CLAUDE.md` that documents:

- The placeholder + embed strategy.
- That `pkg/server/static/dist/*` (except `index.html`) is gitignored.
- The manual copy command for testing the bundled binary in dev.

Two short paragraphs, max.

## Verification checklist

- [ ] `go build` succeeds on a clean checkout (placeholder only).
- [ ] `GET /` returns the placeholder when no real dashboard is bundled.
- [ ] `GET /some/spa/route` returns SPA fallback (200, index.html).
- [ ] `POST /some/path` still returns the existing gateway 404 JSON.
- [ ] `GET /some/path` with `Accept: application/json` still returns the existing gateway 404 JSON.
- [ ] `Content-Encoding: gzip` and `br` are negotiated correctly via `Accept-Encoding`.
- [ ] `If-None-Match` returns `304` when the ETag matches.
- [ ] `Vary: Accept-Encoding` is set on all asset responses.
- [ ] `GET /api/picotera/providers` is untouched.
- [ ] LLM proxy requests against a configured endpoint still work end-to-end.
- [ ] After populating `dist/`, the real Vue dashboard loads in the browser and client-side routing survives a reload.
- [ ] No new request rows in the DB and no S3 PUTs for ANY unmatched-path traffic — dashboard hits, POST to unknown paths, JSON-Accept GETs to unknown paths (verify via `select count(*) from request` before/after).
- [ ] No changes to `mise.toml`, `vite.config.ts`, migrations, or sqlc queries.

## Files touched

| File | Action |
|---|---|
| `pkg/server/static/embed.go` | new |
| `pkg/server/static/assets.go` | new — asset table + precompression |
| `pkg/server/static/handler.go` | new — handler + Accept-Encoding negotiation |
| `pkg/server/static/dist/index.html` | new (placeholder) |
| `pkg/server/static/dist/.gitignore` | new — local ignore rules |
| `pkg/server/server.go` | add `staticHandler` field, populate in `NewServer` |
| `pkg/server/handle_gateway.go` | reorder top of `ServeHTTP`: hoist endpoint resolution above body read; route unmatched-path failures to SPA (browser nav) or `handleGatewayErr` (everything else) without inserting a meta row |
| `pkg/server/gateway_helpers.go` | add `isRouteNotFound`, `looksLikeBrowserNav`; add `logx.Error` inside `resolveEndpoint` for real DB errors |
| `go.mod`, `go.sum` | add `github.com/andybalholm/brotli` |
| `CLAUDE.md` | document the bundle workflow |
