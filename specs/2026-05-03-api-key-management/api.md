# API — API Key 管理

所有路径前缀仍为 `/api/picotera`，鉴权策略与现有管理 API 一致（不需要 API Key）。资源命名空间用连字符：`api-keys`。

## 资源 View

```ts
type ApiKeyView = {
  id: number
  name: string
  key: string                 // 明文，不脱敏
  disabled: boolean
  annotations: Record<string, string>
  createdAt: string           // RFC3339
  updatedAt: string           // RFC3339
}
```

## Operations

### `GET /api-keys` — `listApiKeys`

- 响应：`ApiKeyView[]`，按 `created_at DESC, id DESC`。

### `GET /api-keys/{id}` — `getApiKey`

- Path：`id` (int)。
- 响应：`ApiKeyView`。
- 404：`api key not found`。

### `POST /api-keys` — `createApiKey`

- Body：
  ```ts
  {
    name: string
    key?: string              // 缺省时由后端生成，格式 sk_pt_<32 hex>
    disabled?: boolean        // 默认 false
    annotations?: Record<string, string>
  }
  ```
- 响应：`ApiKeyView`。
- 409：`key already exists`（`UNIQUE` 冲突）。

### `PUT /api-keys/{id}` — `updateApiKey`

- Path：`id` (int)。
- Body：与 `createApiKey` 相同，但 `name` / `key` / `disabled` / `annotations` 都按全量替换语义。
- 响应：`ApiKeyView`。
- 404：`api key not found`。
- 409：`key already exists`。

### `POST /api-keys/delete` — `deleteApiKey`

- Body：`{ id: number }`。沿用 `OperationDeleteScript` 的「post + body id」风格，避免 DELETE 方法在某些 CDN/代理下被剥离。
- 响应：`204` 等价的空体。

## 网关错误响应

携带于 `gatewayHandler` 的失败路径，结构对齐现有 `writeGatewayError`：

| 场景 | HTTP | code |
| --- | --- | --- |
| 该 endpoint 的 resolver 允许的位置中都没带 token（位置规则参考 `google-credential-resolvers`） | 401 | `UNAUTHORIZED` |
| 带了但查不到匹配 key | 401 | `UNAUTHORIZED` |
| 命中但 `disabled = true` | 403 | `FORBIDDEN` |
| 查询 DB 出错 | 500 | `INTERNAL_ERROR` |

`FORBIDDEN` 错误码若 `pkg/errorx` 中没有，则在常量表里追加一项。

## Hook ctx 增量

`pkg/jsx/sdk.js` 的钩子签名不变；Go 侧给以下四个 ctx 加 `apiKey` 字段：

```ts
type ApiKeySummary = {
  id: number
  name: string
  annotations: Record<string, string>
  disabled: boolean
}
```

涉及的 ctx：`sortProviders`、`rewriteModel`、`beforeRequest`、`rewriteRequest`。`rewriteProviderModels` 不变。
