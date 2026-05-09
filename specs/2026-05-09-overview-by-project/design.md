# 概览页按项目维度区分 — Design

## Overview

概览页（`OverviewView.vue`）当前支持四个过滤维度（apiKey / model / upstreamModel / provider）以及四个分组维度（同上）。本次改动在数据层面把 `project_id` 加入连续聚合 `request_overview_hourly`，从而让所有概览查询（summary / distribution / series）都能按项目过滤、按项目分组。前端在控件栏加「项目」下拉、在分布与用量的 SegmentedControl 加「项目」选项、并把项目作为最外层加入 Sankey 层级。

`request.project_id` 列已存在（migration 020），由 `pkg/server/project_extractor.go` 在网关入口写入，无需改动写路径。

## Data Model

`request_overview_hourly` 是 TimescaleDB 连续聚合。当前 GROUP BY：`bucket_at, api_key_id, model, upstream_model, provider_id, cost_currency`。

改造方式：DROP 旧 cagg → 重新创建，把 `project_id` 加入 SELECT 与 GROUP BY。新 migration `021_request_overview_hourly_add_project.sql`，沿用 019 的 `-- +goose NO TRANSACTION` 模式与同样的 policy（`start_offset 35 days, end_offset 5 minutes, schedule_interval 5 minutes`）。

旧物化数据被 DROP，重新物化时 lookback 是 35 天，超出窗口的旧数据本来概览页也不展示，无影响。

新 cagg SELECT：

```sql
CREATE MATERIALIZED VIEW request_overview_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  api_key_id,
  model,
  upstream_model,
  provider_id,
  project_id,
  COALESCE(NULLIF(upstream_cost_currency, ''), NULLIF(model_cost_currency, ''), '') AS cost_currency,
  COUNT(*)::bigint AS request_count,
  SUM(COALESCE(input_tokens, 0))::bigint AS input_tokens,
  SUM(COALESCE(cache_read_tokens, 0))::bigint AS cache_read_tokens,
  SUM(COALESCE(output_tokens, 0))::bigint AS output_tokens,
  SUM(COALESCE(cache_write_tokens, 0))::bigint AS cache_write_tokens,
  SUM(COALESCE(cache_write_1h_tokens, 0))::bigint AS cache_write_1h_tokens,
  SUM(COALESCE(upstream_cost, model_cost, 0))::numeric(20, 6) AS cost
FROM request
WHERE type = 1
GROUP BY bucket_at, api_key_id, model, upstream_model, provider_id, project_id, cost_currency
WITH NO DATA;
```

Down：DROP 新 cagg，重建 019 原状（不留 `project_id` 列）。

## Backend query layer

`db/queries/overview.sql` 内 8 个查询统一改造：

1. **过滤参数加 `project_id`**：每个查询的 WHERE 多一行 `AND (sqlc.narg('project_id')::int IS NULL OR project_id = sqlc.narg('project_id')::int)`（`CountTracesFiltered` 与 `ListOverviewSeriesTraces` / `ListOverviewTraceCountsByDimension` 直接打 `request` 表，也加同样的子句）。
2. **dimension CASE 加 `project` 分支**：
   ```sql
   WHEN 'project' THEN COALESCE(project_id::text, '')
   ```
   涉及 `ListOverviewDistribution`, `ListOverviewDistributionCosts`, `ListOverviewTraceCountsByDimension`, `ListOverviewSeriesMetrics`, `ListOverviewSeriesTraces`。
3. **`ListOverviewBreakdownTokens` / `ListOverviewBreakdownCosts`** 增加 `COALESCE(project_id, 0)::int AS project_id` 输出列，并把它加进 GROUP BY。

`pkg/contract/overview.go`：

- `OverviewCommonRequest` 新增 `ProjectID int32 \`query:"projectId,omitempty" minimum:"1"\``。
- `GetOverviewDistributionRequest.Dimension` enum 改为 `apiKey,model,upstreamModel,provider,project`。
- `GetOverviewSeriesRequest.Dimension` enum 改为 `none,apiKey,model,upstreamModel,provider,project`。
- `OverviewBreakdownRowView` 新增 `ProjectID int32 \`json:"projectId"\``。

