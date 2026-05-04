# Design — Unified Generation Endpoints

## Goal

Three new gateway routes accept LLM requests in their native formats, fan out
to every configured generation MPE regardless of upstream protocol, and convert
on the fly when the chosen upstream speaks a different format.

| Route                                                          | Source format            | Stream? |
| -------------------------------------------------------------- | ------------------------ | ------- |
| `/api/picotera/v1/messages`                                    | Anthropic Messages       | body    |
| `/api/picotera/v1/responses`                                   | OpenAI Responses         | body    |
| `/api/picotera/v1/chat/completions`                            | OpenAI Chat Completions  | body    |
| `/api/picotera/v1beta/models/{model}:generateContent`          | Gemini GenerateContent   | false (fixed) |
| `/api/picotera/v1beta/models/{model}:streamGenerateContent`    | Gemini GenerateContent   | true (fixed)  |

Generation `endpoint.endpoint_type` set:

- `anthropicMessages`
- `openaiChatCompletions`
- `openaiResponses`
- `geminiGenerateContent` (non-stream only)
- `geminiStreamGenerateContent` (stream only)

The Gemini pair is filtered by the source request's `stream` flag so we never
ship a streaming request to the non-stream Gemini endpoint or vice-versa.

## Architectural shape

The unified routes are **not** rows in the `endpoint` table. They are
registered as literal chi routes in `server.go` ahead of the catch-all gateway
mount. Treating them as rows would force operators to register them, and the
per-row `CredentialsResolver` / `ModelPath` columns don't apply — these routes
have built-in extraction rules driven by source format.

Two flow shapes share the existing handler:

- **1:1 path**: source format == `endpoint.endpoint_type` of the chosen
  candidate. No conversion. Behaves exactly like today's gateway.
- **Bridge path**: formats differ. Convert the (already hook-mutated) pending
  HTTP body to upstream format just before send; convert the upstream HTTP
  response (and SSE chunks) back before writing to the client.

## Conversion library

We import `github.com/looplj/axonhub/llm` as a Go module dependency. That
sub-tree is LGPL-3.0; module-style linking is compatible. Picotera adds:

- `go.mod` requires the package; `go.sum` pins the version.
- A top-level `THIRD_PARTY_NOTICES.md` (or section in README) attributing the
  LGPL-3.0 sub-tree.

Used surface:

- `llm.Request`, `llm.Response`, `llm/streams`, `llm/httpclient` — unified
  in-memory shape and stream helpers.
- `llm/transformer.Inbound` (anthropic, openai chat, openai responses) — to
  parse the source HTTP body into `*llm.Request` and to serialize a unified
  `*llm.Response` back into client-format HTTP.
- `llm/transformer.Outbound` (anthropic, openai chat, openai responses,
  gemini) — to build the upstream HTTP body from `*llm.Request` and to parse
  upstream HTTP / SSE back into `*llm.Response`.

A small `pkg/llmbridge/` adapter wraps these so the rest of picotera doesn't
import axonhub types directly. `llmbridge` exposes:

- `InboundFor(format) Inbound` — picks a transformer by source format.
- `OutboundFor(endpointType) Outbound` — picks a transformer by upstream type.
- `Bridge(ctx, in Inbound, out Outbound, body []byte, hdr http.Header)
  (upstreamBody []byte, contentType string, err error)` — happy-path body
  conversion.
- `BridgeStream(ctx, in, out, upstream io.Reader)
  (io.ReadCloser, error)` — wraps an SSE reader, parses upstream events,
  re-emits events in the source format. The returned reader is what we hand
  to `streamSuccess`.
- `BridgeNonStream(ctx, in, out, body []byte) ([]byte, error)` — parse a
  full upstream JSON body and re-serialize in source format.

Bridge logic always passes through the unified `*llm.Request` /
`*llm.Response` so adding a fourth source format later only adds an Inbound
implementation.

## Routing & MPE resolution

`GetProvidersByEndpointAndModel` matches on a fixed `endpoint_path`. For the
unified routes we need a sister query that matches on a **set of endpoint
types** and respects the streaming filter.

New sqlc query `GetProvidersByEndpointTypesAndModel`:

```sql
SELECT
  e.endpoint_type,
  e.path AS endpoint_path,
  e.credentials_resolver AS endpoint_credentials_resolver,
  -- + every column GetProvidersByEndpointAndModel already returns
FROM provider AS p
JOIN provider_endpoint AS pe ON pe.provider_id = p.id
JOIN endpoint AS e ON e.path = pe.endpoint_path
JOIN model AS m ON m.name = $model_name
CROSS JOIN LATERAL jsonb_array_elements(p.provider_models) AS elem
WHERE e.endpoint_type = ANY($endpoint_types::int[])
  AND p.provider_models @> jsonb_build_array(jsonb_build_object('model', $model_name::text))
  AND elem ->> 'model' = $model_name::text
  AND p.disabled = FALSE
  AND m.disabled = FALSE
  AND COALESCE((elem ->> 'disabled')::boolean, false) = false
  AND (
    elem -> 'endpoints' IS NULL
    OR jsonb_typeof(elem -> 'endpoints') <> 'array'
    OR jsonb_array_length(elem -> 'endpoints') = 0
    OR elem -> 'endpoints' @> to_jsonb(ARRAY[pe.endpoint_path])
  );
```

