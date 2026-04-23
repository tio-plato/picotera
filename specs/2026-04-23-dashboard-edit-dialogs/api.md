# API: Dashboard 编辑对话框与映射端点字段修正

所有路径均挂在 `/api/picotera` 下。

## Provider

### PUT /providers — upsertProvider

Request body:

```json
{
  "id": 0,
  "name": "openai",
  "credentials": "sk-...",
  "priority": 0,
  "providerModels": ["gpt-4o"],
  "annotations": {}
}
```

- `id` 省略或为 0 时视为创建。
- 有 `id` 时执行 Update；不存在返回 404 `PROVIDER_NOT_FOUND`。

Response body: `ProviderView`

### POST /providers/delete — deleteProvider

Request body:

```json
{ "id": 1 }
```

Response: 204（或 200 空 body）。不存在返回 404 `PROVIDER_NOT_FOUND`。

## Model

### POST /models/delete — deleteModel

Request body:

```json
{ "name": "gpt-4o" }
```

Response: 204。

（`PUT /models` / `GET /models` / `GET /models/{name}` 保持不变。）

## Endpoint Path 字段改名

### ProviderEndpointView / ModelProviderEndpointView

字段变更：

- `endpointId: int32` → `endpointPath: string`

### GET /model-provider-endpoints

查询参数变更：

- `endpointId` → `endpointPath`

### GET /model-provider-endpoints/get — getModelProviderEndpoint

原 `GET /model-provider-endpoints/{modelName}/{providerId}/{endpointId}`，改为 query 形式以安全承载 path：

```
GET /model-provider-endpoints/get?modelName=...&providerId=...&endpointPath=/api/v1/chat/completions
```

### POST /model-provider-endpoints/delete / POST /provider-endpoints/delete

Request body 字段同步改为 `endpointPath`。

### PUT /model-provider-endpoints / PUT /provider-endpoints

Body 中 `endpointId` → `endpointPath`。
