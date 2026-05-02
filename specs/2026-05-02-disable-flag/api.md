# API changes

所有路径相对 `/api/picotera`。仅列出字段差异，未列出的字段保持现状。

## Provider

`ProviderView`（响应体）、`CreateProviderRequest.Body`、`UpsertProviderRequest.Body` 新增字段：

```jsonc
{
  "disabled": false           // bool, 默认 false
}
```

`ProviderModelEntry` 新增可选字段：

```jsonc
{
  "providerModels": {
    "<modelName>": {
      "upstreamModelName": "...",
      "endpoints": [],
      "priority": 0,
      "annotations": {},
      "disabled": false       // bool, 可省略，缺省视为 false
    }
  }
}
```

涉及操作：

- `GET /providers`：响应中每个 provider 含 `disabled`，每个 `providerModels` 条目可能含 `disabled`。
- `GET /providers/{id}`：同上。
- `POST /providers`：请求体可传入 `disabled`；省略时为 `false`。
- `PUT /providers`：请求体可传入 `disabled`；与其它字段一致采用「全量替换」语义。

## Model

`ModelView`（响应体）、`PutModelRequest.Body` 新增字段：

```jsonc
{
  "disabled": false           // bool, 默认 false
}
```

涉及操作：

- `GET /models`、`GET /models/{name}`：响应含 `disabled`。
- `PUT /models`：请求体可传入 `disabled`。

## Routing（内部，不对外暴露 HTTP）

`GetProvidersByEndpointAndModel` SQL 查询排除以下三类候选：

1. `provider.disabled = TRUE`
2. `provider.provider_models[modelName].disabled = true`
3. `model.disabled = TRUE`（通过新增 JOIN）

不引入新的网关返回码；当所有候选被禁用导致无可用路由，沿用现有「找不到 provider」错误路径。

## OpenAPI

修改 `pkg/contract/*` 类型后需要执行 `mise run openapi` 重新生成 `openapi.yaml`，前端通过 `openapi-typescript` 自动更新 `dashboard/src/api.d.ts`。
