# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working Conventions

- **No unsolicited compatibility layers.** When planning or making decisions, do not introduce compatibility shims, fallbacks, deprecated-path branches, or any code whose purpose is to preserve old behavior — unless the user explicitly asks for it. Prefer clean replacements that update all call sites. If you genuinely believe a compatibility layer is unavoidable or strongly warranted, **stop and ask the user before writing it**.
- **Fail fast on input; do not be lenient.** Do not add forgiving normalization of user/API input — no silent trimming of whitespace, no case-folding, no coercing empty strings to defaults, no "did you mean" guessing, no accepting near-miss formats. Validate strictly and reject invalid input with a clear error. Only relax this when the user explicitly asks for it.

## Build & Run Commands

Toolchain is pinned via `mise.toml` (go, node, pnpm, sqlc). The `[tasks]` block in `mise.toml` defines shortcuts; direct commands also work.

```bash
# Backend
mise run server                         # go run ./cmd/picotera/main.go (auto-builds llmbridge plugin first)
go build -o picotera ./cmd/picotera     # build binary

# llmbridge cross-format converter plugin
mise run llmbridge-plugin               # go build → dist/picotera-llmbridge-plugin

# Infra (Postgres on :34052, KeyDB/Redis on :34051, MinIO on :34050)
docker compose up -d

# sqlc — edit db/queries/*.sql, then regenerate pkg/db/
sqlc generate

# OpenAPI spec (checked in at openapi.yaml, consumed by dashboard via openapi-typescript)
mise run openapi                        # writes openapi.yaml from `picotera openapi` subcommand

# Dashboard (pnpm workspace, Vue beta pinned in pnpm-workspace.yaml)
mise run web                            # pnpm --dir dashboard dev
pnpm --dir dashboard build              # type-check + vite build
pnpm --dir dashboard lint               # oxlint + eslint, both with --fix
pnpm --dir dashboard type-check         # vue-tsc --build
pnpm --dir dashboard format             # oxfmt src/
pnpm --dir dashboard generate-openapi   # regenerate TS types from openapi.yaml
```

### OpenAPI → TypeScript SDK workflow

The dashboard does not call the API by hand-written client code; types and the fetch client are generated from the spec.

1. Edit Huma operations / contract types in `pkg/contract/` (and any handlers in `pkg/server/`).
2. Run `mise run openapi` — this invokes `go run ./cmd/picotera/main.go openapi` (the `openapi` subcommand on the `humacli` entry point) and writes the result to `openapi.yaml` at the repo root.
3. Run `pnpm --dir dashboard generate-openapi` — `openapi-typescript` consumes the YAML and emits `dashboard/src/openapi-types.d.ts`. `dashboard/src/api/index.ts` re-exports the schema types; `dashboard/src/api/plugin.ts` wires `openapi-fetch` against those types and installs it on the Vue app.

Always run both steps after touching a contract — backend changes are invisible to the dashboard until the TS types regenerate.

Limited Go tests live under `pkg/llmbridge/` and `pkg/server/` covering the bridge format conversions and unified-handler helper functions; the gateway proper still has no postgres-backed test harness. No Go linter is configured. Dashboard lints via oxlint+eslint.

## Bundling the dashboard

The Go binary embeds `pkg/server/static/dist/` via `//go:embed`. By default that directory only contains a placeholder `index.html` (committed) explaining how to bundle the real dashboard — `go build` works on a clean checkout without running any frontend build. Production assembly (Dockerfile / nix build) is responsible for overwriting the placeholder with `dashboard/dist/` output.

To test the bundled binary locally: `pnpm --dir dashboard build && find pkg/server/static/dist -mindepth 1 ! -name index.html ! -name .gitignore -delete && cp -r dashboard/dist/. pkg/server/static/dist/`. The directory has a local `.gitignore` (`pkg/server/static/dist/.gitignore`) that ignores everything except the placeholder, so populated assets won't accidentally get committed.

## Architecture

PicoTera is an API gateway that routes LLM inference requests across multiple providers. It exposes a management API for configuring providers, models, and endpoints.

**Stack**: Go 1.26, Huma v2 (REST framework) + Chi router, PostgreSQL via pgx, sqlc for type-safe queries, goose for migrations, Viper for config.

