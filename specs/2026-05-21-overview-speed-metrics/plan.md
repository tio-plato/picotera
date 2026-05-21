# Plan: Overview Speed Metrics

## Step 1: 新增 migration

文件：`db/migrations/026_request_speed_hourly_cagg.sql`

创建 `request_speed_hourly` continuous aggregate：
- 按 1 小时分桶，GROUP BY `bucket_at, model, upstream_model, provider_id, api_key_id, project_id`
- 物化 prefill/decode 的分子（token SUM）和分母（time SUM），各自 CASE WHEN 含阈值过滤
- `timescaledb.materialized_only = false`（实时查询最近数据）
- 添加 continuous aggregate policy（与 `request_overview_hourly` 一致）

```sql
-- +goose NO TRANSACTION
-- +goose Up
CREATE MATERIALIZED VIEW request_speed_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  model,
  upstream_model,
  provider_id,
  api_key_id,
  project_id,
  SUM(CASE
    WHEN input_tokens >= 200 AND ttft_ms >= 2000
    THEN input_tokens::float8
  END) AS prefill_token_sum,
  SUM(CASE
    WHEN input_tokens >= 200 AND ttft_ms >= 2000
    THEN ttft_ms::float8
  END) AS prefill_time_sum,
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

SELECT add_continuous_aggregate_policy(
  'request_speed_hourly',
  start_offset      => INTERVAL '35 days',
  end_offset        => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);

-- +goose Down
SELECT remove_continuous_aggregate_policy('request_speed_hourly', if_exists => true);
DROP MATERIALIZED VIEW IF EXISTS request_speed_hourly;
```

## Step 2: 新增 SQL 查询

文件：`db/queries/overview.sql`

追加 `ListOverviewSpeedSeries` 查询，查 `request_speed_hourly` 视图：
- SELECT `bucket_at`, `group_key`（CASE WHEN 动态维度）, `SUM(token_sum) / (SUM(time_sum) / 1000.0)` 得到加权平均 tokens/sec
- WHERE 过滤 bucket_at 范围 + 5 个过滤条件
- GROUP BY bucket_at, group_key
- HAVING `SUM(time_sum) > 0` 排除无数据的桶
- ORDER BY bucket_at ASC, group_key ASC

## Step 3: 运行 sqlc generate

```bash
sqlc generate
```

生成 `ListOverviewSpeedSeries` 对应的 Go 代码到 `pkg/db/`。

## Step 4: 修改 handleGetOverviewSeries

文件：`pkg/server/handle_overview.go`

在 `handleGetOverviewSeries` 函数中：

1. 调用 `s.queries.ListOverviewSpeedSeries`，传入与 `ListOverviewSeriesMetrics` 相同的过滤参数
2. 遍历返回行，跳过 NULL 的 `PrefillSpeed` / `DecodeSpeed`
3. 将非 NULL 值追加到 `points` slice，metric 为 `"prefillSpeed"` 和 `"decodeSpeed"`
4. 对应的 group_key 通过 `addGroup` 注册

## Step 5: 重新生成 OpenAPI spec

```bash
mise run openapi
```

Contract 未变（仅新增 string metric 值），但确保 spec 是最新的。

## Step 6: Dashboard — 新增 OverviewLineChart 组件

文件：`dashboard/src/components/charts/OverviewLineChart.vue`

从 `OverviewAreaStack.vue` 复制，将 `VisArea` 替换为 `VisLine`。保留相同的 props 接口和数据映射逻辑。

## Step 7: Dashboard — 新增 queryKey

文件：`dashboard/src/api/queryKeys.ts`

在 `overview` 对象中新增：
- `speed: (f: OverviewFilters, dim: OverviewSeriesDimension) => ['overview', 'speed', dim, { ...f }] as const`

## Step 8: Dashboard — OverviewView.vue 添加速度图表

文件：`dashboard/src/views/OverviewView.vue`

1. 导入 `OverviewLineChart`
2. 新增 `speedDimension` ref，类型 `OverviewSeriesDimension`，默认 `'model'`
3. 新增 SegmentedControl 维度选择器（选项同 series 区域），标签 "速度统计"
4. 新增 `speedSeriesQuery`，调用 `getOverviewSeries(overviewFilters, speedDimension)`，独立 queryKey `queryKeys.overview.speed(overviewFilters, speedDimension)`
5. 新增 computed：
   - `speedGroups` — 从 speedSeriesData 提取 groups
   - `speedBuckets` — 从 speedSeriesData 提取 buckets
   - `seriesPrefillSpeed` — 从 `speedSeriesData.points` 过滤 `metric === 'prefillSpeed'`
   - `seriesDecodeSpeed` — 从 `speedSeriesData.points` 过滤 `metric === 'decodeSpeed'`
6. 新增 valueFormat 函数 `formatSpeed(v)` — 显示为 `1.2k tok/s` 格式
7. 在 Series 区域之后新增速度统计区域：
   - 维度选择器 SegmentedControl
   - grid 包含两个 DataCard："Prefill 速度" 和 "Decode 速度"

## Step 9: 验证

1. `mise run openapi` — 确保 spec 正确
2. `pnpm --dir dashboard generate-openapi` — 确保类型同步（类型无变化，但保持流程完整）
3. `pnpm --dir dashboard type-check` — 确保无类型错误
4. `pnpm --dir dashboard lint` — 确保 lint 通过
5. `pnpm --dir dashboard build` — 确保构建通过
