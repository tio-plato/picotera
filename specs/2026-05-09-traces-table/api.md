# API — Traces Table

## `GET /api/picotera/request-traces`

Lists materialized traces ordered by latest request activity.

### Query

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `limit` | integer | no | Page size. Defaults to `20`; maximum follows the existing pagination contract. |
| `cursor` | string | no | Cursor from the previous response. |

### Response

```json
{
  "items": [
    {
      "id": "d4hg9b4r3l6v1m2n8p0g",
      "parentSpanId": "sid-abc",
      "metaRequestCount": 1,
      "upstreamRequestCount": 3,
      "totalTokens": 124500,
      "inputTokens": 32000,
      "cacheReadTokens": 2000,
      "outputTokens": 83000,
      "cacheWriteTokens": 1000,
      "cacheWrite1hTokens": 6500,
      "modelCosts": [
        { "currency": "USD", "amount": 0.91 }
      ],
      "upstreamCosts": [
        { "currency": "USD", "amount": 0.73 }
      ],
      "firstRequestAt": "2026-05-09T08:12:30Z",
      "lastRequestAt": "2026-05-09T08:12:34Z",
      "userMessagePreview": "Summarize this trace"
    }
  ],
  "pagination": {
    "hasMore": true,
    "nextCursor": "eyJsYXN0UmVxdWVzdEF0Ijoi...\""
  }
}
```

### Cursor

Cursor payload fields:

- `lastRequestAt`
- `traceId`

The backend applies:

```sql
(traces.last_request_at, traces.id) < (:last_request_at, :trace_id)
```

with ordering:

```sql
ORDER BY traces.last_request_at DESC, traces.id DESC
```

## `GET /api/picotera/requests`

Lists request rows. Trace filtering uses the internal trace id from `GET /api/picotera/request-traces`.

### Trace Query

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `traceId` | string | no | Exact internal xid trace id. When present, the server resolves `traces.id`, then filters requests by the trace row's `parent_span_id`, `first_request_at`, and `last_request_at`. |

### Behavior

- `traceId` is an exact xid string. Invalid values are rejected by backend validation.
- A missing `traceId` does not apply trace filtering.
- An unknown `traceId` returns an empty list.
- `traceId` can be combined with the existing request filters such as `type`, `providerId`, `endpointPath`, `model`, and `upstreamModel`.
- The dashboard no longer uses `parentSpanId` for trace-to-request navigation.

## Response Type Changes

`RequestTraceView` adds:

| Field | Type | Description |
| --- | --- | --- |
| `id` | string | Internal xid trace id from `traces.id`. |
| `firstRequestAt` | string | Earliest request timestamp recorded for this trace. |
| `lastRequestAt` | string | Latest request timestamp recorded for this trace. |

Existing fields remain:

- `parentSpanId`
- `metaRequestCount`
- `upstreamRequestCount`
- token totals
- `modelCosts`
- `upstreamCosts`
- `userMessagePreview`
