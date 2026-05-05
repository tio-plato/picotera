# API — Parent Span Traces

## `GET /api/picotera/request-traces`

分页列出所有已知的非空 `parent_span_id` 聚合。

### Query

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `limit` | integer | 否 | 每页数量，默认 `20`，最大 `100`。 |
| `cursor` | string | 否 | 上一页响应中的 cursor。 |

### Response

```json
{
  "items": [
    {
      "parentSpanId": "sid-abc",
      "requestCount": 8,
      "totalTokens": 124500,
      "modelCosts": [
        { "currency": "USD", "amount": 0.91 },
        { "currency": "CNY", "amount": 2.40 }
      ],
      "upstreamCosts": [
        { "currency": "USD", "amount": 0.73 },
        { "currency": "JPY", "amount": 52.23 }
      ],
      "lastRequestAt": "2026-05-05T08:12:34.123Z"
    }
  ],
  "pagination": {
    "hasMore": true,
    "nextCursor": "eyJsYXN0UmVxdWVzdEF0Ijoi...\""
  }
}
```

### Cursor

Cursor 编码字段：

- `lastRequestAt`
- `parentSpanId`

下一页查询使用 `(last_request_at, parent_span_id) < (:last_request_at, :parent_span_id)`，排序为 `last_request_at DESC, parent_span_id DESC`。

## `GET /api/picotera/requests`

现有请求列表接口新增 query 参数。

### 新增 Query

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `parentSpanId` | string | 否 | 精确筛选 `request.parent_span_id`。 |

### 行为

- `parentSpanId` 为空或缺省时，不启用该筛选。
- `parentSpanId` 存在时，返回 `parent_span_id` 精确等于该值的请求。
- 该筛选可与现有 `type`、`providerId`、`endpointPath`、`model`、`upstreamModel` 筛选组合。

## 成本字段

`GET /api/picotera/request-traces` 同时返回 `modelCosts` 和 `upstreamCosts`。两者都是按币种聚合后的数组。

### `TraceCost`

```json
{
  "currency": "USD",
  "amount": 3.5
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `currency` | string | 原始成本币种。 |
| `amount` | number | 该币种下的成本合计。 |

### 展示约定

dashboard 的“追踪”页面分别显示模型成本和上游成本：

- `modelCosts`：按请求模型定价计算后的成本合计数组。
- `upstreamCosts`：按实际上游模型定价计算后的成本合计数组。

当前端 `useCurrency().targetCurrency` 为 `null` 时，直接展示数组中的每个币种，例如 `$3.50 + ¥52.23`。

当前端 `useCurrency().targetCurrency` 有值时，前端使用 `useCurrency().convert` 把数组中每个金额换算到目标货币，再加总后展示一个目标货币金额。
