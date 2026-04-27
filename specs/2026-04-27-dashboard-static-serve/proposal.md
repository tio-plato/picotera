# Proposal — Serve dashboard SPA from the picotera binary

Today the picotera Go binary serves only:

1. The Huma management API at `/api/picotera/*`
2. A catch-all LLM gateway proxy mounted at `/`

The Vue dashboard at `dashboard/` builds to `dashboard/dist/` but is not served by the backend. To use it, you must run `pnpm --dir dashboard dev` (a separate Vite server with a proxy to `:9898`).

## Goal

Make the picotera binary embed and serve the dashboard SPA, so the production binary is a single self-contained artifact: management API + LLM gateway + dashboard.

## Constraints from the user

- **Embed location** — copy assets into `pkg/server/static/dist/` and embed via `//go:embed`. Self-contained inside the server package.
- **No build-script changes for now** — do NOT touch `mise.toml` tasks, no Makefile, no generate directives. Production assembly (Dockerfile / nix build) will be wired up separately later. In dev, devs build the dashboard manually when they want it bundled.
- **Gateway stays at `/`, no compromise.** The LLM gateway handler MUST remain mounted at root and continue receiving every request that doesn't match the management API. Only when the gateway has determined the request matches *no* configured LLM endpoint should it fall back to the dashboard static handler.
- **No reference to other internal projects** in design docs.

## In scope (added during review)

- **Compression** — gzip and brotli for the dashboard bundle. The Vue+Vite output is ~200KB JS+CSS uncompressed; serving raw is wasteful when assets are immutable and known at compile time.

## Out of scope

- Build-time copying of `dashboard/dist/` into `pkg/server/static/dist/` — the empty embed dir + `.gitkeep` placeholder is sufficient for now; production build will copy.
- Any changes to the dashboard's Vite config (it already builds with `/` as base, which matches root mount).
- Authentication / access control on the dashboard.
