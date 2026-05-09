# Plan

## 1. 数据库迁移

1. 新建 `db/migrations/019_request_overview_hourly_cagg.sql`，按 design 中给出的 SQL 创建连续聚合 `request_overview_hourly`，启用实时聚合并添加刷新策略。
2. 在文件顶部加 `-- +goose NO TRANSACTION`，避免 `CREATE MATERIALIZED VIEW ... WITH (timescaledb.continuous)` 在事务内失败。
3. Down 段移除策略并 `DROP MATERIALIZED VIEW IF EXISTS`。
4. 启动一次后台进程 `mise run server` 验证迁移落库；连上 psql 执行 `\d+ request_overview_hourly` 与 `SELECT * FROM timescaledb_information.continuous_aggregates;` 确认。

## 2. sqlc 查询

1. 新建 `db/queries/overview.sql`，加入：
   - `:one` `GetOverviewTotals` —— 在 `request_overview_hourly` 上 `SUM` 出 `total_tokens` / `request_count` 与 `[(currency, amount)]`（用 `jsonb_agg(jsonb_build_object(...) ORDER BY currency)`）；半开区间 `[startAt, endAt)`；filters 用 `sqlc.narg`。
   - `:one` `CountTraces` —— 无过滤分支；只看 `traces.last_request_at`。
   - `:one` `CountTracesFiltered` —— 多过滤分支；用 `EXISTS` 联 `request`。
   - `:many` `ListOverviewDistribution` —— 接 `dimension` 字符串参数，用 `CASE` 选维度列；返回 `key`、`total_tokens`、`request_count`、`costs jsonb`、（trace_count 由后端再调 distribution 专用 trace 查询补上）。
   - `:many` `ListOverviewTraceCountsByDimension` —— 用 `request` 表（不能用 cagg），按维度返回 `parent_span_id` 命中的 trace 数（`COUNT(DISTINCT parent_span_id)`），与 distribution 合并。
   - `:many` `ListOverviewSeriesMetrics` —— `generate_series` × `LEFT JOIN cagg`；返回每个桶 × 维度的 `total_tokens`、`request_count`、`costs jsonb`。
   - `:many` `ListOverviewSeriesTraces` —— `generate_series` × 子查询：每个桶里命中过滤的 trace 数（`COUNT(DISTINCT parent_span_id)`，按 `traces.last_request_at` 落桶；带分组时按 `request` 行的维度归桶）。
2. 跑 `sqlc generate`，确认 `pkg/db/overview.sql.go` 与 `Querier` 接口同步更新。

## 3. 后端契约

1. 新建 `pkg/contract/overview.go`：
   - 类型：`OverviewRange`、`OverviewDimension`、`OverviewSeriesDimension`、`OverviewMetric`、`OverviewCostView`、`OverviewWindowView`、`OverviewSummaryView`、`OverviewDistributionRowView`、`OverviewSeriesGroupView`、`OverviewSeriesPointView`、`OverviewSeriesView`、`OverviewDistributionView`。
   - `Get*Request` / `Get*Response` 三组：summary / distribution / series。
   - `Operation*` 三个 Huma operation 元数据；路径 `/overview/summary`、`/overview/distribution`、`/overview/series`，均 `GET`。
2. Huma `enum` tag 限制枚举；`apiKeyId` / `providerId` 用 `minimum:"1"`；`model` / `upstreamModel` 用 `minLength:"1"`；缺省即不过滤。
3. 在 `cmd/picotera/main.go` 之外、`pkg/server/server.go` 的 `registerOperations()` 注册三个 operation。

## 4. 处理器

1. 新建 `pkg/server/handle_overview.go`：
   - `windowFor(range)` 返回 `(startAt, endAt time.Time)`，全部对齐到整点，UTC。
   - `summary`：调 `GetOverviewTotals` + `CountTraces` 或 `CountTracesFiltered`，组装 `OverviewSummaryView`。
   - `distribution`：调 `ListOverviewDistribution` + `ListOverviewTraceCountsByDimension`，按 `key` merge，按 `totalTokens DESC, key ASC` 排序。
   - `series`：调 `ListOverviewSeriesMetrics` + `ListOverviewSeriesTraces`，规整成 `points` 列表；构造 `groups`（按出现顺序）和 `buckets`（直接从 `generate_series` 结果取）。
   - `costs` 字段从 jsonb 反序列化为 `[]OverviewCostView`，币种空串行直接丢。
2. 加入小型纯函数单测：`windowFor` 边界；过滤参数构造（确保不 trim）。放 `pkg/server/handle_overview_test.go`。

## 5. 重新生成 OpenAPI / TS 类型

1. `mise run openapi` —— 写 `openapi.yaml`，确认包含三个 `getOverview*` operation。
2. `pnpm --dir dashboard generate-openapi` —— 写 `dashboard/src/openapi-types.d.ts`。
3. `dashboard/src/api/index.ts` 重新导出 `OverviewSummaryView`、`OverviewDistributionView`、`OverviewSeriesView`、`OverviewWindowView`、`OverviewCost*View`、维度/枚举类型。

## 6. 前端依赖与基础设施

