# 概览页按项目维度区分 — Execution plan

按顺序执行，每步结束后代码可编译。

## 1. Migration `021_request_overview_hourly_add_project.sql`

`db/migrations/021_request_overview_hourly_add_project.sql`：

```sql
-- +goose NO TRANSACTION
-- +goose Up
SELECT remove_continuous_aggregate_policy('request_overview_hourly', if_exists => true);
DROP MATERIALIZED VIEW IF EXISTS request_overview_hourly;

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

ALTER MATERIALIZED VIEW request_overview_hourly
  SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy(
  'request_overview_hourly',
  start_offset      => INTERVAL '35 days',
  end_offset        => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);

-- +goose Down
SELECT remove_continuous_aggregate_policy('request_overview_hourly', if_exists => true);
DROP MATERIALIZED VIEW IF EXISTS request_overview_hourly;

CREATE MATERIALIZED VIEW request_overview_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  api_key_id,
  model,
  upstream_model,
  provider_id,
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
GROUP BY bucket_at, api_key_id, model, upstream_model, provider_id, cost_currency
WITH NO DATA;

ALTER MATERIALIZED VIEW request_overview_hourly
  SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy(
  'request_overview_hourly',
  start_offset      => INTERVAL '35 days',
  end_offset        => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);
```

## 2. sqlc 查询改造

`db/queries/overview.sql`，对 8 个查询做以下变更：

**A. 加 `project_id` 过滤参数**（每处 WHERE 末尾追加）：

```sql
AND (sqlc.narg('project_id')::int IS NULL OR project_id = sqlc.narg('project_id')::int)
```

适用查询：
- `GetOverviewTotals` — 在 `filtered` CTE 的 WHERE。
- `CountTracesFiltered` — 在内层 `EXISTS` 子查询，谓词 `r.project_id`。
- `ListOverviewDistribution`
- `ListOverviewDistributionCosts`
- `ListOverviewTraceCountsByDimension` — 谓词 `r.project_id`。
- `ListOverviewSeriesMetrics`
- `ListOverviewSeriesTraces` — 谓词 `r.project_id`。
- `GetOverviewTokenBreakdown`
- `ListOverviewBreakdownTokens`
- `ListOverviewBreakdownCosts`

**B. dimension CASE 加 `project` 分支**：

```sql
WHEN 'project' THEN COALESCE(project_id::text, '')
```

适用查询：
- `ListOverviewDistribution`
- `ListOverviewDistributionCosts`
- `ListOverviewTraceCountsByDimension`（用 `r.project_id`）
- `ListOverviewSeriesMetrics`
- `ListOverviewSeriesTraces`（用 `r.project_id`）

**C. breakdown 输出增加 `project_id` 列与 GROUP BY**：

`ListOverviewBreakdownTokens` / `ListOverviewBreakdownCosts`：

```sql
SELECT
  COALESCE(api_key_id, 0)::int       AS api_key_id,
  COALESCE(model, '')::text          AS model,
  COALESCE(upstream_model, '')::text AS upstream_model,
  COALESCE(provider_id, 0)::int      AS provider_id,
  COALESCE(project_id, 0)::int       AS project_id,
  ...
GROUP BY 1, 2, 3, 4, 5
HAVING ...   -- ListOverviewBreakdownTokens 保留 HAVING；ListOverviewBreakdownCosts 加上 GROUP BY 列号 6（currency）
```

注意 `ListOverviewBreakdownCosts` 原来 GROUP BY 是 `1, 2, 3, 4, 5`（最后一个是 currency）。加 project_id 后变成 `1, 2, 3, 4, 5, 6`，currency 是第 6 列。

跑 `sqlc generate`，校对 `pkg/db/overview.sql.go`。

## 3. Contract 类型

`pkg/contract/overview.go`：

- `OverviewCommonRequest` 末尾追加：
  ```go
  ProjectID int32 `query:"projectId,omitempty" minimum:"1"`
  ```