**Startup flow**: Parse config → run goose migrations → connect PostgreSQL → register Huma operations → mount gateway handler → serve HTTP.

### Package Layout

- `cmd/picotera/` — CLI entry point (humacli + cobra). Also has `openapi` subcommand that prints the spec to stdout.
- `db/migrations/` — goose SQL migrations (embedded in binary). Schema is the source of truth.
- `db/queries/` — sqlc query definitions. **Edit these, then run `sqlc generate`**.
- `pkg/db/` — **Generated by sqlc. Never edit manually.** Contains models, queries, and `Querier` interface.
- `pkg/contract/` — Huma request/response types and operation definitions. Each resource has `To*View`/`From*View` conversion functions between DB models and API views.
- `pkg/server/` — HTTP server, operation handlers (methods on `*Server`), gateway handler.
- `pkg/server/static/` — embedded dashboard SPA (`//go:embed all:dist`). Assets are gzip+brotli precompressed at startup; served as a fallback when the LLM gateway can't match the request path. See "Bundling the dashboard" below.
- `pkg/configx/` — Viper config parsing (env vars with `PICOTERA_` prefix).
- `pkg/errorx/` — Custom error types with structured codes.
- `pkg/logx/` — logrus wrapper.
- `pkg/jsx/` — embedded JavaScript runtime (built on `github.com/fastschema/qjs`) that runs user-supplied scripts as request-lifecycle hooks. See "Scripts" below.
- `pkg/llmbridge/` — adapter types and interfaces for cross-format LLM payload conversion (Anthropic Messages, OpenAI Chat Completions, OpenAI Responses, Gemini GenerateContent). The host talks to the converter through a HashiCorp go-plugin gRPC process.
- `pkg/llmbridgeimpl/` — concrete implementations of `llmbridge` converters, compiled into the plugin target (`cmd/picotera-llmbridge-plugin`). Depends on `github.com/looplj/axonhub/llm` (LGPL-3.0; attribution in `THIRD_PARTY_NOTICES.md`).
- `pkg/kv/` — key-value store abstraction with memory and Redis (KeyDB) backends. Used by user-supplied scripts. CRUD exposed at `/api/picotera/kv`.
- `pkg/artifacts/` — request/response payload serialization. Stores bodies as zstd-compressed JSON with optional line-by-line SSE timings. MinIO-backed via the `picotera-artifacts` bucket.
- `pkg/pricing/` — model pricing calculation and matching logic.
- `pkg/annotations/` — request annotation parsing and handling.
- `pkg/transform/` — generic data transformation utilities.

### Scripts (user JS hooks)

Operators store JS source in the `script` table (CRUD via `/api/picotera/scripts`, dashboard at `ScriptsView.vue` + `ScriptForm.vue`). On each gateway request, `Server.jsxEngine` (configured in `pkg/server/server.go` from `PICOTERA_JS_*` env vars: hook timeout, memory limit, max total attempts, max delay) opens a per-request `jsx.Session`, loads the embedded `pkg/jsx/sdk.js` to install `globalThis.picotera`, then evaluates every enabled script. Scripts call `picotera.hooks.<name>.tap(name, fn, priority)` to register handlers on five waterfalls:

