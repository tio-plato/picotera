# Decode Speed Box Plot Statistics

## Requirements

For model output speed (decode speed), add a dedicated API endpoint that returns box plot statistics:

- **Metrics**: min, max, median (P50), P25, P95
- **Sliding windows**: 24h (`1d`), 7 days (`7d`), 30 days (`1m`)
- **Dimensions**: same as existing overview — `apiKey`, `model`, `upstreamModel`, `provider`, `project`, `none`
- **Filters**: same optional filters — `apiKeyId`, `model`, `upstreamModel`, `providerId`, `projectId`

## Data Source

Query the raw `request` hypertable directly (not the `request_speed_hourly` continuous aggregate), because percentile statistics (P25, P50, P95) cannot be pre-aggregated in materialized views — they require access to individual data points.

Per-request decode speed is calculated as:
```
decode_speed = output_tokens / ((time_spent_ms - ttft_ms) / 1000.0)  -- tokens/sec
```

Apply the same filtering thresholds as `request_speed_hourly`:
- `type = 1` (upstream requests only)
- `output_tokens >= 50`
- `ttft_ms IS NOT NULL AND time_spent_ms IS NOT NULL`
- `(time_spent_ms - ttft_ms) >= 500` (at least 500ms decode time)

## Output Format

Return box plot data per dimension group, suitable for rendering an ECharts boxplot:
```json
{
  "window": { "range": "1d", "startAt": "...", "endAt": "...", "bucket": "hour" },
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

## Approach

Use PostgreSQL's native ordered-set aggregate functions (`percentile_cont`) for real-time exact computation directly against the `request` hypertable. No new migrations or continuous aggregates needed.
