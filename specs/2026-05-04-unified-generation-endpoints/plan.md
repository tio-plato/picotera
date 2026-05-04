# Plan — Unified Generation Endpoints

Each step lands in its own commit and leaves the build green.

## 1. Add the axonhub dependency

- Run `go get github.com/looplj/axonhub@<latest unstable commit>` and pin in
  `go.mod` / `go.sum`.
- Add a `THIRD_PARTY_NOTICES.md` at repo root reproducing the LGPL-3.0 notice
  for the `llm/` sub-tree (mirrored from upstream `llm/LICENSE`).
- Smoke import in a throwaway `_test.go` to confirm the module resolves; delete
  the smoke file before commit.

## 2. `pkg/llmbridge/` adapter

New package wrapping axonhub. Files:

- `llmbridge.go` — `Format` enum (`SourceAnthropicMessages`,
  `SourceOpenAIChat`, `SourceOpenAIResponses`,
  `SourceGeminiGenerate`, `SourceGeminiStreamGenerate`,
  `UpstreamAnthropicMessages`, `UpstreamOpenAIChat`,
  `UpstreamOpenAIResponses`, `UpstreamGeminiGenerate`,
  `UpstreamGeminiStreamGenerate`), `InboundFor`, `OutboundFor`.
- `bridge.go` — `BridgeNonStream(ctx, srcFormat, upFormat, upstreamBody)`
  takes the upstream JSON body and returns source-format JSON. Identity
  shortcut when `srcFormat == upFormat`.
- `bridge_request.go` — `BridgeRequest(ctx, srcFormat, upFormat,
  postRewriteBody, headers)` returns `(upstreamBody, contentType)`. Identity
  shortcut.
- `bridge_stream.go` — `BridgeStream(ctx, srcFormat, upFormat, upstream
  io.Reader) (io.ReadCloser, error)`. Identity returns the input wrapped in
  a `nopCloser`; cross-format wraps an axonhub
  `streams.Stream[*httpclient.StreamEvent]` parsed off the upstream reader,
  passes through `Outbound.TransformStream` → `Inbound.TransformStream`, and
  emits source-format SSE bytes.
- `bridge_test.go` — golden tests for one full request and one full stream
  per format pair we expect in production. Source-of-truth fixtures live in
  `pkg/llmbridge/testdata/`.

Public surface stays minimal. No axonhub types leak.

## 3. New sqlc query and regenerated code

- Edit `db/queries/routing.sql`: add `GetProvidersByEndpointTypesAndModel`
  per `design.md`. Uses `endpoint_type = ANY($endpoint_types::int[])`.
- Run `sqlc generate`. Verify the generated `db.GetProvidersByEndpointTypesAndModelRow`
  carries `EndpointType` and the same set of fields the existing
  `GetProvidersByEndpointAndModelRow` exposes. Add the new method to the
  `Querier` interface (sqlc will do this automatically).
- No DB migration needed.

## 4. Shape `unifiedGatewayHandler`

New file `pkg/server/handle_unified_gateway.go`. The handler is a thin
orchestrator that **calls into existing helpers** in `gateway_helpers.go` —
no logic duplication. Pseudocode:

```go
func (h *Server) handleUnifiedGenerate(srcFormat llmbridge.Format) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1) Read body, insert meta row, upload meta request artifact.
        // 2) Authenticate with resolver=Unknown (full fallback).
        // 3) Resolve model and stream flag:
        //    - Anthropic / OpenAI sources: Inbound.TransformRequest pulls
        //      both from the body. On parse error: 400 INVALID_REQUEST
        //      (or MODEL_NOT_FOUND when only the model is missing).
        //    - Gemini sources: model = chi.URLParam(r, "model"); stream is
        //      a constant baked into the registered Format.
        // 4) Build endpoint_types set from srcFormat + stream flag.
        // 5) Build a "virtual endpoint" db.Endpoint{Path:r.URL.Path,
        //    ModelPath:"", CredentialsResolver:Unknown,
        //    EndpointType: srcFormat-as-int} for hook payloads.
        // 6) jsx.Session.NewSession, defer Close.
        // 7) RunRewriteModelHook -> body.model rewrite via sjson.
        // 8) GetProvidersByEndpointTypesAndModel.
        //    - apply the same priority sort as the path-based gateway.
        //    - RunSortHook (sortProviders) on the candidate list. Same input
        //      shape as today: {Endpoint, Model, Request, Providers, ApiKey}.
        // 9) Retry loop (mirrors handle_gateway.go):
        //      - RunBeforeRequestHook
        //      - buildUpstreamRequest with the candidate's upstream URL
        //      - RunRewriteHook
        //      - bridgeRequest(srcFormat, candidate.EndpointType,
        //        postRewriteBody)  // identity if equal
        //      - upload upstream-request artifact AFTER bridge (upstream view)
        //      - forward
        //      - on 200: streamSuccess wrapped to (a) tee raw upstream bytes
        //        into upstream-artifact buffer, (b) drain bridged bytes into
        //        client + meta-artifact buffer.
        // 10) Failure path identical to handle_gateway.go.
    }
}
```

