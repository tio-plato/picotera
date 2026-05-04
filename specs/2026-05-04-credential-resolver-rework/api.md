# API 变更

## `ProviderEndpointView`

`PUT /api/picotera/provider-endpoints` / `GET /api/picotera/provider-endpoints` / `POST /api/picotera/provider-endpoints/delete` 的请求与响应体增加一个字段：

```ts
type ProviderEndpointView = {
  providerId: number
  endpointPath: string
  upstreamUrl: string
  credentialsResolver:
    | 'unknown'         // 继承 endpoint.credentialsResolver（默认）
    | 'generalApiKey'
    | 'bearerToken'
    | 'xApiKey'
    | 'searchKey'
    | 'googApiKey'
}
```

字段语义：

- 用于覆盖网关向上游发送凭证时的位置（header / query 名）。
- 不影响读取客户端凭证；读取永远使用 `endpoint.credentialsResolver`。
- `unknown` 表示不覆盖、按 endpoint 设置发送。
- DB 列 `provider_endpoint.credentials_resolver` 默认 `0`（即 `unknown`）；旧记录全部按继承处理。

`UpsertProviderEndpoint` 的 `body` 接受省略 `credentialsResolver`：缺省视为 `unknown`。

## 其他 API

无新增 operation。`/api/picotera/endpoints` 不变；`/api/picotera/provider-endpoints/fetch-models` 内部使用新的有效发送解析器，但响应体不变。

## 网关行为变更（非 OpenAPI 表述）

- 入站凭证读取：`endpoint.credentialsResolver` 指定的位置优先；为空时按 Bearer → X-Api-Key → `?key=` → X-Goog-Api-Key 顺序回退。
- 出站凭证发送：使用 `provider_endpoint.credentialsResolver` 覆盖 endpoint 的解析器（`unknown` 表示不覆盖）。
- 上游请求构造：剥离 `Authorization`、`X-Api-Key`、`X-Goog-Api-Key`、`Host`、`Content-Length` header（已有行为）；同时剥离客户端 query 中的 `key=`；其它 query 参数与 upstream URL 自带 query 合并，冲突时 upstream URL 一侧胜出。
