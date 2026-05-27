# Plan: Request Finish Reason

## Step 1: Database migration

Create `db/migrations/027_request_finish_reason.sql`:

```sql
-- +goose Up
ALTER TABLE request ADD COLUMN finish_reason INTEGER;

-- +goose Down
ALTER TABLE request DROP COLUMN IF EXISTS finish_reason;
```

## Step 2: Update sqlc queries

**`db/queries/request.sql`:**

- `UpdateRequestOnComplete`: add `finish_reason = $14` to SET clause.
- `ListRequests`: add `r.finish_reason` to the SELECT column list.
- `ListRequestsBySpan`: add `r.finish_reason` to the SELECT column list.
- `GetRequest`: already uses `SELECT *`, picks up the new column automatically.

Do NOT add `finish_reason` to `InsertRequest` — the column is nullable and defaults to NULL (request not finished).

Run `sqlc generate` to regenerate `pkg/db/`.

## Step 3: Go constants

Add to `pkg/db/request_constants.go`:

```go
const (
	FinishReasonInternal       = 1
	FinishReasonCancelled      = 2
	FinishReasonEOF            = 3
	FinishReasonHeadersTimeout = 4
	FinishReasonReadTimeout    = 5
)
```

## Step 4: Contract types

**`pkg/contract/request.go`:**

- Add `FinishReason *int32` field (json `"finishReason,omitempty"`) to `RequestView`.
- Add `FinishReason pgtype.Int4` to `requestLike`.
- In `toRequestView`, copy `FinishReason` if valid.
- In `ToRequestView`, `ToListRequestRowView`, `ToListRequestsBySpanRowView`: pass `FinishReason` from the DB row to `requestLike`.

## Step 5: Error classification helpers

**`pkg/server/gateway_helpers.go`:**

- Add `var errReadIdleTimeout = errors.New("gateway: read idle timeout")`. Update `idleTimeoutReader.Read` to wrap this sentinel instead of a bare `fmt.Errorf`.
- Add `classifyForwardError(err error, reqCtx context.Context) int32` helper:
  - `context.Canceled` + `reqCtx.Err() != nil` → `cancelled`
  - `net.Error` with `Timeout() == true` → `headers_timeout`
  - `context.DeadlineExceeded` → `headers_timeout`
  - Other → `internal`
- Update `completeFailedAttempt` signature to accept `finishReason int32` and pass it through to `UpdateRequestOnCompleteParams`.

## Step 6: Update error paths (`gateway_flow_errors.go`)

- `failMeta`: add `finishReason int32` parameter. Pass to `UpdateRequestOnCompleteParams`.
- Update all callers:
  - `failGatewayErrorWithFallback` → `internal`
  - `failHook` → `internal`
  - `failInternal` → `internal`
  - `failAllProviders` → check `f.ctxs.Request.Err() != nil`: if yes → `cancelled`, else → `internal`
  - `failSuccessPath` → `internal`
  - `openPathInternalReader` inline call → `internal`
  - `authenticateAndBackfill` inline call → `internal`

## Step 7: Update attempt paths (`gateway_flow_attempts.go`)

- `recordAttemptFailure`: add `finishReason int32` parameter, pass to `completeFailedAttempt`.
- `handleUpstreamNonOK`: pass `internal` to `updateRequestOnComplete`.
- `runSingleAttempt`:
  - After `forwardRequest` fails: call `classifyForwardError` on the error to get the finish reason, pass to `recordAttemptFailure`.
  - For `waitHookDelay` returning false (client cancelled): the state's LastErr is already set, `failAllProviders` handles the meta finish reason.
  - For hook errors: already goes through `failHook` → `internal`.
  - For build/insert errors: pass `internal` to `recordAttemptFailure`.

## Step 8: Update success path (`gateway_flow_success.go`)

- `pipePathResponse`: change return signature to also return `finishReason int32`. Capture the last `readErr` from the loop. After the loop, classify:
  - `io.EOF` → `eof`
  - `errors.Is(readErr, errReadIdleTimeout)` → `read_timeout`
  - `f.ctxs.Request.Err() != nil` → `cancelled`
  - Other → `eof`
- `completeGatewaySuccess`: add `finishReason int32` parameter. Pass to both upstream and meta `UpdateRequestOnCompleteParams`.
- `streamSuccess`: receive the `finishReason` from `pipePathResponse`, pass to `completeGatewaySuccess`.

## Step 9: Verify build

Run `go build ./...` to verify compilation.