- `GetOverviewDistributionRequest.Dimension` enum：
  ```go
  Dimension string `query:"dimension" enum:"apiKey,model,upstreamModel,provider,project" required:"true"`
  ```
- `GetOverviewSeriesRequest.Dimension` enum：
  ```go
  Dimension string `query:"dimension" enum:"none,apiKey,model,upstreamModel,provider,project" required:"true"`
  ```
- `OverviewBreakdownRowView` 末尾追加：
  ```go
  ProjectID int32 `json:"projectId"`
  ```

## 4. Handler 改造

`pkg/server/handle_overview.go`：

- `hasFilters` 末尾加 `|| in.ProjectID != 0`。
- 每处构造 `db.*Params` 的地方追加 `ProjectID: toPgInt4(in.ProjectID)`（共 8 处：`GetOverviewTotals`、`GetOverviewTokenBreakdown`、`ListOverviewBreakdownTokens`、`ListOverviewBreakdownCosts`、`CountTracesFiltered`、`ListOverviewDistribution`、`ListOverviewDistributionCosts`、`ListOverviewTraceCountsByDimension`、`ListOverviewSeriesMetrics`、`ListOverviewSeriesTraces`）。
- 注意 `s.queries.CountTraces` 这一支不带过滤，无需改动。

`pkg/server/overview_breakdown.go`：

- 复合 row key 加上 `project_id` 维度（与 apiKeyId/model/upstreamModel/providerId 一起作为复合 key）。
- 输出 `OverviewBreakdownRowView.ProjectID`。

## 5. OpenAPI 重生成

```
mise run openapi
pnpm --dir dashboard generate-openapi
```

校对 diff 至少包含：
- 三个 operation 的 `projectId` query。
- `getOverviewDistribution` / `getOverviewSeries` dimension enum 增加 `project`。
- `OverviewBreakdownRowView.projectId`。

## 6. Dashboard data layer

`dashboard/src/api/queryKeys.ts`：

`OverviewFilters` 类型加 `projectId?: number`。

`dashboard/src/api/client.ts`：

`getOverviewSummary` / `getOverviewDistribution` / `getOverviewSeries` 在拼装 query 参数时把 `filters.projectId` 透传（仅当 `> 0`）。模板照搬现有 `apiKeyId` / `providerId` 的写法。

## 7. OverviewView 改造

`dashboard/src/views/OverviewView.vue`：

**state**

```ts
const filters = reactive({
  range: '1d' as OverviewRange,
  apiKeyId: 0,
  model: '',
  upstreamModel: '',
  providerId: 0,
  projectId: 0,
})
```

`overviewFilters` computed 中追加：

```ts
if (filters.projectId) out.projectId = filters.projectId
```

**项目数据**

```ts
import { listProjects } from '@/api/client'
const projectsQuery = useQuery({ queryKey: queryKeys.projects.all, queryFn: listProjects })
const projects = computed(() => projectsQuery.data.value ?? [])
const projectLabelById = computed(() => {
  const m = new Map<number, string>()
  for (const p of projects.value) m.set(p.id, p.name)
  return m
})
```

**控件栏**：在「渠道」之后插入：

```html
<div class="flex flex-col gap-1">
  <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">项目</span>
  <Select v-model.number="filters.projectId" size="sm">
    <option :value="0">全部</option>
    <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
  </Select>
</div>
```

**dimension 选项**：

```ts
const distributionDimensionOptions: { value: OverviewDimension; label: string }[] = [
  { value: 'provider', label: '渠道' },
  { value: 'apiKey', label: '密钥' },
  { value: 'model', label: '请求模型' },
  { value: 'upstreamModel', label: '上游模型' },
  { value: 'project', label: '项目' },
]
const seriesDimensionOptions: { value: OverviewSeriesDimension; label: string }[] = [
  { value: 'none', label: '全部' },
  { value: 'provider', label: '渠道' },
  { value: 'apiKey', label: '密钥' },
  { value: 'model', label: '请求模型' },
  { value: 'upstreamModel', label: '上游模型' },
  { value: 'project', label: '项目' },
]
```

