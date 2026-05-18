# Simulate dispatch

Add a simulation feature: the operator picks an endpoint, an API key, and a model; the backend runs the candidate-resolution pipeline (and the JS hooks that influence it) without sending a real upstream request, and returns the sorted MPE list for the dashboard to render.

## Inputs
- **Endpoint**: either a configured path endpoint (row in the `endpoint` table) or one of the five unified routes.
- **API key**: by `api_key.id`.
- **Model**: model name string.
- **Body**: a JSON request body the operator pastes in the form. Bytes are passed verbatim to JS hooks as `ctx.request.body`. Empty body means body is omitted from the hook context (same rule as production).
- **Stream flag**: inferred — for the two Gemini unified routes the route variant fixes it; for the other unified sources and for path endpoints we read `body.stream`.

## Pipeline (dry run)
1. Resolve the endpoint (path table or unified route) — same code paths the real gateway uses.
2. Load the API key by id; reject disabled keys with the same error shape.
3. Run `rewriteModel` once.
4. Resolve candidate MPEs:
   - Path endpoint → `GetProvidersByEndpointAndModel` (same priority sort as real dispatch).
   - Unified route → `GetProvidersByEndpointTypesAndModel` with the `(srcFormat, stream)` type set, run through the same dedupe + priority sort.
5. Build JS `Candidate` list (same shape as real dispatch, including merged annotations).
6. Run `sortProviders` once.
7. Return the sorted candidate list plus captured console logs.

No request rows are inserted, no artifacts uploaded, no `project_seen` upserts.

## Returned per candidate
- Provider + MPE summary: `providerId`, `providerName`, `endpointPath`, `modelName`, `upstreamModelName`, `priority` (combined), `disabled`, plus the candidate's own provider/MPE annotation maps.
- Merged annotations (model + provider + entry + apiKey — same map JS sees).
- Bridge format info: `sourceFormat` and `upstreamFormat` (only differs for unified routes — flags candidates that will go through the bridge).

The top-level response also returns the resolved model name post-`rewriteModel`, the original model name, and the captured `console.*` log entries.

## Dashboard
New top-level route `/simulate` registered in the sidebar (alongside Endpoints / Models / Scripts), backed by `SimulateView.vue`. The form lets the operator pick the endpoint kind (path endpoint dropdown vs unified route radio), the API key, the model, and a JSON body editor. Results render as a ranked list, one card per candidate, showing provider/MPE summary, bridge info, and merged annotations; a separate panel shows the console logs collected during the simulation.
