# Plan: Cache Hit Rate Chart

## Step 1: 新增 SQL 查询

文件：`db/queries/overview.sql`

新增 `ListOverviewCacheHitRateSeries`：

- 从 `request_overview_hourly` 查询
- 动态维度 CASE 与 `ListOverviewSeriesMetrics` 保持一致
- WHERE 复用 Overview 的时间范围和五个过滤条件
- SELECT 返回：
  - `bucket_at`
  - `group_key`
  - `SUM(cache_read_tokens)::float8 AS cache_read_token_sum`
  - `SUM(input_tokens + cache_read_tokens + cache_write_tokens + cache_write_1h_tokens)::float8 AS input_token_sum`
- `HAVING SUM(input_tokens + cache_read_tokens + cache_write_tokens + cache_write_1h_tokens) > 0`
- `ORDER BY bucket_at ASC, group_key ASC`

## Step 2: 运行 sqlc

```bash
sqlc generate
```

生成 `ListOverviewCacheHitRateSeries` 的 Go 类型与查询方法。

## Step 3: 修改 Overview series handler

文件：`pkg/server/handle_overview.go`

在 `handleGetOverviewSeries` 中：

- 调用 `s.queries.ListOverviewCacheHitRateSeries`
- 复用 `startTS`, `endTS`, `toPgInt4`, `toPgText` 参数构造
- 遍历返回行，跳过无效 bucket 或 `InputTokenSum <= 0` 的行
- 计算 `rate := CacheReadTokenSum / InputTokenSum`
- 用 `overviewBucketAt` 对齐前端 bucket
- 调用 `addGroup(group)`
- 存入 `cacheHitRateByBG`
- 生成 points 时追加：
  - `Metric: "cacheHitRate"`
  - `BucketAt: bucket`
  - `GroupKey: group`
  - `Value: rate`
  - `Currency: ""`

只为 SQL 返回的有效桶追加点，不补 0。

## Step 4: 调整前端 query key

文件：`dashboard/src/api/queryKeys.ts`

在 `overview` 对象中新增：

```ts
cacheHitRate: (f: OverviewFilters, dim: OverviewSeriesDimension) =>
  ['overview', 'cacheHitRate', dim, { ...f }] as const
```

## Step 5: 复用 OverviewLineChart 缺失点行为

文件：`dashboard/src/components/charts/OverviewLineChart.vue`

无需修改。该组件已经保留 `Datum.values` 的 `undefined` 缺失语义，并在 `VisLine` 上启用 `interpolateMissingData`。Unovis 会自动用虚线连接跨缺失桶的线段。

## Step 6: 修改 OverviewView.vue

文件：`dashboard/src/views/OverviewView.vue`

新增缓存命中率区域：

- 新增 `cacheHitRateDimension = ref<OverviewSeriesDimension>('model')`
- 新增 `cacheHitRateSeriesQuery`
- 新增 computed：
  - `cacheHitRateSeriesData`
  - `cacheHitRateGroups`
  - `cacheHitRateBuckets`
  - `seriesCacheHitRate`
- 新增 `formatPercent(v)`，将 `0.624` 显示为 `62.4%`
- `refreshAll` 增加 `cacheHitRateSeriesQuery.refetch()`
- `isRefreshing` 增加 `cacheHitRateSeriesQuery.isFetching.value`
- 在速度统计区域之后新增一块：
  - 标题“缓存命中率”
  - 维度 `SegmentedControl`
  - 一个 `DataCard`
  - 使用 `OverviewLineChart`

## Step 7: 重新生成 OpenAPI 和 dashboard 类型

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

## Step 8: 验证

运行：

```bash
go test ./pkg/server/...
pnpm --dir dashboard type-check
pnpm --dir dashboard lint
pnpm --dir dashboard build
```

若本地缺少数据库、TinyGo、Node 依赖或服务环境导致某项无法运行，在最终说明中记录具体失败原因。
