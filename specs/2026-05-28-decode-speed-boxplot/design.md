# Design: Decode Speed Box Plot

## Architecture

A single new endpoint `GET /overview/speed-boxplot` queries the raw `request` hypertable in real time. No new migrations, no continuous aggregates, no additional materialized views.

## Why Real-Time

Percentile functions (`percentile_cont`) are ordered-set aggregates that require the full set of individual values — they cannot be pre-computed in TimescaleDB continuous aggregates, which only support partial aggregates (SUM, COUNT, AVG, MIN, MAX). The raw `request` table is a TimescaleDB hypertable partitioned by `created_at`, so time-windowed queries prune efficiently to the relevant chunks.

## Query Strategy

A single SQL query with a CTE:

1. **CTE `speeds`**: filters the `request` table for eligible decode requests (same thresholds as `request_speed_hourly`) and computes per-request `decode_speed = output_tokens / ((time_spent_ms - ttft_ms) / 1000.0)`. Applies time window + optional dimension filters.
2. **Outer query**: groups by dimension key, runs `MIN`, `MAX`, `percentile_cont(0.25)`, `percentile_cont(0.5)`, `percentile_cont(0.95)`, and `COUNT` over `decode_speed`.

PostgreSQL's `percentile_cont` is an ordered-set aggregate available natively (no extensions needed beyond TimescaleDB). It interpolates linearly between values for non-integer percentile positions.

## Performance Expectations

- The `request` hypertable has a composite index on `(created_at DESC, id DESC)`.
- TimescaleDB chunk pruning eliminates partitions outside the time window.
- The `type = 1` and threshold filters significantly reduce the working set.
- For 30-day windows with tens of thousands of qualifying requests, the sort + percentile computation should complete in tens of milliseconds on PostgreSQL 17.

## Response Shape

Flat array of box plot items, one per dimension group. Each item contains `key`, `label`, `min`, `p25`, `median`, `p95`, `max`, `count`. The `count` field helps the frontend decide whether to display a group (e.g., skip groups with < 5 data points).

## Dashboard Integration

Replace the current `OverviewSpeedTimeline` component (which computes a fake median from `(min+max)/2` based on hourly aggregate data) with a new component that consumes the boxplot endpoint. The new component renders a proper ECharts horizontal boxplot with whiskers at min/max, box edges at P25/P95, and a median line.
