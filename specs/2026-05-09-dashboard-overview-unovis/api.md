# API

所有路径都在 `/api/picotera` 下。三个端点共享下文的查询参数与类型。所有时间戳都是 UTC，RFC3339Nano 字符串。

## Common Query Parameters

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `range` | enum | 是 | `1d` / `7d` / `1m`。仅这三个值；其它一律 `400`。 |
| `apiKeyId` | int | 否 | 精确匹配；必须 ≥ 1。 |
| `model` | string | 否 | 精确匹配 `request.model`；提供则非空。 |
| `upstreamModel` | string | 否 | 精确匹配 `request.upstream_model`；提供则非空。 |
| `providerId` | int | 否 | 精确匹配；必须 ≥ 1。 |

校验由 Huma 完成；不做 trim、case-fold、空串容忍。

## Common Types

```ts
type OverviewRange = '1d' | '7d' | '1m'
type OverviewDimension = 'apiKey' | 'model' | 'upstreamModel' | 'provider'
type OverviewSeriesDimension = 'none' | OverviewDimension
type OverviewMetric = 'tokens' | 'cost' | 'requests' | 'traces'

interface OverviewCost {
  currency: string
  amount: number
}

interface OverviewWindow {
  range: OverviewRange
  startAt: string
  endAt: string
  bucket: 'hour'
}
```

## `GET /overview/summary`

- Operation ID: `getOverviewSummary`
- 返回所选窗口与过滤器下的四个总数。

### Request

公共参数。

### Response

```jsonc
{
  "window": {
    "range": "7d",
    "startAt": "2026-05-02T00:00:00Z",
    "endAt": "2026-05-09T08:00:00Z",
    "bucket": "hour"
  },
  "totalTokens": 91827364,
  "totalRequests": 42150,
  "totalTraceCount": 19220,
  "costs": [
    { "currency": "USD", "amount": 128.45 },
    { "currency": "CNY", "amount": 31.20 }
  ]
}
```

```ts
interface OverviewSummaryResponseBody {
  window: OverviewWindow
  totalTokens: number
  totalRequests: number
  totalTraceCount: number
  costs: OverviewCost[]
}
```

## `GET /overview/distribution`

- Operation ID: `getOverviewDistribution`
- 返回某一维度下的分布数据，同时驱动 token 饼图与 cost 饼图。

### Extra Query Parameters

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `dimension` | enum | 是 | `apiKey` / `model` / `upstreamModel` / `provider`。 |

### Response

```jsonc
{
  "window": { /* same as summary */ },
  "dimension": "provider",
  "rows": [
    {
      "key": "1",
      "label": "OpenAI",
      "totalTokens": 50678000,
      "requestCount": 20100,
      "traceCount": 8200,
      "costs": [{ "currency": "USD", "amount": 71.22 }]
    },
    {
      "key": "",
      "label": "",
      "totalTokens": 1200,
      "requestCount": 4,
      "traceCount": 2,
      "costs": []
    }
  ]
}
```

`key = ""` 表示该维度上为 NULL；前端渲染为「未设置」。

```ts
interface OverviewDistributionRow {
  key: string
  label: string
  totalTokens: number
  requestCount: number
  traceCount: number
  costs: OverviewCost[]
}

interface OverviewDistributionResponseBody {
  window: OverviewWindow
  dimension: OverviewDimension
  rows: OverviewDistributionRow[]
}
```

排序：`totalTokens DESC, key ASC`。

## `GET /overview/series`

- Operation ID: `getOverviewSeries`
- 返回四种 metric 的小时级序列。

### Extra Query Parameters

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `dimension` | enum | 是 | `none` / `apiKey` / `model` / `upstreamModel` / `provider`。 |

### Response

```jsonc
{
  "window": { /* same as summary */ },
  "dimension": "model",
  "groups": [
    { "key": "gpt-4.1", "label": "gpt-4.1" },
    { "key": "claude-sonnet-4", "label": "claude-sonnet-4" }
  ],
  "buckets": [
    "2026-05-09T07:00:00Z",
    "2026-05-09T08:00:00Z"
  ],
  "points": [
    {
      "metric": "tokens",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "value": 235000,
      "currency": ""
    },
    {
      "metric": "cost",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "value": 1.42,
      "currency": "USD"
    },
    {
      "metric": "requests",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "value": 91,
      "currency": ""
    },
    {
      "metric": "traces",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "value": 52,
      "currency": ""
    }
  ]
}
```

约定：

- `dimension = none` 时 `groups` 长度为 1，`groupKey = ""`、`label = ""`。
- `buckets` 是后端用 `generate_series` 生成的完整桶轴；前端可直接拿来当 X 轴。
- `points` 中只包含值非零或经过补零后被显式塞入的行；零值桶亦显式存在以便 stacked area 不错位。
- `cost` 在多币种时为每种币种 × 每分组生成独立的 `points`（`currency` 字段区分）；其它 metric `currency = ""`。

```ts
interface OverviewSeriesGroup {
  key: string
  label: string
}

interface OverviewSeriesPoint {
  metric: OverviewMetric
  bucketAt: string
  groupKey: string
  value: number
  currency: string
}

interface OverviewSeriesResponseBody {
  window: OverviewWindow
  dimension: OverviewSeriesDimension
  groups: OverviewSeriesGroup[]
  buckets: string[]
  points: OverviewSeriesPoint[]
}
```

## Status Codes

所有端点共用：

- `200`：返回数据。
- `400`：`range` / `dimension` / `apiKeyId` / `providerId` / `model` / `upstreamModel` 任一无效。
- `500`：数据库查询失败（包括连续聚合不可用）。
