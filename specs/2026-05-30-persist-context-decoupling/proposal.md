# Proposal: Decouple persistence context from request lifetime

## Problem

When an upstream streaming response runs long and eventually stalls/times out, the
gateway fails to write the final request status and aggregated response into the
request table. The logs show:

```
WARN artifact: aggregate response stream failed    error="context deadline exceeded" format=openaiChatCompletions
ERRO failed to update request on complete           error="context deadline exceeded"
```

The request context must still be cancellable so that a client disconnect cancels
upstream reads. The fix should give persistence (request-table writes + stream
aggregation) its own context, separate from the request lifetime.

## Root cause

`newGatewayContexts` builds the `Persist` context with a deadline of
`GatewayReadTimeout + 2s` anchored at **request start**. `GatewayReadTimeout` is a
per-read *idle* timeout, not a whole-stream budget, so a streaming response that
keeps trickling data outlives that deadline. By the time the stream ends, the
`Persist` context has already expired, and the two synchronous consumers of it fail:

- `bridge.AggregateStream(ctx, …)` — the WASM stream aggregation.
- `UpdateRequestOnComplete(ctx, …)` — the final request-table write.

Artifact uploads are unaffected because `minioSink.Put` enqueues to a channel and the
worker uploads with its own `context.Background()` + 30s timeout.

## Decision

- Persistence runs on a context derived from `context.WithoutCancel(requestCtx)` with
  **no fixed deadline anchored at request start**.
- Each persistence phase mints its own bounded context whose timeout starts when that
  phase runs, so it can never expire mid-stream.
- The per-phase persistence timeout is a fixed **30s** constant.
- The request/attempt context stays cancellable so client disconnect still cancels
  upstream reads.
