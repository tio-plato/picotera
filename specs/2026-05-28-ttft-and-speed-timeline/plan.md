# 执行计划

## Step 1: 新增 migration — 重建 `request_speed_hourly`

文件：`db/migrations/028_request_speed_hourly_add_count.sql`

内容：
- `-- +goose NO TRANSACTION`
- Up: 移除策略 → 删除视图 → 用原有定义 + `prefill_request_count` 列重建 → 设置 `materialized_only = false` → 添加策略
- Down: 移除策略 → 删除视图 → 用原始定义（无 count）重建 → 设置 `materialized_only = false` → 添加策略

`prefill_request_count` 定义：
```sql
COUNT(CASE WHEN input_tokens >= 50 AND ttft_ms >= 500 THEN 1 END) AS prefill_request_count
```

## Step 2: 修改 `ListOverviewSpeedSeries` 查询

文件：`db/queries/overview.sql`

在 `ListOverviewSpeedSeries` 查询的 SELECT 中新增：
```sql
COALESCE((SUM(prefill_time_sum) / NULLIF(SUM(prefill_request_count), 0))::float8, 0)::float8 AS avg_ttft
```

## Step 3: 运行 `sqlc generate`

重新生成 `pkg/db/` 下的 Go 代码，使 `ListOverviewSpeedSeriesRow` 结构体包含 `AvgTtft` 字段。

## Step 4: 后端 handler 添加 `avgTtft` metric

文件：`pkg/server/handle_overview.go`

在 `handleGetOverviewSeries` 函数中：
1. 新增 `avgTtftByBG` map（与 `prefillSpeedByBG` 同级）
2. 在遍历 `speedRows` 时填充 `avgTtftByBG`
3. 在组装 points 时，仿照 `prefillSpeed` 的逻辑，新增 `avgTtft` metric point

## Step 5: 重新生成 OpenAPI spec

运行 `mise run openapi` 更新 `openapi.yaml`（metric 字段是 string，无需修改 contract 类型）。

## Step 6: 重新生成前端 OpenAPI 类型

运行 `pnpm --dir dashboard generate-openapi`。

## Step 7: 前端 — 新增 `OverviewSpeedTimeline.vue` 组件

文件：`dashboard/src/components/charts/OverviewSpeedTimeline.vue`

Props：
- `groups: SeriesGroup[]` — 分组列表
- `points: SeriesPoint[]` — decode speed 数据点（与折线图相同）
- `valueFormat?: (v: number) => string` — 速度格式化函数

实现：
- 对每个 group 遍历 points，计算 min/max speed
- 生成 timeline dataset，每条记录 `{ row: group.label, x: minSpeed, length: maxSpeed - minSpeed, color }`
- 使用 `VisXYContainer` + `VisTimeline`（`lineRow` 按 group label 分行，`x` 为 min speed，`lineDuration` 为 max-min）+ `VisAxis`（x 轴格式化为 tok/s）
- 显示 row labels（`showRowLabels: true`）
- Tooltip 显示该 group 的 min、max 值

## Step 8: 前端 — 修改 `OverviewView.vue`

1. 导入 `OverviewSpeedTimeline` 组件
2. 新增 `seriesAvgTtft` computed，从 `speedSeriesData.points` 过滤 `metric === 'avgTtft'`
3. 新增 `formatTtft` 函数：≥1000ms 显示为 `X.X s`，否则 `X ms`
4. 在模板中速度统计的两张折线图下方新增：
   - TTFT 平均时间折线图：全宽 `DataCard`，使用 `OverviewLineChart`，传入 `seriesAvgTtft`、`formatTtft`
   - Decode 速度范围 Timeline：全宽 `DataCard`，使用 `OverviewSpeedTimeline`，传入 `speedGroups`、`seriesDecodeSpeed`、`formatSpeed`
