# Design

## Overview

新增 `/overview` 页面，作为控制台的默认入口，集中展示一个时间窗口内的请求量、token 消耗、上游费用、追踪数，及其按 API key / 实际模型 / 上游模型 / 渠道 的分布与小时级趋势。

页面只读，所有数据来自三个新增的管理 API：`/api/picotera/overview/summary`、`/overview/distribution`、`/overview/series`。后端依赖一张新的 TimescaleDB 连续聚合 `request_overview_hourly`（基于 `request` 超表）以及现有的 `traces` 表。

## Goals

- 一眼看清整体健康度和资源消耗。
- 维度切换（饼图维度 / 面积图聚合维度）可独立刷新，不重算 summary。
- 在 1m（30 天）范围下查询代价稳定，不随原始 `request` 行数线性增长。
- 视觉延续现有 dashboard 的设计语言，使用 `src/ui/` 原语 + Tailwind v4 工具类，不引入第三方 UI 套件。

## Non-Goals

- 实时（亚分钟）刷新；当前定位是仪表盘，5s `OPERATIONAL_STALE_TIME` 已足够。
- 多币种折算；保留现有 `MoneyDisplay` 单条折算逻辑，后端按币种分组返回。
- 自定义时间范围；仅 `1d` / `7d` / `1m` 三档预设。
- 元请求（`request.type = 0`）参与统计；统一只看上游请求 `type = 1`。

## Data Sources

| 数据 | 来源 | 备注 |
| --- | --- | --- |
| 请求数、token、上游费用 | `request_overview_hourly`（新连续聚合） | 1 小时桶；过滤 `type = 1` |
| 追踪数 | `traces` 表 | 用 `last_request_at` 落入窗口；带 request 过滤时再用 `EXISTS` |
| 维度标签 | `api_key`、`provider`、`model` 现有列表 | 前端拉一次缓存复用 |

### Why a continuous aggregate

`request` 是 hypertable，30 天窗口下的 `SUM(...) GROUP BY hour, model, ...` 直接打原表会随流量线性变慢，且每次切换维度都要重算。把每小时五维（`api_key_id`、`model`、`upstream_model`、`provider_id`、`upstream_cost_currency`）的小计预聚合一份，能：

- 把 30 天查询折成几千～几万行的扫描。
- 维度切换无需重算 SUM；后端只在小聚合上再 `GROUP BY`。
- 通过 `materialized_only = false` 让最近 5 分钟的实时数据继续走原表。

## TimescaleDB Continuous Aggregate

迁移文件：`db/migrations/019_request_overview_hourly_cagg.sql`

```sql
-- +goose NO TRANSACTION
-- +goose Up
CREATE MATERIALIZED VIEW request_overview_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  api_key_id,
  model,
  upstream_model,
  provider_id,
  upstream_cost_currency,
  COUNT(*)::bigint AS request_count,
  SUM(COALESCE(input_tokens, 0))::bigint        AS input_tokens,
  SUM(COALESCE(cache_read_tokens, 0))::bigint   AS cache_read_tokens,
  SUM(COALESCE(output_tokens, 0))::bigint       AS output_tokens,
  SUM(COALESCE(cache_write_tokens, 0))::bigint  AS cache_write_tokens,
  SUM(COALESCE(cache_write_1h_tokens, 0))::bigint AS cache_write_1h_tokens,
  SUM(COALESCE(upstream_cost, 0))::numeric(20, 6) AS upstream_cost
FROM request
WHERE type = 1
GROUP BY bucket_at, api_key_id, model, upstream_model, provider_id, upstream_cost_currency
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
```

注意：

- `CREATE MATERIALIZED VIEW ... WITH (timescaledb.continuous)` 不能在事务中执行，必须 `-- +goose NO TRANSACTION`。
- `start_offset = 35 days` 覆盖 30 天窗口的最大重算需求。
- 存的是 `SUM(...)` 而非 token 总数；总 token 在查询时再相加，避免重复存储。

## Time Ranges

| `range` | 含义 | 起点 (`startAt`) | 终点 (`endAt`) |
| --- | --- | --- | --- |
| `1d` | 最近 24 小时（滚动） | `now() - interval '24 hours'`，向下取整到小时 | `now()`，向上取整到小时 |
| `7d` | 最近 7 天 | `now() - interval '7 days'`，向下取整到小时 | 同上 |
| `1m` | 最近 30 天 | `now() - interval '30 days'`，向下取整到小时 | 同上 |