The handler builds the `endpoint_types` array from the source format and the
detected `stream` flag.

## Handler flow

`handle_gateway.go` keeps its current orchestration. We extract the per-step
logic into helpers and add a `unifiedHandler` that:

1. Authenticates with a synthetic resolver = `Unknown` (so `extractClientToken`
   uses its full fallback chain).
2. Parses the source body once with the format's `Inbound.TransformRequest`
   into a unified `*llm.Request`. The model name and `stream` flag are read
   from the parsed request for the Anthropic / OpenAI routes; for the Gemini
   routes the model comes from the chi path variable `{model}` and the
   stream flag is fixed by the route (`generateContent` = false,
   `streamGenerateContent` = true) — the request body never carries either.
3. Runs **`rewriteModel`** on the source request shape.
4. Calls `GetProvidersByEndpointTypesAndModel` with the right type set,
   applies the existing priority sort, then runs **`sortProviders`** on the
   resulting candidate list (same hook input shape as the path-based gateway:
   `Endpoint`, `Model`, `Request`, `Providers`, `ApiKey`).
5. Per attempt:
   - **`beforeRequest`** hook (sees source format).
   - Build the **pending** upstream `*http.Request`. Body = source-format JSON
     (the same JSON the client sent, with the model swapped). URL = the
     candidate's `upstream_url`. Credentials applied as today.
   - **`rewriteRequest`** hook (sees source-format body, upstream URL+headers).
     Mirrors today's contract — JS may freely edit URL, headers, body.
   - **Bridge step** (new). If candidate's `endpoint.endpoint_type` ≠ source
     format: parse the post-rewrite body via source `Inbound`, serialize via
     candidate `Outbound`, replace `req.Body` and `Content-Type`. The hook is
     blind to the conversion, exactly as the proposal asks.
   - Send. Same retry rules as today.
   - On 200: feed the upstream `Response` through the bridge (or pass through
     for 1:1) and stream/serialize to the client in source format.

All five existing hooks (`sortProviders`, `rewriteModel`, `beforeRequest`,
`rewriteRequest`, `rewriteProviderModels`) run for unified routes with the
same call sites and visible shapes as the path-based gateway. The bridge
step sits **after** `rewriteRequest` so JS scripts always see and edit the
source-format request. (`rewriteProviderModels` is unrelated to per-request
flow and continues to fire only from the "fetch models" admin path.)

## Artifacts

The user requested:

- **Meta artifacts** stay in source-client view — request bytes are the raw
  client body; response bytes are the post-bridge, source-format response
  written back to the client.
- **Upstream artifacts** stay in upstream view — request bytes are what we
  actually wrote on the wire (post-bridge), response bytes are the raw upstream
  response (pre-bridge).

This requires capturing the upstream-format response separately during the
streaming write. The streaming code already accumulates a capture buffer for
artifacts; we tee it from the **upstream** reader (pre-bridge), and the client
write loop drains the **bridged** reader. Two buffers, one per artifact view.

## Stream identity-passthrough

When the source format and candidate type match (1:1 path), the bridge is a
literal `io.Copy`. We must preserve that — no JSON re-encoding round-trip,
because `streamSuccess` already handles raw SSE byte-for-byte and the
`ResponseExtractor` peeks at upstream bytes for TTFT/usage. The bridge
selector explicitly returns a passthrough wrapper in this case.

## What we are NOT doing

- No tool/embedding/image/video conversion. Only chat-style generation.
- No new endpoint rows for users to manage. The five routes are runtime
  constants. Users still configure `endpoint` rows for the underlying upstream
  endpoint types, exactly as today.
- No changes to the existing direct-path gateway. That code path is unchanged.

## Risks

- **Tool-call / content-block fidelity**: bridging Anthropic↔OpenAI reshapes
  some structured content. We adopt axonhub's transformer semantics as ground
  truth and surface conversion errors as 502 with a clear message.
- **SSE event-name mapping**: providers use different SSE event names
  (Anthropic `message_start` / `content_block_delta`, OpenAI `data: {…}`
  only). axonhub normalizes them; the golden-fixture tests in
  `pkg/llmbridge/` lock in the mapping so regressions surface in CI rather
  than in production.
- **LGPL exposure**: linking against an LGPL sub-tree means downstream picotera
  consumers must be able to relink against a modified axonhub. Go modules
  satisfy this trivially via `replace` directives. Document it.