1. `pnpm --dir dashboard add @unovis/vue@^1.6.5 @unovis/ts@^1.6.5`。
2. `dashboard/src/main.ts`：`import '@unovis/ts/styles/index.css'`。
3. 在 `dashboard/src/ui/icons/paths.ts` 注册 `chart-pie`（来自 `@tabler/icons-vue`）。

## 7. 数据层 wiring

1. `dashboard/src/api/queryKeys.ts`：
   ```ts
   export type OverviewFilters = Readonly<{
     range: '1d' | '7d' | '1m'
     apiKeyId?: number
     model?: string
     upstreamModel?: string
     providerId?: number
   }>
   // ...
   overview: {
     all: ['overview'] as const,
     summary: (f: OverviewFilters) => ['overview', 'summary', { ...f }] as const,
     distribution: (f: OverviewFilters, dim: string) =>
       ['overview', 'distribution', dim, { ...f }] as const,
     series: (f: OverviewFilters, dim: string) =>
       ['overview', 'series', dim, { ...f }] as const,
   }
   ```
2. `dashboard/src/api/client.ts`：加 `getOverviewSummary`、`getOverviewDistribution`、`getOverviewSeries`，沿用 `ApiRequestError` 错误风格。

## 8. Chart wrapper 组件

1. `dashboard/src/components/charts/OverviewDonut.vue`：包 `VisSingleContainer` + `VisDonut`，props = `data: { key: string; label: string; value: number }[]`，`valueFormat?`。内置图例（`Tag` + `text-2xs`），点击图例切换可见。
2. `dashboard/src/components/charts/OverviewAreaStack.vue`：包 `VisXYContainer` + `VisStackedArea` + `VisAxis x` + `VisAxis y` + `VisTooltip` + `VisCrosshair`。props = `groups: { key, label }[]`、`buckets: string[]`、`points: { groupKey, bucketAt, value }[]`、`valueFormat?`、`currency?`。空数据由 `StateText` 包裹卡内显示。
3. 颜色函数 `groupColor(index)` 从 `var(--color-accent)`、`var(--color-ok)`、`var(--color-warn)`、`var(--color-err)` 循环；超过 4 组叠加 OKLCH hue 偏移。

## 9. View

1. 新建 `dashboard/src/views/OverviewView.vue`。
2. 顶部控制条：
   - `SegmentedControl` for range（默认 `1d`）。
   - 4 个过滤 `Select`（API Key / 实际模型 / 上游模型 / 渠道），候选项分别来自 `listApiKeys`、`listProviders`、扁平化的模型列表。无值时 disabled。
3. Bento 区：4 列 `DataCard`，每张卡内：上方 kicker 标题，下方大号数字（`text-xl`，mono，`tabular-nums`）。费用按币种纵向罗列，每行用 `MoneyDisplay`。
4. 分布区：维度 `SegmentedControl`（默认 `provider`）+ 2 列 `DataCard`，左 token donut，右 cost donut。
5. 趋势区：聚合维度 `SegmentedControl`（默认 `none`）+ 2 列网格 4 张卡，分别画 tokens / cost / requests / traces 的 stacked area。
6. 三个 `useQuery` 独立挂载；切换 range 或 filter 同时 invalidate 三者；切换分布维度只 invalidate distribution；切换聚合维度只 invalidate series。
7. 每张卡内独立 `loading` / `error` / `empty` 处理（`StateText`）。

## 10. 路由 / 侧边栏 / 首屏

1. `dashboard/src/router/index.ts`：新增 `overview` 路由；根 `/` 重定向改为 `/overview`。
2. `dashboard/src/App.vue`：在 `pageMeta` 加 `overview: { title: '概览', hint: '过去窗口内的请求、token、费用与追踪一览' }`。
3. `dashboard/src/components/AppSidebar.vue`：在「监控」（或现有顶部分组）下新增「概览」入口，使用新注册的 `chart-pie` 图标。

## 11. 验证

1. `docker compose up -d` 启动 TimescaleDB 与 KeyDB / MinIO。
2. `go build -o picotera ./cmd/picotera` —— 确保后端编译通过。
3. `go test ./pkg/server ./pkg/llmbridge` —— 现有测试不退步；新加 handler 单测应通过。
4. `pnpm --dir dashboard type-check`。
5. `pnpm --dir dashboard lint`。
6. `pnpm --dir dashboard build`。
7. 启动前后端，浏览器手测：
   - 默认 `1d` 无过滤，三个端点均返回；bento 数字与 distribution / series 加和一致。
   - 切换到 `7d` / `1m`，桶数对应（24 / 168 / 720）。
   - 选择某 `apiKeyId` 后，summary / distribution / series 同步缩小。
   - 切换 distribution 维度只刷新分布区；切换 series 维度只刷新趋势区。
   - 多币种场景下 cost 系列与 cost 饼图能切币种。
   - 关 docker 后端再启动，确认连续聚合刷新策略仍存在。
8. 在 psql 中跑 `EXPLAIN ANALYZE` 验证 series 查询走的是 `request_overview_hourly` 而非原 `request` 全表扫描。
