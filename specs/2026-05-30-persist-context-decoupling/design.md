# Design: Decouple persistence context from request lifetime

## Goal

Persistence work — request-table writes (`InsertRequest`, `UpdateRequestOnHeader`,
`UpdateRequestModel`, `UpdateRequestOnComplete`), cost lookup (`GetModelByName`), and
synchronous stream aggregation (`bridge.AggregateStream`) — must complete even when the
streaming response outlived any single read-idle window. The request/attempt context
remains cancellable for upstream-read cancellation.

## Current shape

`gatewayContexts` (`pkg/server/gateway_flow_context.go`):

```go
type gatewayContexts struct {
    Request       context.Context
    Persist       context.Context   // WithoutCancel(request) + WithTimeout(GatewayReadTimeout+2s)
    CancelPersist context.CancelFunc
}
```

`Persist` is a single context whose deadline is fixed at request start. Every persistence
call site passes `f.ctxs.Persist` directly. The deadline expires while a long stream is
still being read, so end-of-stream writes fail.

## New shape

`Persist` stops being a single fixed-deadline context. The base is deadline-free; each
persistence phase mints a fresh bounded child whose 30s budget starts at call time.

```go
const persistTimeout = 30 * time.Second

type gatewayContexts struct {
    Request     context.Context
    persistBase context.Context     // WithoutCancel(request), no deadline
    cancelBase  context.CancelFunc  // released when the flow returns
}

// Persist returns a fresh persistence context bounded by persistTimeout, starting now.
// Callers MUST defer the returned cancel.
func (c gatewayContexts) Persist() (context.Context, context.CancelFunc) {
    return context.WithTimeout(c.persistBase, persistTimeout)
}
```

`newGatewayContexts` builds `persistBase = context.WithoutCancel(r.Context())` and a
`cancelBase` that `run()` defers (releases the base when the whole flow returns; all
synchronous writes finish before that point, so it never cancels a live write). The
`cfg`/`GatewayReadTimeout` parameter is no longer used for the persist deadline.

## Call-site conversion

Every site that currently reads the `f.ctxs.Persist` **field** mints a bounded context
instead. Granularity is **per logical phase** (one mint per group of writes that run
together), not per individual statement:

| Location | Phase |
|---|---|
| `gateway_flow.go:insertMetaRequest` | insert + request-artifact + `go upsertProjectSeen` |
| `gateway_flow.go:authenticateAndBackfill` | header backfill |
| `gateway_flow.go:updateMetaModel` | model backfill |
| `gateway_flow_attempts.go` (attempt insert / response artifact / completion) | per-attempt persistence |
| `gateway_flow_success.go:streamSuccess` | header-received, aggregation, completion |
| `gateway_flow_errors.go` (`failMeta`, `failGatewayError`, `failHook`, `failInternal`, `failAllProviders`, `failSuccessPath`) | error persistence |
| `gateway_unified_helpers.go:unifiedStreamSuccess` | header writes (start) + aggregation/completion (after stream loop) |

### Success / unified paths (the actual bug site)

`streamSuccess` currently reads `input.Flow.ctxs.Persist` in three helpers
(`markPathHeadersReceived`, `aggregatePathResponse`, `completeGatewaySuccess`).
- `markPathHeadersReceived` runs at stream start — mints its own persist context.
- `aggregatePathResponse` + `completeGatewaySuccess` run **after** the read loop — they
  mint a fresh persist context (30s budget begins post-stream). This is the line that
  fixes the reported error.

`unifiedStreamSuccess` captures `bgCtx` once at struct-build time (`unifiedStreamArgs`).
The single `bgCtx` field is removed; the function mints one persist context for the
header writes at the top and a second fresh persist context after the streaming read
loop for the aggregation + completion block.

### Goroutine

`go upsertProjectSeen(ctx, …)` keeps its internal `context.WithTimeout(ctx, 5s)`. It is
handed a persist context minted in `insertMetaRequest`; the goroutine owns the lifetime
(cancel deferred inside the goroutine, not by the synchronous caller) so the early-mint
context is not cancelled before the goroutine runs.

## Why artifacts already work

`minioSink.Put` is fire-and-forget: it enqueues a `job` onto a buffered channel and the
worker uploads with `context.Background()` + 30s. The `ctx` argument to `Put` is not
consumed for the upload, so artifact persistence was never affected by the expired
`Persist`. Only `AggregateStream` and the pgx queries consume the passed context.

## Non-goals

- No change to `GatewayReadTimeout` or `idleTimeoutReader` behavior.
- No change to request/attempt cancellation semantics.
- No new config var — the 30s persistence budget is a fixed constant.
- No compatibility shim: `gatewayContexts.Persist` changes from a field to a method and
  every call site is updated.
