# Design: Overview Speed Metrics

## 数据层

### 新增 continuous aggregate `request_speed_hourly`

新建独立的 continuous aggregate，按 1 小时分桶，物化 prefill/decode 速度的平均值。

物化分子分母的 SUM，查询时再除得到加权平均速度。两种速度的过滤条件不同，用 CASE WHEN 区分。

```sql
CREATE MATERIALIZED VIEW request_speed_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  model,
  upstream_model,
  provider_id,
  api_key_id,
  project_id,
  -- prefill: 分子 = input_tokens, 分母 = ttft_ms
  SUM(CASE
    WHEN input_tokens >= 200 AND ttft_ms >= 2000
    THEN input_tokens::float8
  END) AS prefill_token_sum,
  SUM(CASE
    WHEN input_tokens >= 200 AND ttft_ms >= 2000
    THEN ttft_ms::float8
  END) AS prefill_time_sum,
  -- decode: 分子 = output_tokens, 分母 = (time_spent_ms - ttft_ms)
  SUM(CASE
    WHEN output_tokens >= 200
      AND ttft_ms IS NOT NULL
      AND time_spent_ms IS NOT NULL
      AND (time_spent_ms - ttft_ms) >= 2000
    THEN output_tokens::float8
  END) AS decode_token_sum,
  SUM(CASE
    WHEN output_tokens >= 200
      AND ttft_ms IS NOT NULL
      AND time_spent_ms IS NOT NULL
      AND (time_spent_ms - ttft_ms) >= 2000
    THEN (time_spent_ms - ttft_ms)::float8
  END) AS decode_time_sum
FROM request
WHERE type = 1
GROUP BY bucket_at, model, upstream_model, provider_id, api_key_id, project_id
WITH NO DATA;

ALTER MATERIALIZED VIEW request_speed_hourly
  SET (timescaledb.materialized_only = false);
```

查询时 `SUM(prefill_token_sum) / (SUM(prefill_time_sum) / 1000.0)` 得到 tokens/sec。跨多桶 SUM 自然就是加权平均，权重为各桶的分母（时间）。

Continuous aggregate policy 与 `request_overview_hourly` 一致：`start_offset = 35 days`, `end_offset = 5 minutes`, `schedule_interval = 5 minutes`。

> **精度说明**：跨多桶聚合时，`AVG(小时平均)` 等于所有小时桶的简单平均，每桶权重相同。

### SQL 查询

新增 `ListOverviewSpeedSeries` 查询，直接查 `request_speed_hourly` 视图。查询时做除法，分母为 0 或全 NULL 时过滤掉。

```sql
-- name: ListOverviewSpeedSeries :many
SELECT
  bucket_at::timestamp AS bucket_at,
  CASE sqlc.arg('dimension')::text
    WHEN 'model' THEN COALESCE(model, '')
    WHEN 'upstreamModel' THEN COALESCE(upstream_model, '')
    WHEN 'provider' THEN COALESCE(provider_id::text, '')
    WHEN 'apiKey' THEN COALESCE(api_key_id::text, '')
    WHEN 'project' THEN COALESCE(project_id::text, '')
    ELSE ''
  END AS group_key,
  SUM(prefill_token_sum) / (SUM(prefill_time_sum) / 1000.0) AS prefill_speed,
  SUM(decode_token_sum) / (SUM(decode_time_sum) / 1000.0) AS decode_speed
FROM request_speed_hourly
WHERE bucket_at >= sqlc.arg('start_at')::timestamp
  AND bucket_at < sqlc.arg('end_at')::timestamp
  AND (sqlc.narg('api_key_id')::int IS NULL OR api_key_id = sqlc.narg('api_key_id')::int)
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model')::text)
  AND (sqlc.narg('upstream_model')::text IS NULL OR upstream_model = sqlc.narg('upstream_model')::text)
  AND (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id')::int)
  AND (sqlc.narg('project_id')::int IS NULL OR project_id = sqlc.narg('project_id')::int)
GROUP BY bucket_at, group_key
HAVING SUM(prefill_time_sum) > 0 OR SUM(decode_time_sum) > 0
ORDER BY bucket_at ASC, group_key ASC;
```

视图存 SUM（分子分母），查询做除法，`HAVING SUM(time_sum) > 0` 过滤无数据的桶。

## API 层

### 复用现有 series endpoint

在 `GetOverviewSeries` 响应的 `points` 数组中，新增两个 metric 类型：

- `metric: "prefillSpeed"` — prefill 速度 (tokens/sec)
- `metric: "decodeSpeed"` — decode 速度 (tokens/sec)

与现有 `tokens`、`requests`、`traces`、`cost` 并列，前端按 metric 名过滤即可。无需新增 endpoint 或修改 contract 响应类型（metric 字段是 string 类型，已有扩展性）。

Go 层不做速度计算，直接将查询结果（已是 tokens/sec）写入 points。

### 维度

speed 系列仅在 `dimension != 'none'` 时有意义（因为需要 group_key 做分组折线）。当 `dimension = 'none'` 时，仍计算整体加权平均（group_key = ''）。

## Dashboard 层

### 新增 OverviewLineChart 组件

`dashboard/src/components/charts/OverviewLineChart.vue`

使用 Unovis `VisLine`（非 `VisArea`），结构与 `OverviewAreaStack.vue` 完全对齐：
- 同样的 `SeriesGroup`、`SeriesPoint` 接口
- 同样的 bucket/group 映射逻辑
- 同样的 tooltip 和 legend
- 仅将 `VisArea` 替换为 `VisLine`

### OverviewView.vue 修改

新增独立的速度统计区域（位于现有 Series 区域之后），包含：

1. 自己的 `speedDimension` ref（`OverviewSeriesDimension` 类型），带 SegmentedControl 维度选择器，选项同 series 区域
2. 新增 `speedSeriesQuery`，调用 `getOverviewSeries(overviewFilters, speedDimension)` — 与 series 区域使用相同 API，独立 queryKey
3. 从响应中过滤出 `prefillSpeed` 和 `decodeSpeed` 两个 metric，分别画折线图
4. 两个 DataCard：
   - **Prefill 速度** — `OverviewLineChart`，数据来自 `speedSeriesData.points.filter(p => p.metric === 'prefillSpeed')`
   - **Decode 速度** — `OverviewLineChart`，数据来自 `speedSeriesData.points.filter(p => p.metric === 'decodeSpeed')`
5. valueFormat 显示为 `tokens/sec`（如 `1.2k tok/s`）

### API 调用

速度数据嵌入现有 `getOverviewSeries` 响应。仅需在 `handleGetOverviewSeries` 中追加一次 SQL 调用并合并 points。前端新增一个 `useQuery` 调用相同 endpoint 但使用独立的 dimension 和 queryKey。
