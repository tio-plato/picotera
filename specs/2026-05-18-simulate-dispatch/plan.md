# Plan — Simulate dispatch

## Step 1 — Contract types

Create `pkg/contract/simulate.go`:

- `SimulateEndpointSelector` struct: `Kind string` (`"path"` | `"unified"`), `Path string`, `Format string`.
- `SimulateDispatchRequest` (Huma): `Body` carries `Endpoint SimulateEndpointSelector`, `ApiKeyID int32`, `Model string`, `Body string` (the raw JSON request body string), `PathVars map[string]string` (omitempty).
- `SimulateProviderSummary` mirrors `jsx.ProviderSummary` minus `ProviderModels` (always omitted here): `ID`, `Name`, `Priority`, `Annotations`, `Disabled`.
- `SimulateMPE` mirrors `jsx.CandidateMPE`: `ModelName`, `ProviderID`, `EndpointPath`, `UpstreamModelName`, `Priority`, `Annotations`.
- `SimulateCandidate`: `Provider`, `MPE`, `MergedAnnotations map[string]string`, `UpstreamFormat string`, `Bridged bool`.
- `SimulateLogEntry`: `Level`, `Message`, `Ts string` (RFC 3339).
- `SimulateDispatchResponse` (Huma): `Body` with `OriginalModel`, `ResolvedModel`, `SourceFormat string`, `Stream bool`, `Candidates []SimulateCandidate`, `Logs []SimulateLogEntry`.
- `var OperationSimulateDispatch = huma.Operation{ OperationID: "simulateDispatch", Method: POST, Path: "/simulate/dispatch", Summary: "Simulate dispatch and return ranked candidates" }`.

Then register it in `pkg/server/server.go:registerOperations()`.

## Step 2 — Backend handler

Create `pkg/server/handle_simulate.go` with `(*Server).handleSimulateDispatch(ctx, *contract.SimulateDispatchRequest) (*contract.SimulateDispatchResponse, error)`. Implementation order inside the handler:

1. **Parse body bytes**: if `req.Body != ""`, require `json.Valid([]byte(req.Body))`; otherwise return `huma.Error400BadRequest(...)` with code `InvalidRequest`. Empty body → `nil` byte slice.
2. **Load API key**: `queries.GetApiKeyByID(ctx, req.ApiKeyID)`; map `pgx.ErrNoRows` to 404 `NotFound`; reject `Disabled` with 403 `Forbidden`. Build `*jsx.ApiKeySummary` via the existing `apiKeySummaryFromRow`.
3. **Resolve endpoint / synthesize virtual**:
   - `kind == "path"`: call `endpointRouter.MatchExact(path)` (or fall through to `Match` if exact-match is already what the router does for a literal path) — return the matched `db.Endpoint`. Map miss to 404 `RouteNotFound`. Then call `extractModel` against the user-supplied body bytes + `pathVars` so `modelPath="{name}"` endpoints get their model name from `pathVars["name"]` rather than the form field; if `extractModel` returns a value, prefer the form's `Model` field but warn-log if they differ. (Production reads the body; the simulator's authoritative model name is the form input, but we still validate against `extractModel` so misconfigured path-vars surface to the operator.)
   - `kind == "unified"`: parse `format` via a `simulateFormatFromString(s)` helper that maps strings to `llmbridge.Format` constants; reject unknown with 400 `InvalidRequest`. Build the virtual `db.Endpoint` exactly as `handleUnifiedGenerate` does (`Name: "(unified)"`, `Path: unifiedRoutePath(format)`, `ModelPath: ""`, `CredentialsResolver: Unknown`, `EndpointType: sourceEndpointType(format)`).
4. **Determine `streaming`**:
   - Path endpoint: `gjson.GetBytes(body, "stream").Bool()`.
   - Unified `geminiStreamGenerateContent`: `true`.
   - Unified `geminiGenerateContent`: `false`.
   - Other unified: `gjson.GetBytes(body, "stream").Bool()`.
5. **Build JSX session**: `jsxEngine.NewSession(ctx, "sim-"+xid.New().String())`; `defer session.Close()`.
6. **Run `rewriteModel`** with `RewriteModelInput` populated from a synthesized `RequestShape` (path: `endpoint.Path`; method `POST`; headers `{}`; model: `req.Model`; pathVars: `req.PathVars`; body: `jsonBodyOrNil(application/json header, body)` — reuse the helper by faking a header map). If the hook returns a new model, update `body` via `sjson.SetBytes(body, "model", newModel)` (unified Gemini routes have no body model field, so for those we skip the body update — same behavior as `setUnifiedModel`).
7. **Resolve candidates**:
   - Path: `resolveProviders(ctx, endpoint.Path, resolvedModel)`.
   - Unified: `resolveProvidersByTypes(ctx, resolvedModel, candidateEndpointTypes(format, streaming), sourceEndpointType(format))`.
   On `NoProviderAvailable`, return `huma.Error404NotFound`.