- `sortProviders` — reorder/filter provider candidates before dispatch.
- `rewriteModel` — rewrite the requested model name once, between extraction and provider resolution.
- `beforeRequest` — inspect / delay / skip an attempt; can override `upstreamModel` for that attempt.
- `rewriteRequest` — mutate the pending upstream URL/headers/body just before send.
- `rewriteProviderModels` — rewrite a provider's `providerModels` array based on an upstream `/models` response (used by the "fetch models" flow in `ProviderForm`).
- `afterUpstreamError` — runs after **every** failed upstream attempt (HTTP non-200, connection/network failure, decode/bridge failure, and in-stream SSE errors). Input `{ break, statusCode, message, streamed }`; output `{ break, statusCode, message }`. When `break=true` **and** `streamed=false` (the response hasn't started writing yet), the gateway stops trying further providers and writes the downstream response: `statusCode<=0` follows the upstream's original status (falling back to `502`), `message==""` follows the upstream's original body + `Content-Type` (overrides write the `message` bytes as `application/json`). For in-stream errors (HTTP 200 + SSE error event) the response has already streamed, so `streamed=true` and `break` is ignored — the hook still runs for observation (logging/kv). `statusCode` is integer-cast; a non-string `message` becomes `""`. The hook is advisory: if it errors/times out it is logged and treated as `break=false`. `ctx.attempt.lastError` (`{providerId, statusCode, message}`) is set before the hook runs and, absent a `break`, carries into the next attempt's `beforeRequest`. The hook's `rewriteRequest`/`beforeRequest` JS errors (including timeout-tainted sessions) do **not** trigger it — those keep the existing `failHook` behavior.

Hooks are run as priority-sorted waterfalls (higher priority first); each tap may return a value that becomes the input to the next tap, or `undefined` to pass through. JS-visible context shapes are defined in `pkg/jsx/types.go` (`Candidate`, `RequestShape`, `BeforeRequestInput`, `RewriteInput`, `ProviderSummary`, etc.) — provider credentials are deliberately stripped before crossing the JS boundary. Scripts also get `picotera.fetch(url, init)` (host-side fetch returning parsed JSON) and a `console` shim that captures up to 1000 entries / 256 KiB per session into `LogEntry` records persisted alongside the request.

**Body Proxies (`pkg/jsx/objects.go` + `sdk.js`).** The two large request bodies a hook can touch — `ctx.request.body` and rewriteRequest's `pending.body` — are NOT eagerly embedded as JS objects. They live as `pkg/jsonast` `*Node` trees on the Go side; `sdk.js` wraps each accessed object/array node in a `Proxy` keyed by an integer id whose get/set/enumerate/delete traps forward through the synchronous `__picotera_obj_*` host functions. So a scalar (e.g. a multi-MiB data-url) crosses into QuickJS only when a script reads that field, and writes land straight on the Go tree — keeping memory flat for bodies that previously blew the JS memory limit on the embed→parse→re-stringify round-trip. The session installs `ctx.request.body` via `Session.SetClientBody([]byte)` (call it after any PatchContext that sets `request`; re-calling it after the model rewrite changes the body invalidates the prior Proxy); `RunRewriteRequest(initial, body []byte)` installs `pending.body` and returns the final upstream bytes (nil = fall back to pre-hook bytes, including the byte-identical clean-passthrough case). A hook that never touches the body never parses it. Script-visible semantics: nested reads return cached child Proxies (`body.a === body.a`); `Array.isArray`/`map`/`filter`/`slice`/spread/`Object.keys`/`JSON.stringify` and the array mutators all work; assigning a plain object or another Proxy deep-copies it into the tree (no aliasing); writing `undefined`/functions, out-of-range array writes, deleting a non-last array element, or growing `length` all throw; a stale Proxy (used after its body tree was replaced) throws. Array Proxies carry a `ProxiedArray` prototype (`target → ProxiedArrayProto → Array.prototype`) whose mutators — `splice`/`push`/`pop`/`shift`/`unshift`/`reverse` — route through `__picotera_arr_splice` / `__picotera_arr_reverse` so the Go side reorders the `[]*Node` slice by pointer: existing elements that merely shift position are **relocated, never cloned** (a Proxy passed as an inserted item is still deep-copied, matching the set path). `sort` (needs a JS comparator) keeps the native `Array.prototype` implementation and so still clones via per-index set traps. There is **no** data-url masking at the JS boundary — scripts read originals (the `pkg/datamask` package and `PICOTERA_JS_DATA_URL_MASK_MIN_BYTES` env var were removed from the JS path).

### Key Patterns

- **Endpoint matching**: request paths are matched against `endpoint.path` patterns (which may contain `{name}` placeholders matching any non-empty string, including `/`). The matcher is an in-memory cache (`pkg/server/endpoint_router.go`) loaded lazily from `GetEndpoints` and sorted by literal-character specificity. Any mutation of the `endpoint` table **must** call `Server.endpointRouter.Invalidate()`. Do not reintroduce `GetEndpointByPath` for gateway routing — it only remains for exact-path validation in `handle_provider_endpoint.go`.
- **Project matching**: every gateway request body is scanned by `pkg/server/project_extractor.go` against three fixed regexes (`Workspace root folder:`, `Primary working directory:`, `<cwd>…</cwd>`). Captures are JSON-string unescaped and looked up in `Server.projectRouter` (in-memory longest-prefix cache from `db/queries/project.sql:ListProjectPaths`, mirrors `endpointRouter`). The matched `project_id` is written onto every `request` row for that gateway call (meta + upstream attempts) and triggers an asynchronous `UpsertProjectSeen` updating `project.first_seen_at` / `project.last_seen_at`. Any mutation of the `project` table **must** call `Server.projectRouter.Invalidate()`.
- **sqlc workflow**: Write queries in `db/queries/*.sql` → run `sqlc generate` → use generated code in `pkg/db/`. The `Querier` interface in `pkg/db/querier.go` lists all available DB methods. sqlc is configured in `sqlc.yaml` (pgx/v5 driver, `emit_interface: true`, camelCase JSON tags).
- **Adding an API operation**: Define operation + request/response types in `pkg/contract/`, add handler method on `*Server` in `pkg/server/`, register in `registerOperations()`. After the change, regenerate `openapi.yaml` so the dashboard's typed client picks it up.
- **Config**: All settings via env vars with `PICOTERA_` prefix (e.g., `PICOTERA_DATABASE_URL`, `PICOTERA_PORT`). Default port is 9898.
- **API base path**: All management operations are under `/api/picotera`.
- **Database**: TimescaleDB (`timescale/timescaledb:2.26.4-pg17` image) on port 34052 via docker-compose. Migrations auto-run on startup. A MinIO instance on 34050 (bucket `picotera-artifacts`, bootstrapped by `minio-init`) backs the artifact sink (`pkg/artifacts/`).
- **`request` is a TimescaleDB hypertable** (migration 017): partitioned by `created_at` with composite primary key `(id, created_at)`. `created_at` no longer has a default — every insert/update/delete must supply it (see the `id_created_at` / `created_at` args in `db/queries/request.sql`). Cursor pagination tuples are `(created_at, id)`, not just `id`.

### Unified generation routes

Five chi routes are registered as runtime constants in `server.go` BEFORE the catch-all gateway mount and back the cross-format dispatch:

- `POST /api/picotera/v1/messages` — Anthropic Messages source.
- `POST /api/picotera/v1/responses` — OpenAI Responses source.
- `POST /api/picotera/v1/chat/completions` — OpenAI Chat Completions source.
- `POST /api/picotera/v1beta/models/{model}:generateContent` — Gemini GenerateContent source (non-stream).
- `POST /api/picotera/v1beta/models/{model}:streamGenerateContent` — Gemini GenerateContent source (stream).

These are NOT rows in the `endpoint` table — operators only configure the underlying upstream `endpoint` rows (`anthropicMessages`, `openaiChatCompletions`, `openaiResponses`, `geminiGenerateContent`, `geminiStreamGenerateContent`). The unified handler (`pkg/server/handle_unified_gateway.go`) collects every candidate MPE that supports the requested model+stream tuple via `GetProvidersByEndpointTypesAndModel`, runs all five JS hooks (same shapes as the path-based gateway), and per attempt: if the chosen upstream's `endpoint_type` differs from the source format, runs the body through `pkg/llmbridge/` **before** `rewriteRequest` (the hook sees and mutates the upstream-format body) and the response back before writing to the client. Identity (1:1) attempts are byte-for-byte passthrough so token/TTFT extraction is unaffected.

### Database Schema

Core tables: `provider`, `endpoint`, `provider_endpoint`, `model`, `model_provider_endpoint`, `api_key`, `request` (hypertable), `script`, `traces`, `project`. Uses JSONB for flexible fields (provider models, annotations, project paths). Upsert pattern via `ON CONFLICT DO UPDATE`. The `request` hypertable also carries a nullable `project_id` foreign reference (no FK constraint) populated by the project extractor on insert. TimescaleDB continuous aggregates (`request_overview_hourly`, `request_speed_hourly`) power the overview dashboard metrics.

## Dashboard

See `dashboard/CLAUDE.md` for all dashboard-specific documentation (architecture, components, composables, UI primitives, data layer, design context).
