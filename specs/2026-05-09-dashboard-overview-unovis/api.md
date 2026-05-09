# API

All paths are under `/api/picotera`.

## `GET /overview`

Operation ID: `getOverview`

Returns summary metrics, distribution data, and hourly chart series for the dashboard overview page.

Request, token, and cost values are served from the TimescaleDB continuous aggregate `request_overview_hourly`. Trace values are served from `traces` with indexed time range predicates and exact request membership checks when request filters are present.

### Query Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `range` | string enum | yes | One of `24h`, `1d`, `7d`, `1m`. |
| `apiKeyId` | integer | no | Exact API key id. |
| `model` | string | no | Exact actual model from `request.model`. |
| `upstreamModel` | string | no | Exact upstream model from `request.upstream_model`. |
| `providerId` | integer | no | Exact provider id. |
| `distributionDimension` | string enum | no | One of `apiKey`, `model`, `upstreamModel`, `provider`. Defaults to `provider`. |
| `seriesDimension` | string enum | no | One of `none`, `apiKey`, `model`, `upstreamModel`, `provider`. Defaults to `none`. |

Validation:

- `range` accepts only `24h`, `1d`, `7d`, or `1m`.
- `distributionDimension` accepts only `apiKey`, `model`, `upstreamModel`, or `provider`.
- `seriesDimension` accepts only `none`, `apiKey`, `model`, `upstreamModel`, or `provider`.
- `apiKeyId` and `providerId` must be positive integers when present.
- `model` and `upstreamModel` must be non-empty when present.
- The backend does not trim, case-fold, coerce empty strings to defaults, or accept near-miss values.

### Response

```jsonc
{
  "range": "7d",
  "startAt": "2026-05-02T00:00:00Z",
  "endAt": "2026-05-09T00:00:00Z",
  "bucket": "hour",
  "summary": {
    "totalTokens": 91827364,
    "totalRequests": 42150,
    "totalTraceCount": 19220,
    "costs": [
      { "currency": "USD", "amount": 128.45 },
      { "currency": "CNY", "amount": 31.2 }
    ]
  },
  "dimensions": {
    "distribution": "provider",
    "series": "model"
  },
  "distributions": [
    {
      "dimension": "provider",
      "key": "1",
      "label": "OpenAI",
      "totalTokens": 50678000,
      "requestCount": 20100,
      "traceCount": 8200,
      "costs": [
        { "currency": "USD", "amount": 71.22 }
      ]
    }
  ],
  "series": [
    {
      "metric": "tokens",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "groupLabel": "gpt-4.1",
      "value": 235000,
      "currency": ""
    },
    {
      "metric": "cost",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "groupLabel": "gpt-4.1",
      "value": 1.42,
      "currency": "USD"
    },
    {
      "metric": "requests",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "groupLabel": "gpt-4.1",
      "value": 91,
      "currency": ""
    },
    {
      "metric": "traces",
      "bucketAt": "2026-05-09T07:00:00Z",
      "groupKey": "gpt-4.1",
      "groupLabel": "gpt-4.1",
      "value": 52,
      "currency": ""
    }
  ]
}
```

### Types

#### `OverviewRange`

```ts
type OverviewRange = '24h' | '1d' | '7d' | '1m'
```

#### `OverviewDimension`

```ts
type OverviewDimension = 'apiKey' | 'model' | 'upstreamModel' | 'provider'
```

#### `OverviewSeriesDimension`

```ts
type OverviewSeriesDimension = 'none' | OverviewDimension
```

#### `OverviewCost`

```ts
interface OverviewCost {
  currency: string
  amount: number
}
```

#### `OverviewSummary`

```ts
interface OverviewSummary {
  totalTokens: number
  totalRequests: number
  totalTraceCount: number
  costs: OverviewCost[]
}
```

#### `OverviewDistributionRow`

```ts
interface OverviewDistributionRow {
  dimension: OverviewDimension
  key: string
  label: string
  totalTokens: number
  requestCount: number
  traceCount: number
  costs: OverviewCost[]
}
```

#### `OverviewSeriesRow`

```ts
interface OverviewSeriesRow {
  metric: 'tokens' | 'cost' | 'requests' | 'traces'
  bucketAt: string
  groupKey: string
  groupLabel: string
  value: number
  currency: string
}
```

#### `OverviewResponseBody`

```ts
interface OverviewResponseBody {
  range: OverviewRange
  startAt: string
  endAt: string
  bucket: 'hour'
  summary: OverviewSummary
  dimensions: {
    distribution: OverviewDimension
    series: OverviewSeriesDimension
  }
  distributions: OverviewDistributionRow[]
  series: OverviewSeriesRow[]
}
```

### Status Codes

- `200`: Overview data returned.
- `400`: Invalid range, dimension, id, or empty string filter.
- `500`: Database aggregation failed.
