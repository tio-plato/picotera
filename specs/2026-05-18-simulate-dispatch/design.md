# Design — Simulate dispatch

## Overview

The simulator is a read-only Huma operation at `POST /api/picotera/simulate/dispatch` that runs the first half of the gateway pipeline (endpoint match → auth → `rewriteModel` → candidate resolution → `sortProviders`) and returns the resulting candidate list to the dashboard. No request rows, artifacts, or seen-at timestamps are written.

## Backend structure

A single new handler file `pkg/server/handle_simulate.go` plus a contract type set in `pkg/contract/simulate.go`. Both the path-endpoint flow and the unified-route flow share one entry point; the handler branches on the `endpoint` discriminator in the request body and reuses the existing helpers:

- `Server.resolveEndpoint` + the in-memory endpoint router for path endpoints — unchanged.
- `Server.resolveProviders` for path endpoints.
- `Server.resolveProvidersByTypes` + `candidateEndpointTypes` + `dedupeUnifiedRows` for unified routes.
- `newCandidateAnnotationsBuilder` + its `merge` method for the per-candidate annotation merge.
- `jsx.Session` with `RunRewriteModelHook` and `RunSortHook`.

No new SQL queries are required. Nothing in `db/migrations/` changes.

### Auth flow

The simulator deliberately bypasses the production credential-extraction code (`extractClientToken` / `authenticateClient`) because the caller is the dashboard operator, not the API key holder; they pick the api key by id. We load the row directly via `GetApiKeyByID` and reject disabled keys with the same `Forbidden` error shape the gateway returns. The simulator endpoint itself sits under `/api/picotera/` and inherits whatever management-API authorization the rest of the dashboard uses (none today).

### Endpoint resolution

The request discriminates between two endpoint kinds:

- `kind: "path"` with `path` — looked up via `endpointRouter.Match(path)`. The matched row's `EndpointType` and `CredentialsResolver` flow into the JS `endpoint` context exactly like production. Path variables matched off the request URL are surfaced too — but because the simulator has no real request URL, `pathVars` is taken from a `pathVars` map in the request body (empty by default). `extractModel` is invoked the same way so that endpoints with `modelPath="{name}"` work.
- `kind: "unified"` with `format` ∈ `{anthropicMessages, openaiChatCompletions, openaiResponses, geminiGenerateContent, geminiStreamGenerateContent}` — we synthesize a virtual endpoint identical to what `handleUnifiedGenerate` builds (`Name: "(unified)"`, `Path` from the matching unified route, etc.). The stream flag for the two non-Gemini formats is read from `body.stream` (same as production); for the Gemini routes it is fixed by the chosen format variant.

### Hook execution

Two of the existing five hooks run:

1. `RunRewriteModelHook` — once, with `RewriteModelInput` populated exactly like the gateway. If the hook returns a new model name and the source format carries a `model` field, we update the simulation body via `sjson.SetBytes(body, "model", newModel)`. The candidate query is then run against the rewritten model.
2. `RunSortHook` — once, with `SortInput` populated from the resolved candidates.

`beforeRequest`, `rewriteRequest`, `beforeTransform`, and `rewriteProviderModels` are not run. The session's `console.*` capture buffer is read once at the end via `session.Logs()` and surfaced verbatim in the response.

### Bridge format info

For path endpoints, `sourceFormat == upstreamFormat == endpoint.EndpointType`. For unified routes, `sourceFormat` is the route's format constant and `upstreamFormat` per candidate comes from `upstreamFormatFor(row.EndpointType)` (same helper the unified handler uses). Both strings use the same vocabulary as `llmbridge.Format.String()` so the frontend can display them without translation.

### Returned shape

`SimulateDispatchResponse.Body` carries:

- `originalModel` and `resolvedModel` (post-`rewriteModel`).
- `sourceFormat` (string).
- `stream` (bool, mostly meaningful for unified routes).
- `candidates`: ordered array of `SimulateCandidate` entries. Each candidate has `provider`, `mpe`, `mergedAnnotations`, `upstreamFormat`, and `bridged` (bool — `sourceFormat != upstreamFormat`).
- `logs`: console entries `{level, message, ts}` from the JSX session.

### Errors

All failures use the same `errorx` codes as production: `RouteNotFound`, `Unauthorized` / `Forbidden`, `ModelNotFound`, `NoProviderAvailable`, `InternalError`. Huma surfaces them via the standard `huma.Error*` constructors; we map `gatewayError.status` to the matching huma error so the dashboard sees a typed response.

## Frontend

A new top-level route `simulate` mapped to `views/SimulateView.vue` and listed in `AppSidebar` plus `App.vue`'s `pageMeta` map. The view is composed entirely from `src/ui/` primitives — no new UI dependency.

### View layout

Two-column layout on wide screens, stacked on narrow:

1. **Form column** (`DataCard` titled "模拟参数"):
   - `SegmentedControl` for endpoint kind (`path` vs `unified`).
   - When `path`: a `Select` listing `endpointRouter`'s configured endpoints (`listEndpoints` already exists). When `unified`: a `Select` listing the five formats.
   - `Select` for API key (`listApiKeys` already exists).
   - `Input` for the model name.
   - `Textarea` for the JSON body (with a "format" affordance — JSON.parse + JSON.stringify on blur, never silently fix bad JSON).
   - `Button` "模拟" that triggers the mutation.
2. **Result column** (`DataCard` titled "排序后候选项"):
   - Header rows for `originalModel → resolvedModel`, `sourceFormat`, `stream`.
   - One numbered card per candidate with provider name + id, MPE info (endpoint path, upstreamModelName, priority), bridge badges (`Tag`/`Badge`), and a collapsible "annotations" block rendered as a key/value table.
   - Empty state when the candidate list is empty (e.g. NoProviderAvailable returns 404 surfaced as an error toast; we render the message and the candidates panel stays empty).
3. **Logs panel** below the result column or in a separate `DataCard` titled "脚本日志" — rendered only when `logs.length > 0`. Uses the same monospaced styling as the request-detail logs panel.

### Data layer

- New fetcher in `src/api/client.ts`: `simulateDispatch(body)` returning the typed response.
- New key in `src/api/queryKeys.ts`: not needed — simulation is a mutation, never cached.
- `useMutation` in the view; on error, the localized message from `ApiRequestError` is shown in a `StateText`-styled banner above the results.
- The form's JSON body field is held in a local `ref<string>`; on submit we `JSON.parse` and re-stringify before sending — invalid JSON is rejected client-side with the same "fail fast" attitude the backend enforces.

## Open questions / decisions
- `apiKey.disabled` is rejected with a clear error (matches production).
- The body is sent as-is (UTF-8 bytes) without normalization. The backend asserts `application/json` content-type implicitly because hooks see `body` only when it's valid JSON — same rule as production.
- `pathVars` for path endpoints with `{name}` placeholders is an explicit map in the request body. The form only surfaces it if the chosen endpoint's path contains `{…}` tokens (a small helper extracts the variable names and renders one `Input` per name).
