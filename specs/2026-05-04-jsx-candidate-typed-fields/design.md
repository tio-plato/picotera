## Design

### Goal

Replace the `any`-typed `Provider` and `MPE` fields on `jsx.Candidate`, `jsx.BeforeRequestInput`, and `jsx.RewriteInput` with named struct types. The de-facto JS-visible shape becomes Go-visible, so call sites construct typed literals and helpers read typed fields instead of asserting `map[string]any`.

### Current state

Both `handle_gateway.go` and `handle_unified_gateway.go` build candidates with the same `map[string]any` literal:

- `Provider`: `id (int32)`, `name (string)`, `priority (int32)`, `annotations (json.RawMessage)`.
- `MPE`: `modelName (string)`, `providerId (int32)`, `endpointPath (string)`, `upstreamModelName (string)`, `priority (int32)`, `annotations (json.RawMessage)`.

The same shape rides along on `BeforeRequestInput.{Provider, MPE}` and `RewriteInput.{Provider, MPE}` (copied from the chosen candidate). After a JS hook returns modified candidates, helpers (`candidateProviderID`, `candidateUpstreamModel`, `candidateEndpointPath`) probe the maps with type assertions and float64 fallbacks.

### Reuse `ProviderSummary` for `Candidate.Provider`

The existing `jsx.ProviderSummary` (currently used by `rewriteProviderModels`) has the right shape — `ID`, `Name`, `Priority`, `Annotations`, `Disabled`, plus `ProviderModels []ProviderModelEntry`. Reuse it as-is for `Candidate.Provider`. JS scripts then see a single, consistent provider shape across all four hooks.

One adjustment: change `ProviderSummary.Annotations` from `map[string]string` to `json.RawMessage`. Reasons:

- Candidate construction reads `row.ProviderAnnotations []byte` from the DB and currently passes it as `json.RawMessage` — already raw JSONB. Forcing an unmarshal-per-candidate is wasteful and lossy (the JSONB column may hold non-string values).
- The `rewriteProviderModels` call site (`handle_provider_endpoint.go:130–152`) already has `provider.Annotations` as raw `[]byte`. Drop the local `json.Unmarshal` block; pass the bytes through as `json.RawMessage`.
- JS scripts read `provider.annotations.foo` either way — JSON object on the wire is identical.

`ProviderModels` stays as-is. For dispatch hooks (sort/beforeRequest/rewriteRequest) the candidate-fetch query does not return the full models array, so leave the field as the zero value (`nil`/`[]`). Add `omitempty` so dispatch-hook candidates don't carry an empty `providerModels: []` over the JS boundary.

### New `CandidateMPE` struct

`ProviderModelEntry` exists but doesn't fit MPE — different field names (`Model` vs `ModelName`), missing `ProviderID`, has `Endpoints []string` (config allow-list) instead of the resolved `EndpointPath`. Introduce a new struct:

```go
// CandidateMPE is the JS-visible projection of a model_provider_endpoint row,
// extended with the resolved endpoint path so hooks can disambiguate
// candidates that share a provider but differ by endpoint.
type CandidateMPE struct {
    ModelName         string          `json:"modelName"`
    ProviderID        int32           `json:"providerId"`
    EndpointPath      string          `json:"endpointPath"`
    UpstreamModelName string          `json:"upstreamModelName"`
    Priority          int32           `json:"priority"`
    Annotations       json.RawMessage `json:"annotations,omitempty"`
}
```

### Field changes

- `Candidate.Provider any` → `ProviderSummary`.
- `Candidate.MPE any` → `CandidateMPE`.
- `BeforeRequestInput.Provider any` → `ProviderSummary`.
- `BeforeRequestInput.MPE any` → `CandidateMPE`.
- `RewriteInput.Provider any` → `ProviderSummary`.
- `RewriteInput.MPE any` → `CandidateMPE`.

`SortInput.Endpoint`, `SortInput.Model`, and `BeforeRequestInput.Endpoint`/`Model`, `RewriteInput.Endpoint`/`Model` stay `any` — out of scope.

### `ProviderSummary.Annotations` migration impact

One existing call site:

- `pkg/server/handle_provider_endpoint.go:130–152` — drop the `var annotations map[string]string; json.Unmarshal(provider.Annotations, &annotations)` block; pass `Annotations: json.RawMessage(provider.Annotations)` directly.

Tests in `pkg/jsx/engine_test.go` constructing `ProviderSummary{... Annotations: map[string]string{...}}` (if any) need their annotation literal converted to `json.RawMessage([]byte(`{"k":"v"}`))`.

### JS round-trip

When a hook returns a modified candidate array, `RunSortHook` decodes JSON straight into `[]Candidate`. With struct fields, `encoding/json` parses JSON numbers directly into `int32` — no more float64 fallback. `Annotations json.RawMessage` round-trips JSONB objects untouched.

The JS side is contract-stable: scripts read/write the same property names (`id`, `name`, `priority`, `annotations`, `modelName`, etc.). No SDK changes.

### Helper simplification

- `candidateProviderID(c jsx.Candidate) int32` — return `c.Provider.ID` directly. Drops the `(int32, bool)` signature; callers no longer need to skip "malformed" candidates because a typed struct can't be malformed.
- `candidateUpstreamModel(c jsx.Candidate) string` — return `c.MPE.UpstreamModelName`.
- `candidateEndpointPath(c jsx.Candidate) string` — return `c.MPE.EndpointPath`.

The two gateway loops drop the `if !ok { skip }` guard for malformed candidates and the related `currentRetryCount = 0; continue` branch. The sidecar lookup (`hasSide`) remains — that guards against JS introducing an unknown `(providerID, endpointPath)` pair, which is still a runtime concern.

### Test impact

`pkg/jsx/engine_test.go` constructs `Candidate{Provider: map[string]any{...}}` literals in three places. Replace with `Candidate{Provider: CandidateProvider{ID: 1}, MPE: CandidateMPE{ProviderID: 1}}`. Post-hook assertions that read `out[0].Provider.(map[string]any)["id"].(float64)` become `out[0].Provider.ID`.

### Out of scope

- TODO #2 (annotation-driven outbound transform).
- Endpoint/Model fields on hook inputs.
- Anything in `pkg/jsx/sdk.js`.
