# API: Endpoint type field

## `EndpointView`

```ts
interface EndpointView {
  name: string
  path: string
  modelPath: string  // 允许空串；仅 openai*/anthropic* 类型必填
  credentialsResolver: 'generalApiKey' | 'bearerToken' | 'xApiKey' | 'unknown'
  endpointType:
    | 'general'
    | 'openaiChatCompletions'
    | 'openaiResponses'
    | 'anthropicMessages'
    | 'anthropicCountTokens'
    | 'generalListModels'
    | 'unknown'
}
```

## 受影响的 operation

| operationId           | method | path                  | 变更                                  |
| --------------------- | ------ | --------------------- | ------------------------------------- |
| `listEndpoints`       | GET    | `/endpoints`          | 响应 body 元素新增 `endpointType`，`modelPath` 仍为字符串（允许空串） |
| `upsertEndpoint`      | PUT    | `/endpoints`          | 请求/响应 body 同上                    |
| `deleteEndpoint`      | POST   | `/endpoints/delete`   | 不变                                  |
| `fetchModels`         | POST   | `/provider-endpoints/fetch-models` | 不变（前端按类型过滤来源端点）   |

## 行为约定

- `endpointType` 必填字符串；空字符串/未知值会被规整为 `unknown`。
- `modelPath` 在 `endpointType ∈ {openaiChatCompletions, openaiResponses, anthropicMessages, anthropicCountTokens}` 时必须非空（前端校验 + 业务约定）。其他类型允许空串。
- 网关命中端点时若 `modelPath` 为空，统一返回 400 + `model_not_found`，message `endpoint has no model path configured`。
- 现有 `/endpoints` PUT 调用方未传 `endpointType` 时，huma 校验会返回 422；调用方需升级。
- 旧客户端 GET 时收到的新增字段不影响其行为（JSON 解析忽略未知字段）。
