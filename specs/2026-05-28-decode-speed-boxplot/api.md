# API: Decode Speed Box Plot

## `GET /api/picotera/overview/speed-boxplot`

Returns box plot statistics for decode speed (output tokens/sec), grouped by a selected dimension.

### Query Parameters

| Parameter       | Type    | Required | Description                                                 |
|-----------------|---------|----------|-------------------------------------------------------------|
| `range`         | string  | yes      | Time window. One of: `1d`, `7d`, `1m`                      |
| `dimension`     | string  | yes      | Grouping dimension. One of: `none`, `apiKey`, `model`, `upstreamModel`, `provider`, `project` |
| `apiKeyId`      | integer | no       | Filter by API key ID                                        |
| `model`         | string  | no       | Filter by model name                                        |
| `upstreamModel` | string  | no       | Filter by upstream model name                               |
| `providerId`    | integer | no       | Filter by provider ID                                       |
| `projectId`     | integer | no       | Filter by project ID                                        |

### Response `200`

```json
{
  "window": {
    "range": "1d",
    "startAt": "2026-05-27T09:00:00Z",
    "endAt": "2026-05-28T09:00:00Z",
    "bucket": "hour"
  },
  "dimension": "model",
  "items": [
    {
      "key": "claude-sonnet-4-20250514",
      "label": "claude-sonnet-4-20250514",
      "min": 42.5,
      "p25": 55.0,
      "median": 68.3,
      "p95": 120.0,
      "max": 185.0,
      "count": 1234
    }
  ]
}
```

### Field Definitions

- `min`: minimum decode speed in tokens/sec across all qualifying requests in the window
- `p25`: 25th percentile (first quartile) of decode speed
- `median`: 50th percentile (median) of decode speed
- `p95`: 95th percentile of decode speed
- `max`: maximum decode speed
- `count`: number of qualifying requests used in the calculation

### Qualifying Request Criteria

A request is included in the box plot calculation when all of the following are true:
- `type = 1` (upstream provider request)
- `status = 2` (completed)
- `output_tokens >= 50`
- `ttft_ms IS NOT NULL AND time_spent_ms IS NOT NULL`
- `(time_spent_ms - ttft_ms) >= 500` (decode phase lasted at least 500ms)
