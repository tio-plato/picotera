# Design: Cache Hit Rate Chart

## 目标

在 Overview 页面增加缓存命中率折线图，布局和交互与现有速度统计区域一致。图表支持按渠道、密钥、请求模型、上游模型、项目以及全局维度统计。缺少有效数据的时间桶不按 0 展示，折线跨越缺口时使用虚线连接。

## 命中率定义

缓存命中率按输入侧 token 计算：

```text
cacheHitRate = cache_read_tokens / (input_tokens + cache_read_tokens + cache_write_tokens + cache_write_1h_tokens)
```

分母为输入侧总 token，包含未缓存输入、缓存读取、短期缓存写入和 1h 缓存写入。输出 token 不参与缓存命中率计算。

聚合时先分别汇总分子和分母，再相除：

```text
SUM(cache_read_tokens) / SUM(input_tokens + cache_read_tokens + cache_write_tokens + cache_write_1h_tokens)
```

这样跨请求、跨小时桶、跨维度筛选时得到按 token 数加权的命中率。

## 数据层

复用现有 `request_overview_hourly` continuous aggregate。该视图已经按 1 小时、渠道、密钥、请求模型、上游模型、项目聚合了：

- `input_tokens`
- `cache_read_tokens`
- `cache_write_tokens`
- `cache_write_1h_tokens`

无需新增 migration 或新的 continuous aggregate。

在 `db/queries/overview.sql` 新增 `ListOverviewCacheHitRateSeries` 查询：

- 查询 `request_overview_hourly`
- 支持现有 Overview 过滤器：`range`, `apiKeyId`, `model`, `upstreamModel`, `providerId`, `projectId`
- 支持 `dimension = none | provider | apiKey | model | upstreamModel | project`
- 按 `bucket_at` 与动态 `group_key` 聚合
- 返回 `cache_read_tokens` 汇总值和输入侧 token 汇总值
- 使用 `HAVING SUM(input_tokens + cache_read_tokens + cache_write_tokens + cache_write_1h_tokens) > 0` 排除无有效分母的桶

Go handler 负责计算最终比例，避免在 SQL 中把 NULL 或 0 分母误转成 0 命中率。

## API 层

复用现有 `GET /api/picotera/overview/series` 响应结构，在 `points` 中增加一个 metric：

- `metric: "cacheHitRate"`
- `value`: `0` 到 `1` 的小数比例
- `currency`: 空字符串

`OverviewSeriesPointView.metric` 当前是 string 类型，不需要修改 contract 结构。

`handleGetOverviewSeries` 在查询常规 series、速度 series、trace series 后，再查询缓存命中率 series，并把非空结果追加到同一个 `points` 数组。响应中的 `groups` 会包含缓存命中率查询出现过的分组。

## Dashboard 层

在 `OverviewView.vue` 新增缓存统计区域，位置放在速度统计区域之后：

- 新增 `cacheHitRateDimension`，类型为 `OverviewSeriesDimension`，默认值为 `model`
- 新增 `cacheHitRateSeriesQuery`，调用现有 `getOverviewSeries(overviewFilters, cacheHitRateDimension)`
- 新增 query key：`queryKeys.overview.cacheHitRate(...)`
- 从响应 points 中过滤 `metric === "cacheHitRate"` 后传给 `OverviewLineChart`
- value formatter 显示百分比，例如 `62.4%`

图表标题为“缓存命中率”。维度选择器复用现有 series 维度选项，包含“全部、渠道、密钥、请求模型、上游模型、项目”。

## 缺失点虚线连接

现有 `OverviewLineChart` 已用 `undefined` 表示缺失点，并在 `VisLine` 上启用 `interpolateMissingData`。Unovis 会自动用虚线连接跨缺失桶的线段。

缓存命中率只为 SQL 返回的有效桶提供点，不为缺失桶补 0。前端沿用现有 `OverviewLineChart`，由组件当前的缺失点插值行为展示虚线连接。

## 第三方库

不新增第三方依赖。折线图继续使用现有 `@unovis/vue`。
