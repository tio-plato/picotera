# API: Cache Hit Rate Chart

## Endpoint

不新增 endpoint。缓存命中率数据复用：

```http
GET /api/picotera/overview/series
```

## Query 参数

沿用现有参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `range` | `1d | 7d | 1m` | 是 | 时间范围 |
| `dimension` | `none | apiKey | model | upstreamModel | provider | project` | 是 | 分组维度 |
| `apiKeyId` | integer | 否 | 过滤密钥 |
| `model` | string | 否 | 过滤请求模型 |
| `upstreamModel` | string | 否 | 过滤上游模型 |
| `providerId` | integer | 否 | 过滤渠道 |
| `projectId` | integer | 否 | 过滤项目 |

## 新增 metric

`OverviewSeriesPointView.metric` 新增：

| metric | 含义 | 单位 |
| --- | --- | --- |
| `cacheHitRate` | 缓存命中率 | `0` 到 `1` 的比例 |

现有 metric 保持不变：

| metric | 含义 |
| --- | --- |
| `tokens` | Token 数量 |
| `requests` | 请求数 |
| `traces` | Trace 数 |
| `cost` | 费用 |
| `prefillSpeed` | Prefill 速度 |
| `decodeSpeed` | Decode 速度 |

## 计算规则

单个响应点的计算规则：

```text
value = SUM(cache_read_tokens) / SUM(input_tokens + cache_read_tokens + cache_write_tokens + cache_write_1h_tokens)
```

分母为 0 的桶不返回 `cacheHitRate` 点。前端把缺失点视为无数据，而不是 0%。

## 响应示例

```json
{
  "window": {
    "range": "1d",
    "startAt": "2026-05-25T16:00:00Z",
    "endAt": "2026-05-26T16:00:00Z",
    "bucket": "hour"
  },
  "dimension": "provider",
  "groups": [
    { "key": "1", "label": "1" },
    { "key": "2", "label": "2" }
  ],
  "buckets": [
    "2026-05-25T16:00:00Z",
    "2026-05-25T17:00:00Z"
  ],
  "points": [
    {
      "metric": "cacheHitRate",
      "bucketAt": "2026-05-25T16:00:00Z",
      "groupKey": "1",
      "value": 0.624,
      "currency": ""
    }
  ]
}
```

## OpenAPI

Contract 结构不变。实现完成后仍运行 `mise run openapi` 和 `pnpm --dir dashboard generate-openapi`，保持 checked-in spec 与 dashboard 类型同步。
