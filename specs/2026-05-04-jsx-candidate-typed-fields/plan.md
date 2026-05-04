## Plan

### 1. Adjust `ProviderSummary` and add `CandidateMPE` in `pkg/jsx/types.go`

- Change `ProviderSummary.Annotations` from `map[string]string` to `json.RawMessage`.
- Add `omitempty` on `ProviderSummary.ProviderModels` so dispatch-hook candidates don't carry an empty array.
- Add `CandidateMPE` struct (ModelName, ProviderID, EndpointPath, UpstreamModelName, Priority, Annotations).
- Change `Candidate.Provider` from `any` to `ProviderSummary`.
- Change `Candidate.MPE` from `any` to `CandidateMPE`.
- Change `BeforeRequestInput.Provider` / `MPE` likewise (`ProviderSummary` / `CandidateMPE`).
- Change `RewriteInput.Provider` / `MPE` likewise.

### 2. Update candidate construction in gateway handlers

`pkg/server/handle_gateway.go` (~line 240): replace the `map[string]any{...}` literal with `jsx.ProviderSummary{...}` / `jsx.CandidateMPE{...}`. Keep the same field values; `ProviderModels` stays unset.

`pkg/server/handle_unified_gateway.go` (~line 245): same treatment.

### 2b. Update `rewriteProviderModels` call site

`pkg/server/handle_provider_endpoint.go:127–152`:

- Remove the `var annotations map[string]string; json.Unmarshal(provider.Annotations, &annotations)` block.
- Pass `Annotations: json.RawMessage(provider.Annotations)` directly into `jsx.ProviderSummary{}`.

### 3. Simplify candidate helpers

`pkg/server/gateway_helpers.go`:

- `candidateProviderID(c jsx.Candidate) int32` — drop `(int32, bool)` return, drop type-switch fallback.
- `candidateUpstreamModel(c jsx.Candidate) string` — drop the map probe.

`pkg/server/handle_unified_gateway.go`:

- `candidateEndpointPath(c jsx.Candidate) string` — drop the map probe.

### 4. Drop "malformed candidate" skip branches

In both gateway loops:

- `handle_gateway.go` (~line 291–296): remove the `if !ok { i++; currentRetryCount = 0; continue }` block tied to `candidateProviderID`'s old bool return. Use the new return signature.
- `handle_unified_gateway.go` (~line 292–297): same removal. The `hasSide` guard immediately below stays.

### 5. Update tests in `pkg/jsx/engine_test.go`

- `TestSession_Hooks_Sort` (line 141): change `Candidate{Provider: map[string]any{"id": 1}, MPE: map[string]any{"providerId": 1}}` to `Candidate{Provider: ProviderSummary{ID: 1}, MPE: CandidateMPE{ProviderID: 1}}`. Update the assertion `pm["id"].(float64)` → `out[0].Provider.ID`.
- `TestSession_Hooks_Sort_Passthrough` (line 163): typed literal.
- Any other test constructing `Candidate{...}` or `ProviderSummary{Annotations: map[string]string{...}}` — convert annotation literal to `json.RawMessage([]byte(\`{"k":"v"}\`))`.
- Sweep file for any remaining `Provider: map[string]any{...}` / `MPE: map[string]any{...}` patterns.

### 6. Verify

- `sqlc generate` is not needed (no DB changes).
- `mise run openapi` is not needed (no contract changes).
- `go build ./...` — must compile.
- `go test ./pkg/jsx/... ./pkg/server/...` — must pass.
- Manual: spin up server, run a request through both `handle_gateway` and `handle_unified_gateway`, confirm hooks still see same JS-visible shape (a quick `console.log(ctx.providers[0])` script suffices).