桶大小固定为 1 小时。后端返回的所有时间戳都是 UTC RFC3339Nano；前端按浏览器本地时区格式化。

## Filters

四个独立过滤器：`apiKeyId`、`model`、`upstreamModel`、`providerId`。全部精确匹配，**fail-fast**：

- 缺省 = 不过滤。
- 整数字段（`apiKeyId`、`providerId`）必须为正整数。
- 字符串字段（`model`、`upstreamModel`）若提供则必须非空；后端不做 trim、case-fold、空串容忍。

过滤器同时作用于 summary、distribution、series 三个端点（追踪数除外的特殊处理见下文）。

## Aggregation Semantics

- **请求数 / token / 费用**：从 `request_overview_hourly` 走 `SUM(...)`，`bucket_at` 半开区间 `[startAt, endAt)`。
- **总 token** = `SUM(input + cache_read + output + cache_write + cache_write_1h)`。
- **费用**按 `upstream_cost_currency` 分组返回 `[{ currency, amount }]`。空币种的行被丢弃。
- **追踪数**：
  - 无 request 过滤时：`COUNT(*) FROM traces WHERE last_request_at >= startAt AND last_request_at < endAt`。
  - 有 request 过滤时：再加 `EXISTS (SELECT 1 FROM request r WHERE r.parent_span_id = traces.parent_span_id AND r.created_at BETWEEN traces.first_request_at AND traces.last_request_at AND r.type = 1 AND <filters>)`。
  - 这里 `EXISTS` 必须打原 `request` 表（连续聚合丢失了 trace 关联），但已被 `request_parent_span_created_at_idx` 索引覆盖。

## Distribution

distribution 端点支持四个维度：`apiKey` / `model` / `upstreamModel` / `provider`。

每行包含 `key`、`label`、`totalTokens`、`requestCount`、`traceCount`、`costs[]`。一次查询返回完整列表，前端在同一份数据上同时驱动 token 饼图和 cost 饼图。

null 维度值（如未关联 `api_key_id`）：`key = ""`、`label = "未设置"`（标签由前端兜底）。

排序：按 `totalTokens DESC`，前端可截断为 Top N（默认 8，其余合并为「其他」组）。

## Series

series 端点支持五个聚合维度：`none` / `apiKey` / `model` / `upstreamModel` / `provider`。

每次返回四种 metric 的小时级行：`tokens` / `cost` / `requests` / `traces`。每行包含 `bucketAt`、`groupKey`、`groupLabel`、`value`、`currency`（仅 `cost` 有意义）。

- `none`：每个 metric 在每个桶上一行，`groupKey = ""`。
- 其它维度：每个 (桶 × 分组) 一行；前端做 stacked area。
- **桶补零**：后端用 `generate_series(startAt, endAt - INTERVAL '1 hour', INTERVAL '1 hour')` 生成完整桶轴，左 join 数据，缺值返回 0。这样 X 轴稳定，Unovis stacked area 不会因为缺桶错位。
- **trace 系列在分组时**：每个匹配过滤器的请求行被归到对应分组，去重后用 `COUNT(DISTINCT parent_span_id)`。同一 trace 横跨多分组时，每个分组各计一次（与 distribution 一致）。

`cost` 当前一次返回所有币种各一条系列；前端默认显示出现次数最多的币种，并提供币种切换。

## API Shape

三个端点：

- `GET /api/picotera/overview/summary`
- `GET /api/picotera/overview/distribution`
- `GET /api/picotera/overview/series`

公共查询参数：`range` + 四个 filters。`distribution` 多一个 `dimension`，`series` 多一个 `dimension`（含 `none`）。详细见 `api.md`。

## Dashboard Architecture

- 路由：新增 `{ path: '/overview', name: 'overview', component: OverviewView }`，根 `/` 重定向到 `/overview`。
- `App.vue`：在 `pageMeta` 加 `overview: { title: '概览', hint: '过去窗口内的请求、token、费用与追踪一览' }`。
- 侧边栏：在 `AppSidebar` 顶部新增「概览」入口，使用 `chart-pie` 图标（在 `src/ui/icons/paths.ts` 注册）。

### View 组成

`dashboard/src/views/OverviewView.vue` 自上而下：