8. **Build candidate + sidecar maps**: copy-paste the candidate construction loop from `handle_unified_gateway.go` (path version is slightly simpler — keyed by `providerID` alone). Factor the shared bit into a small helper `buildSimulationCandidates(rows, srcFormat, isUnified, apiKeyAnno, modelAnno) (candidates, sidecar)`; both `handle_gateway.go` / `handle_unified_gateway.go` already do this inline so the helper lives in `handle_simulate.go`. Compute `modelAnno` from the first row's `ModelAnnotations` exactly like production.
9. **Run `sortProviders`** with the same `SortInput` production builds (Endpoint, Model, Request, Providers, ApiKey, Annotations).
10. **Drop unknown candidates**: iterate the sorted list; for each, look up `sidecar[providerID|endpointPath]`. If missing, skip — same as the dispatch loop's safety check.
11. **Build response**: for each surviving candidate, fill `provider`, `mpe`, `mergedAnnotations` from the candidate's own `Annotations` (fallback to sidecar entry), `upstreamFormat: sidecar.upFormat.String()`, `bridged: sidecar.upFormat != srcFormat`. `logs` from `session.Logs()` mapped to `contract.SimulateLogEntry`.

### Helpers added in `handle_simulate.go`

- `simulateFormatFromString(s string) (llmbridge.Format, error)`
- `unifiedRoutePath(f llmbridge.Format) string` — returns `/api/picotera/v1/messages` etc. Could also live in `handle_unified_gateway.go` and be reused.

## Step 3 — Generate OpenAPI + TS types

After Steps 1–2 compile:

```
mise run openapi
pnpm --dir dashboard generate-openapi
```

These rewrite `openapi.yaml` and `dashboard/src/openapi-types.d.ts` so the dashboard's typed client picks up the new operation.

## Step 4 — Dashboard data layer

In `dashboard/src/api/client.ts`, add:

```ts
export async function simulateDispatch(body: components['schemas']['SimulateDispatchRequestBody']):
  Promise<components['schemas']['SimulateDispatchResponseBody']> { … }
```

(Schema names will match what openapi-typescript generates — adjust if Huma names them differently.) Re-export the response/candidate types from `src/api/index.ts`. No `queryKeys` entry — simulate is mutation-only.

## Step 5 — Dashboard view

Create `dashboard/src/views/SimulateView.vue`:

- Two columns built from `DataCard`, `Field`, `Input`, `Select`, `SegmentedControl`, `Textarea`, `Button`.
- Form state: `kind`, `path`, `format`, `apiKeyId`, `model`, `bodyText`, `pathVars`.
- Submission: `useMutation` calling `simulateDispatch`. Pre-submit, run `JSON.parse(bodyText)` if non-empty; on parse error, set a banner via local `ref<string>` and abort the mutation. Re-stringify so the wire payload is canonical.
- Path-endpoint variable inputs: derived from the chosen endpoint's `path` field — extract `{name}` tokens via a small regex helper in the view, render one `Input` per token, store in `pathVars` map. Hide the block when there are no tokens.
- Result panel: ranked list of candidates rendered with primitives. Use `Tag` for bridge info (e.g. `bridged` shows two-format pill), `StateText` for disabled providers.
- Logs panel: list with `level` color-coded via `StateText`. Reuse the request-detail-style monospace rendering if a primitive already exists; otherwise inline Tailwind.

Add the route to `dashboard/src/router/index.ts` (name: `simulate`, path: `/simulate`, component import).

Add the navigation entry to `dashboard/src/components/AppSidebar.vue` (icon: pick something appropriate from `@tabler/icons-vue` already in use, e.g. `IconPlayerPlay` or `IconBolt`).

Add a `simulate` entry to the `pageMeta` map in `dashboard/src/App.vue` with `title: '模拟'`, hint short.

## Step 6 — Smoke check

- `go build ./...` to ensure the new contract / handler compile.
- `mise run openapi` regenerates the spec; verify the response schemas appear.
- `pnpm --dir dashboard generate-openapi` regenerates TS types.
- `pnpm --dir dashboard type-check && pnpm --dir dashboard lint` to ensure the view + client compile and lint clean.
- Manual smoke: with `docker compose up -d` running and a path endpoint + provider configured locally, hit `/simulate` from the dashboard and confirm the candidate list matches what `RequestsView` shows for a real request against the same endpoint/key/model.

## Files touched

New files:
- `pkg/contract/simulate.go`
- `pkg/server/handle_simulate.go`
- `dashboard/src/views/SimulateView.vue`

Modified files:
- `pkg/server/server.go` — `registerOperations` registration.
- `dashboard/src/api/client.ts` — fetcher.
- `dashboard/src/api/index.ts` — type re-exports if needed.
- `dashboard/src/router/index.ts` — route.
- `dashboard/src/components/AppSidebar.vue` — nav entry.
- `dashboard/src/App.vue` — `pageMeta` entry.
- `openapi.yaml`, `dashboard/src/openapi-types.d.ts` — regenerated.
