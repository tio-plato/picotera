# Request TimescaleDB Partitioning API

## GET /api/picotera/requests

Adds required query parameters:

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `createdAtFrom` | RFC3339 timestamp | yes | Inclusive lower bound for request `created_at`. |
| `createdAtTo` | RFC3339 timestamp | yes | Exclusive upper bound for request `created_at`. |

Existing filters remain:

- `type`
- `providerId`
- `endpointPath`
- `model`
- `upstreamModel`
- `parentSpanId`
- `cursor`
- `limit`

Validation:

- `createdAtFrom` and `createdAtTo` must parse as RFC3339 or RFC3339Nano timestamps.
- `createdAtFrom` must be strictly earlier than `createdAtTo`.
- Invalid timestamp values return `400 Bad Request`.
- The server does not trim or normalize query values.

Cursor behavior:

- Cursor encoding remains based on `createdAt` and `id`.
- Cursors are valid only within the same requested time window.

## GET /api/picotera/request-traces

Adds required query parameters:

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `createdAtFrom` | RFC3339 timestamp | yes | Inclusive lower bound for trace member requests. |
| `createdAtTo` | RFC3339 timestamp | yes | Exclusive upper bound for trace member requests. |

Existing parameters remain:

- `cursor`
- `limit`

Validation matches `GET /api/picotera/requests`.

Trace totals are calculated only from request rows inside the requested time window. `lastRequestAt` is the latest request timestamp inside that same window.

## GET /api/picotera/requests/{id}

The API shape does not change.

The server parses `{id}` as a strict xid and derives the exact `created_at` lookup value from the encoded timestamp. Invalid xid values return `400 Bad Request`. Unknown valid xids return `404 Not Found`.

## GET /api/picotera/requests/{id}/spans

Adds optional query parameters:

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `createdAtFrom` | RFC3339 timestamp | no | Inclusive lower bound for returned span rows. |
| `createdAtTo` | RFC3339 timestamp | no | Exclusive upper bound for returned span rows. |

If the range is omitted, the server uses the anchor request's decoded xid timestamp and a bounded default trace window around it. The implementation will use a fixed 24-hour lookback and 24-hour lookahead:

```text
[anchorCreatedAt - 24h, anchorCreatedAt + 24h)
```

When supplied, both parameters are required together and validation matches `GET /api/picotera/requests`.

The anchor request itself is found from the xid-derived timestamp before span expansion.