1. **控制条**：左侧 `SegmentedControl`（1d / 7d / 1m，默认 1d），右侧四个过滤器（API Key、实际模型、上游模型、渠道）。过滤器是「未选 + 候选列表」的 `Select`，候选项来自现有 `listApiKeys` / `listModels` / `listProviders` 缓存（实际模型与上游模型从 `models` 列表里分别取 `name` / `upstreamModels` 扁平化去重）。
2. **Bento 区**：4 列 `DataCard`，分别展示 总 token、总请求、总费用、总追踪数。`StateText` 处理 loading / error / 空。
3. **分布区**：2 列 `DataCard`。共享一个维度切换 `SegmentedControl`（API Key / 实际模型 / 上游模型 / 渠道，默认渠道）。左卡内用 Unovis donut 画 `totalTokens` 分布，右卡画 `costs[currency=最大币种]` 分布。
4. **趋势区**：2 列 4 卡的网格（在窄屏堆叠为 1 列），分别绘制 `tokens` / `cost` / `requests` / `traces` 的 stacked area。共享一个聚合维度 `SegmentedControl`（不聚合 / API Key / 实际模型 / 上游模型 / 渠道，默认不聚合）。
5. **空状态 / 错误状态**：每张卡内单独用 `StateText` 处理；不阻塞其它卡。

### Vue Query

- `OPERATIONAL_STALE_TIME`，因为是实时数据。
- 三个独立 query：
  - `queryKeys.overview.summary(filters)` → `getOverviewSummary(filters)`
  - `queryKeys.overview.distribution(filters, dimension)` → `getOverviewDistribution(...)`
  - `queryKeys.overview.series(filters, dimension)` → `getOverviewSeries(...)`
- 拆 query 后，切换饼图维度只重算 distribution；切换面积图维度只重算 series；改 range 或过滤会同时刷新三者。
- 错误经 `ApiRequestError` 抛出，消息直接渲染到对应卡的 `StateText`。

### Filter Sources

- API Key 列表来自 `listApiKeys()`。
- 渠道（provider）列表来自 `listProviders()`。
- 实际模型 = 当前所有 `model.name`。
- 上游模型 = 所有 `model.upstreamModels[*].upstreamModel` 扁平化去重，按字母序。

控制条上方加一行「无数据则禁用过滤项」的 hint 不做；改用 disabled + tooltip 的代价高于价值。

## Charts: Unovis

依赖：`@unovis/vue@^1.6.5` + `@unovis/ts@^1.6.5`，通过 `pnpm --dir dashboard add` 加入 `dashboard/package.json`。

约定：

- 在 `src/main.ts` 引入 `@unovis/ts/styles/index.css`。
- 自建两个 wrapper：
  - `src/components/charts/OverviewDonut.vue` — 包 `VisDonut` + `VisSingleContainer` + 图例。
  - `src/components/charts/OverviewAreaStack.vue` — 包 `VisXYContainer` + `VisStackedArea` + `VisAxis` + `VisTooltip` + 图例。
- 颜色取自 dashboard semantic tokens：分组配色循环 `accent`、`ok`、`warn`、`err`，及对应的 `*-faint`；超过 4 组开始用 OKLCH 环（base = `--color-accent`，按 hue 偏移），仍以 `var(--color-*)` 暴露。
- 字体、字号、tooltip 背景全部走 token；不在 chart 内嵌硬编码颜色。
- 不使用 Unovis 默认主题；图例自己用 `Tag` + `text-2xs` 渲染，避免和 dashboard 的视觉语言脱节。

## OpenAPI Workflow

后端契约改动后：

1. `mise run openapi`（写 `openapi.yaml`）。
2. `pnpm --dir dashboard generate-openapi`（写 `dashboard/src/openapi-types.d.ts`）。
3. 在 `dashboard/src/api/index.ts` 重新导出新的视图类型供 OverviewView 与 chart wrapper 引用。

## Risks & Mitigations

- **连续聚合首次构建慢**：迁移用 `WITH NO DATA`，启动时 5 分钟刷新策略会异步把存量回填；前端在数据未就绪时只显示「最近 5 分钟以内的数据」（即原表实时部分）。文档会提示运维若需要立即看到历史，可手动 `CALL refresh_continuous_aggregate('request_overview_hourly', NULL, NULL)`。
- **30 天 trace `EXISTS` 联表**：在 `request` 上已有 `(parent_span_id, created_at DESC)` 部分索引，覆盖即可；如压测显示瓶颈，再考虑追踪侧也加冗余索引或聚合。
- **货币组合多**：当一个时间窗口包含多种 `upstream_cost_currency`，cost 系列会有多个币种系列。前端默认只画主导币种，并提供切换。
- **codex 旧实现遗留**：旧实现已被 revert，迁移序号 019 保持空闲，本规范从干净状态推进。