**`dimensionLabel`** 重写：

```ts
function dimensionLabel(dim: OverviewDimension | OverviewSeriesDimension, key: string): string {
  if (dim === 'project') {
    if (key === '' || key === '0') return '未关联'
    const id = Number(key)
    return Number.isFinite(id) ? projectLabelById.value.get(id) ?? `#${key}` : key
  }
  if (key === '') return '全部'
  if (dim === 'provider') {
    const id = Number(key)
    return Number.isFinite(id) ? providerLabelById.value.get(id) ?? `#${key}` : key
  }
  if (dim === 'apiKey') {
    const id = Number(key)
    return Number.isFinite(id) ? apiKeyLabelById.value.get(id) ?? `#${key}` : key
  }
  return key
}
```

注意：把 `project` 分支放在 `key === ''` 早返回之前，让未关联项目走「未关联」标签而不是「全部」。

**Sankey 层级与 DimKind**：

```ts
type DimKind = 'apiKey' | 'model' | 'upstreamModel' | 'provider' | 'project'

function rowDimKey(row: OverviewBreakdownRowView, dim: DimKind): string {
  switch (dim) {
    case 'apiKey':        return `apiKey:${row.apiKeyId || 0}`
    case 'model':         return `model:${row.model || ''}`
    case 'upstreamModel': return `upstreamModel:${row.upstreamModel || ''}`
    case 'provider':      return `provider:${row.providerId || 0}`
    case 'project':       return `project:${row.projectId || 0}`
  }
}

function rawValueFromKey(key: string): { dim: DimKind; raw: string } | null {
  const idx = key.indexOf(':')
  if (idx < 0) return null
  const dim = key.slice(0, idx) as DimKind
  if (
    dim !== 'apiKey' && dim !== 'model' && dim !== 'upstreamModel' &&
    dim !== 'provider' && dim !== 'project'
  ) return null
  return { dim, raw: key.slice(idx + 1) }
}

const tokensInLayers: DimKind[] = ['provider', 'upstreamModel', 'model', 'apiKey', 'project']
const tokensOutLayers: DimKind[] = ['project', 'apiKey', 'model', 'upstreamModel', 'provider']
const costInLayers: DimKind[]   = ['provider', 'upstreamModel', 'model', 'apiKey', 'project']
const costOutLayers: DimKind[]  = ['project', 'apiKey', 'model', 'upstreamModel', 'provider']
```

把 `tokensInSankey` / `tokensOutSankey` 也切到 `tokensInLayers` / `tokensOutLayers`（之前是字面量数组，统一抽常量）。

`dimNodeLabel` 不变 — 它依赖 `dimensionLabel`，已经统一处理 project。`raw === '0'` 走「未知」分支会被 `dimensionLabel('project', '0')` 抢先返回「未关联」，符合预期。

## 8. 验证

- `sqlc generate` 无 diff 缺失。
- `go build ./...` 通过。
- `mise run openapi` 后 `pnpm --dir dashboard generate-openapi`，`dashboard/src/openapi-types.d.ts` 出现 `projectId` 字段与新 enum 值。
- `pnpm --dir dashboard build` 通过（vue-tsc + vite）。
- 手测：
  1. `docker compose up -d`，启动 backend，应用 migration 021。
  2. 创建一个项目（带匹配路径），发若干请求让命中 / 未命中两类都有。
  3. 概览页选「项目 = X」，确认所有图表过滤正确。
  4. 分布统计切到「项目」维度，确认 Token 分布 / 费用分布 按项目切分；包含一个「未关联」分片。
  5. 用量统计切到「项目」维度，确认堆叠区域图按项目切分。
  6. Sankey 切到 `tokensIn`，确认末层是项目；切到 `tokensOut`，确认首层是项目；`costIn` / `costOut` 同样。