`pkg/server/handle_overview.go`：

- `hasFilters` 增加 `in.ProjectID != 0` 判断。
- 所有 `db.*Params` 构造增加 `ProjectID: toPgInt4(in.ProjectID)`。

`pkg/server/overview_breakdown.go`：

- `mergeBreakdown` 行键加 `project_id` 维度（与 apiKeyId/model/upstreamModel/providerId 一起作为复合 key），`OverviewBreakdownRowView` 输出 `ProjectID`。

## Frontend data layer

`dashboard/src/api/queryKeys.ts`：

- `OverviewFilters` 加 `projectId?: number`。
- `RequestsFilters` 已经有 `projectId?: number`（migration 020 时加过），无变更。

`dashboard/src/api/client.ts`：

- `getOverviewSummary` / `getOverviewDistribution` / `getOverviewSeries` 三个 fetcher 在拼装查询参数时透传 `projectId`。
- 不需要新的 `invalidate*` 链路。

OpenAPI 重生成：`mise run openapi` → `pnpm --dir dashboard generate-openapi`，TS 类型自动覆盖到三个 dimension enum 与 `OverviewBreakdownRowView.projectId`。

## Frontend UI

`dashboard/src/views/OverviewView.vue`：

1. **filters reactive** 加 `projectId: 0`。`overviewFilters` computed 在 `filters.projectId !== 0` 时透传。
2. **控件栏**：`渠道` 后插入新「项目」`Select`，使用 `useQuery({ queryKey: queryKeys.projects.all, queryFn: listProjects })`，`<option :value="0">全部</option>` + 项目列表。
3. **dimension 选项**：
   - `distributionDimensionOptions` 末尾加 `{ value: 'project', label: '项目' }`
   - `seriesDimensionOptions` 末尾加 `{ value: 'project', label: '项目' }`
4. **`dimensionLabel`** 增加 `dim === 'project'` 分支，查 `projectLabelById`（基于 projectsQuery 数据构建的 `Map<number, string>`）。
5. **`DimKind` 联合**加 `'project'`。`rowDimKey` 加 `case 'project': return \`project:${row.projectId || 0}\``。`dimNodeLabel` 不变（直接调 `dimensionLabel`，未知值已统一处理）。
6. **Sankey 层级**：
   - `tokensInSankey` / `costInSankeyConverted` / `buildCostInSankeyForCurrency` → layers `['provider', 'upstreamModel', 'model', 'apiKey', 'project']`
   - `tokensOutSankey` / `costOutSankeyConverted` / `buildCostOutSankeyForCurrency` → layers `['project', 'apiKey', 'model', 'upstreamModel', 'provider']`
   - 提取常量 `costInLayers` / `costOutLayers` 已存在，直接修改即可。

未关联项目（`project_id IS NULL`）在 cagg 中保留 NULL，CASE 输出空字符串。`dimensionLabel(dim, key)` 当前在 `key === ''` 时直接返回「全部」，对 project 维度而言会把"未关联项目"误标成"全部"。因此 `dimensionLabel` 改造：当 `dim === 'project'` 时优先判断 `key === '' || key === '0'` 返回「未关联」，再走通用流程；其它维度行为保持不变。

## API surface

无新增 operation。三个现有 operation 增加 `projectId` query 参数，distribution/series 的 `dimension` enum 增加 `project`，`OverviewBreakdownRowView` 增加 `projectId` 字段。详见 `api.md`。

## Out of scope

- 旧数据（migration 020 之前）`project_id` 永远是 NULL，不补刷。
- 不在 4 个 bento 卡里加项目维度的总数。
- 不动请求页 / 追踪页（这些已经支持 projectId 过滤）。
- 不在概览的过滤组里加多选（`projectId` 单选即可，与其他过滤一致）。
