# API

## `ProviderView` 新增字段

```ts
type ProviderView = {
  // ...既有字段
  modelsEndpointUrl?: string
  modelsEndpointResolver?:
    | 'unknown'
    | 'generalApiKey'
    | 'bearerToken'
    | 'xApiKey'
    | 'searchKey'
    | 'googApiKey'
}
```

`CreateProviderRequestBody` / `UpsertProviderRequestBody` 同步增加这两个字段。

## Fetch models 操作

```http
POST /api/picotera/providers/fetch-models
Content-Type: application/json

{ "providerId": 12 }
```

响应保持现状：

```ts
type FetchModelsResponseBody = {
  providerId: number
  providerModels: ProviderModelEntry[]
  removedModels: string[]
}
```

错误：

- `400` — `provider has no models endpoint configured`（provider.modelsEndpointUrl 为空字符串时）。
- `404` — provider 不存在。
- `502` — 上游请求失败 / 响应解码失败 / 响应非 JSON。
- `422` — 响应 JSON 无法解析出模型名。

操作 ID 保持 `fetchModels`，旧路径 `/provider-endpoints/fetch-models` 删除（无兼容层）。

## Endpoint 类型枚举

`EndpointView.endpointType` 的取值集合去掉 `generalListModels`：

```ts
type EndpointType =
  | 'general'
  | 'openaiChatCompletions'
  | 'openaiResponses'
  | 'anthropicMessages'
  | 'anthropicCountTokens'
  | 'geminiGenerateContent'
  | 'geminiStreamGenerateContent'
  | 'unknown'
```

## `rewriteProviderModels` JS 钩子输入

```ts
type RewriteProviderModelsInput = {
  provider: ProviderSummary
  model: ModelSummary | null
  upstreamResponse: unknown // 原 JSON 解码结果
  annotations: Record<string, string>
  // endpointPath 字段被移除
}
```
