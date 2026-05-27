# Design: Request Finish Reason

## Column

`finish_reason INTEGER` nullable column on the `request` table. NULL means the request is still in progress. Set exactly once when `UpdateRequestOnComplete` is called. Follows the same integer-enum pattern as `type` and `status`.

Go constants in `pkg/db/request_constants.go`:

```go
const (
	FinishReasonInternal       = 1 // internal error (no providers, script failure, etc.)
	FinishReasonCancelled      = 2 // client cancelled
	FinishReasonEOF            = 3 // server ended request normally
	FinishReasonHeadersTimeout = 4 // timeout before receiving response headers (TLS, connect, etc.)
	FinishReasonReadTimeout    = 5 // timeout reading response body
)
```

## `finish_reason` vs `status`

`finish_reason` is orthogonal to `status`. A request that streamed partial data before a read timeout will still have `status = completed` (HTTP 200 headers were already sent) but `finish_reason = read_timeout`. This gives observability into why a request ended without changing existing status semantics.

## Classification by call site

### Upstream requests

| Scenario | finish_reason |
|---|---|
| `completeGatewaySuccess` — 200 response fully streamed | `eof` |
| `pipePathResponse` — `idleTimeoutReader` fired | `read_timeout` |
| `pipePathResponse` — client context cancelled during streaming | `cancelled` |
| `forwardRequest` returned `net.Error.Timeout()` or `context.DeadlineExceeded` | `headers_timeout` |
| `forwardRequest` returned `context.Canceled` and request context is done | `cancelled` |
| `handleUpstreamNonOK` — upstream returned non-200 | `internal` |
| `recordAttemptFailure` — build error, decode error, other | `internal` |

### Meta requests

| Scenario | finish_reason |
|---|---|
| `completeGatewaySuccess` — request completed normally | same as upstream's finish_reason |
| `failAllProviders` — all upstreams exhausted, request context cancelled | `cancelled` |
| `failAllProviders` — all upstreams exhausted, request context alive | `internal` |
| `failHook` — JS hook error | `internal` |
| `failInternal` — internal server error | `internal` |
| `failGatewayErrorWithFallback` — gateway error | `internal` |
| `failSuccessPath` — error during response processing | `internal` |
| `openPathInternalReader` — decode error | `internal` |
| `authenticateAndBackfill` — auth failure | `internal` |

## Detecting exit conditions in the streaming path

`pipePathResponse` currently discards the final read error. It needs to return a `finishReason` string derived from the last error:

1. `io.EOF` → `eof`
2. `errors.Is(err, errReadIdleTimeout)` → `read_timeout` (new sentinel on `idleTimeoutReader`)
3. Request context cancelled → `cancelled`
4. Other → `eof` (partial data was already written to client; we can't retract the 200)

The sentinel `errReadIdleTimeout` replaces the ad-hoc `fmt.Errorf` in `idleTimeoutReader.Read`.

## Detecting exit conditions in `forwardRequest` errors

A `classifyForwardError` helper inspects the error from `transport.RoundTrip`:

1. `context.Canceled` and request context is done → `cancelled`
2. `net.Error` with `Timeout() == true` → `headers_timeout`
3. `context.DeadlineExceeded` → `headers_timeout`
4. Other → `internal`

## Continuous aggregates

`request_overview_hourly` and `request_speed_hourly` do not reference `finish_reason`. No changes needed.
