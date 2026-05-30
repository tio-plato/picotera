# Plan: Decouple persistence context from request lifetime

Convert `gatewayContexts.Persist` from a single fixed-deadline field into a method that
mints a fresh `context.WithTimeout(persistBase, 30s)` per persistence phase. `persistBase`
is `context.WithoutCancel(requestCtx)` with no deadline; `cancelBase` is released when the
flow returns. Every site reading the old `f.ctxs.Persist` field mints a bounded context.

## Step 1 — Rework `gatewayContexts` (`pkg/server/gateway_flow_context.go`)

- Add `const persistTimeout = 30 * time.Second`.
- Replace the struct fields with:
  - `Request context.Context`
  - `persistBase context.Context`
  - `cancelBase context.CancelFunc`
- `newGatewayContexts(r *http.Request, cfg *configx.Config)`:
  - Keep signature (cfg becomes unused — drop the parameter and update the caller, since
    the timeout is no longer config-derived). Caller is `gateway_flow.go:run()`:
    `f.ctxs = newGatewayContexts(f.r)`.
  - Build `persistBase, cancelBase := context.WithCancel(context.WithoutCancel(r.Context()))`.
  - Return `gatewayContexts{Request: r.Context(), persistBase: persistBase, cancelBase: cancelBase}`.
- Add method:
  ```go
  func (c gatewayContexts) Persist() (context.Context, context.CancelFunc) {
      return context.WithTimeout(c.persistBase, persistTimeout)
  }
  ```
- Remove the `configx` import if it is now unused; drop the `time` import only if unused
  (it is still used by `persistTimeout`).

## Step 2 — `run()` cleanup (`pkg/server/gateway_flow.go`)

- Change `defer f.ctxs.CancelPersist()` → `defer f.ctxs.cancelBase()`.

## Step 3 — Convert path-flow persistence sites (`pkg/server/gateway_flow.go`)

Mint one persist context per method, used for all persistence calls in that method:

- `insertMetaRequest`: at top, `pctx, pcancel := f.ctxs.Persist(); defer pcancel()`. Use
  `pctx` for `insertRequest` and `uploadRequestArtifact`. For the
  `go f.h.upsertProjectSeen(...)` goroutine, mint a **separate** context owned by the
  goroutine so the deferred cancel above does not kill it:
  ```go
  if projectIDPg.Valid {
      seenCtx, seenCancel := f.ctxs.Persist()
      go func() { defer seenCancel(); f.h.upsertProjectSeen(seenCtx, projectIDPg.Int32, createdAt) }()
  }
  ```
- `authenticateAndBackfill`: mint `pctx`, use for `updateRequestOnHeader`.
- `updateMetaModel`: mint `pctx`, use for `updateRequestModel`.

Leave `f.ctxs.Request` sites (`extractProjectID`, `authenticateClient`, `NewSession`,
`fetchModelAnnotations`, `ResolveCandidates`) unchanged.

## Step 4 — Convert attempt-path sites (`pkg/server/gateway_flow_attempts.go`)

- `runSingleAttempt` (line 137): mint `pctx` for the `uploadRequestArtifact` call.
- `insertUpstreamAttempt` (line 197): mint `pctx` for `insertRequest`.
- `handleUpstreamNonOK` (lines 272/274): mint one `pctx`, use for both
  `uploadResponseArtifact` and `updateRequestOnComplete`.
- `recordAttemptFailure` (line 287): mint `pctx` for `completeFailedAttemptWithReason`.

Leave `f.ctxs.Request` sites (`waitHookDelay`, `classifyForwardError`, `insertUpstreamAttempt`'s
`context.WithCancel(f.ctxs.Request)`) unchanged.

## Step 5 — Convert error-path sites (`pkg/server/gateway_flow_errors.go`)

Each of these mints its own `pctx`, used for its persistence calls:

- `failMeta` — `updateRequestOnComplete`.
- `failGatewayError` — `uploadMetaResponseArtifact`.
- `failHook` — `uploadMetaResponseArtifact`.
- `failInternal` — `uploadMetaResponseArtifact`.
- `failAllProviders` — `uploadMetaResponseArtifact` (keep the `f.ctxs.Request.Err()` check
  as-is).
- `failSuccessPath` — `completeFailedAttemptWithReason` and `uploadMetaResponseArtifact`.

Methods that delegate (e.g. `failSuccessPath`/`failGatewayErrorWithFallback` call
`failMeta`) let the delegated method mint its own; only the direct calls in the outer
method use the outer `pctx`.

## Step 6 — Fix the path success path (`pkg/server/gateway_flow_success.go`) — primary bug site

`streamSuccess` calls three helpers that read `input.Flow.ctxs.Persist`. Convert them to
mint at the moment they run:

- `markPathHeadersReceived` (runs at stream start): mint `pctx` at top, replace
  `bgCtx := input.Flow.ctxs.Persist` with it, use for both `updateRequestOnHeader` calls.
- `aggregatePathResponse` (runs **after** the read loop): mint `pctx` at top, use for
  `buildAggregatedArtifact`, `uploadResponseArtifactWithAggregation`,
  `uploadMetaResponseArtifactWithAggregation`. **This is the line whose timeout was
  expiring mid-stream.**
- `completeGatewaySuccess` (runs after the read loop): mint `pctx` at top, replace
  `bgCtx := input.Flow.ctxs.Persist`, use for `costsFor` and both
  `updateRequestOnComplete` calls.
- `openPathInternalReader` error branch (lines 145–158): mint `pctx`, use for
  `completeFailedAttemptWithReason`, `updateRequestOnComplete`, `uploadMetaResponseArtifact`.

Leave `pipePathResponse`'s `classifyStreamFinishReason(..., input.Flow.ctxs.Request)`
unchanged.

## Step 7 — Fix the unified success path (`pkg/server/gateway_unified_helpers.go`)

- Remove the `bgCtx context.Context` field from `unifiedStreamArgs` and its assignment in
  `unifiedStreamArgsFromSuccess`.
- In `unifiedStreamSuccess`:
  - Mint `hdrCtx, hdrCancel := input.Flow.ctxs.Persist(); defer hdrCancel()` near the top
    for the two `updateRequestOnHeader` calls (lines 316/326).
  - After the streaming read loop, mint a fresh `pctx, pcancel := input.Flow.ctxs.Persist();
    defer pcancel()` for the aggregation + completion block (the `buildAggregatedArtifact`
    calls at 508/511, the upload calls at 513/518, `costsFor` at 523, and the
    `updateRequestOnComplete` calls at 540/558/583/593, plus `uploadMetaResponseArtifact`
    at 602). Replace all `a.bgCtx` references with the appropriate minted context.
  - `unifiedStreamSuccess` accesses `input.Flow` already, so it can call
    `input.Flow.ctxs.Persist()` directly.

## Step 8 — Build & test

- `go build ./...`
- `go test ./pkg/server/...` (covers `gateway_flow_test.go`,
  `handle_unified_gateway_test.go`; the unified test calls `buildAggregatedArtifact` with
  `context.Background()` directly, so it is unaffected).
- Grep to confirm no remaining `ctxs.Persist` **field** reads:
  `grep -rn "ctxs.Persist\b" pkg/server` should only match `Persist()` method calls.

## Verification

- Build passes; `pkg/server` tests pass.
- No `CancelPersist` references remain (`grep -rn CancelPersist pkg/server`).
- Manual reasoning: a long stream that ends after `GatewayReadTimeout` has elapsed many
  times now persists completion + aggregation, because each persistence phase gets a fresh
  30s budget starting when it runs, on a base that never carries a request-start deadline.
