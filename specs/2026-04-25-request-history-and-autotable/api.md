# API

Base path：`/api/picotera`。

## Types

### `RequestView`

```ts
interface RequestView {
  id: string
  spanId?: string
  parentSpanId?: string
  providerId: number
  endpointPath: string
  apiKeyId?: number
  model?: string
  inputTokens?: number
  cacheReadTokens?: number
  outputTokens?: number
  cacheWriteTokens?: number
  statusCode: number
  errorMessage?: string
  ttftMs?: number
  timeSpentMs: number
  createdAt: string  // RFC3339
}
```

Nullable 字段在数据库为空时省略（`omitempty`）。

## Operations

### `listRequests` — `GET /requests`

Query params：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `limit` | int (1–100, 默认 20) | 单页大小 |
| `cursor` | string | 上一页返回的 `nextCursor` |
| `providerId` | int, 可选 | 按渠道过滤 |
| `endpointPath` | string, 可选 | 按端点过滤 |
| `model` | string, 可选 | 按模型过滤 |

排序：`created_at DESC, id DESC`。

响应体：

```ts
{
  items: RequestView[]
  pagination: {
    hasMore: boolean
    nextCursor?: string
  }
}
```

`nextCursor` 编码 `{ createdAt: string, id: string }`（base64 JSON，与现有 `contract.EncodeCursor` 一致）。

### `getRequest` — `GET /requests/{id}`

Path param：`id: string`。

200 响应体：`RequestView`。

404：`request not found`（`errorx.RequestNotFound`）。

## Errors

| 情况 | HTTP | Error code |
| --- | --- | --- |
| 请求不存在 | 404 | `RequestNotFound` |
| 无效 cursor | 400 | （沿用 Huma 默认） |
| 内部错误 | 500 | `InternalError` |