Concrete refactors to keep `handle_gateway.go` and the new handler honest:

- Pull the per-attempt block (after candidate selection, up to and including
  `streamSuccess`) into a helper `runAttempt(...)` shared by both handlers.
- Pull the failure-path closures (`failMeta`, `failHook`, etc.) into a small
  `metaCompletion` helper that accepts the meta IDs + collectLogs closure.

After refactor, `handle_gateway.go` shrinks but its observable behavior is
unchanged.

## 5. Bridge wiring inside `runAttempt`

Augment `runAttempt` to accept:

- `srcFormat llmbridge.Format` (zero value = "no bridge, run as today").
- `upFormat llmbridge.Format` (only consulted when `srcFormat != 0`).

When both are non-zero and unequal:

- After `buildRequestFromPending`, call
  `llmbridge.BridgeRequest(ctx, srcFormat, upFormat, reqBody, req.Header)`
  and replace `req.Body` / `req.ContentLength` / `Content-Type` with the
  result.
- Inside `streamSuccess`, swap the read pipeline:

  ```text
  raw upstream io.Reader
        │
        ├── tee → upstream-artifact buffer (raw upstream bytes)
        │
        └── llmbridge.BridgeStream(...)  // identity if formats match
              │
              └── ResponseExtractor → idleTimeoutReader → w (client)
                                                       └→ meta-artifact buffer
  ```

  The non-stream JSON path goes through `BridgeNonStream` instead; same tee
  pattern (raw bytes → upstream artifact, bridged bytes → meta artifact +
  client).

Compatibility: when `srcFormat == upFormat` the bridge functions are identity
shims, so existing 1:1 paths through `runAttempt` stay byte-for-byte the
same. Verify with the existing `response_extractor_test.go` continuing to
pass unchanged.

## 6. Mount the routes

- In `server.go`'s router setup, register the five routes **before** the
  fallback gateway mount:

  ```go
  router.Post("/api/picotera/v1/messages",         h.handleUnifiedGenerate(llmbridge.SourceAnthropicMessages))
  router.Post("/api/picotera/v1/responses",        h.handleUnifiedGenerate(llmbridge.SourceOpenAIResponses))
  router.Post("/api/picotera/v1/chat/completions", h.handleUnifiedGenerate(llmbridge.SourceOpenAIChat))
  router.Post("/api/picotera/v1beta/models/{model}:generateContent",       h.handleUnifiedGenerate(llmbridge.SourceGeminiGenerate))
  router.Post("/api/picotera/v1beta/models/{model}:streamGenerateContent", h.handleUnifiedGenerate(llmbridge.SourceGeminiStreamGenerate))
  ```

  For the Gemini routes the handler reads the model from `chi.URLParam(r,
  "model")` and ignores any `model` field that may appear in the body. The
  `stream` flag is fixed by which of the two source formats was registered
  for the route (Generate = false, StreamGenerate = true).

- Literal `Post` registrations take precedence over the catch-all gateway
  mount in chi. Document this precedence in `endpoint_router.go`'s top
  comment so future readers know why these three paths never reach
  `endpointRouter.Match`.

## 7. Errors

- Add `errorx.InvalidRequest = errorx.Code("INVALID_REQUEST")` if not present.
- Map all `transformer.Err…` errors and bridge errors to that code at the
  unified handler boundary.

## 8. Tests

Picotera has no Go tests today. Introduce a minimal table:

- `pkg/llmbridge/bridge_test.go` — golden body conversion + golden SSE
  conversion for each format pair we route in production. Fixtures captured
  from real upstream responses (committed under `testdata/`).
- `pkg/server/handle_unified_gateway_test.go` — smoke only: route is
  registered and 401s on missing credentials, 404s on unknown model, 502s
  when the bridge fails. Uses an in-memory chi router with the DB queries
  faked through the `db.Querier` interface; no real postgres.

DB-backed integration tests are out of scope. The codebase has no postgres
test harness today, and adding one belongs in a separate spec.

## 9. Documentation

- Update `CLAUDE.md`: a new section under "Architecture" describing the
  unified routes, the `pkg/llmbridge/` module, and the LGPL-3.0 attribution
  pointer.
- Update `THIRD_PARTY_NOTICES.md`.

## 10. Verification before merge

- `mise run server` — boot the binary against a populated dev DB. Hit each
  of the three routes with `curl` for both stream and non-stream cases, and
  for each (source format × candidate upstream type) pair that exists in the
  dev config. Confirm:
  - 200 with sensible body in source format.
  - Meta artifact body matches what the client got; upstream artifact body
    matches raw upstream wire bytes.
  - Request DB row records the chosen provider, model, upstream model.
- `pnpm --dir dashboard build` — guards against accidental dashboard
  breakage. (No dashboard changes are part of this plan, but the build is
  cheap.)
- `go build ./...` clean.

## Out of scope

- Embeddings, image, video, rerank.
- A management UI for the unified routes (they need none — they are runtime
  constants).
- Token-budget enforcement, content moderation, anything beyond byte-and-shape
  passthrough.
