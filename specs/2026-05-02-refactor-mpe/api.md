# API Changes

所有路径前缀 `/api/picotera`。

## 1. 类型定义

```ts
type ProviderModelEntry = {
  upstreamModelName?: string
  endpoints?: string[]                // 未填或空数组 = 全部已绑定端点
  priority?: number                   // 默认 0
  annotations?: Record<string, string> // 默认 {}
}

type ProviderView = {
  id: number
  name: string
  credentials: string
  priority: number
  providerModels: Record<string, ProviderModelEntry>
  annotations: Record<string, string>
}
```

Go 端 `ProviderView.ProviderModels` 类型：`map[string]ProviderModelEntry`。
JSON 序列化时，`ProviderModelEntry` 字段全部 `omitempty`。

## 2. 变更的 operation

### `GET /providers`、`GET /providers/{id}`
响应中 `providerModels` 改为 object 形态。

### `POST /providers`、`PUT /providers`
请求体 `providerModels` 改为 object 形态。`PUT` 使用 `set_provider_models` 时整体替换为新的 object。

### `POST /provider-endpoints/fetch-models`
请求体不变：`{ providerId, endpointPath }`。

行为变更：
- 不再写库。
- 仅向上游 GET 请求模型列表，解析后回传。

响应不变：

```ts
{
  providerId: number
  models: string[]   // 上游解析出的模型名（去重、排序）
}
```

## 3. 删除的 operation

整组删除：

| Method | Path |
|---|---|
| GET  | `/model-provider-endpoints` |
| GET  | `/model-provider-endpoints/get` |
| PUT  | `/model-provider-endpoints` |
| POST | `/model-provider-endpoints/delete` |

OpenAPI spec、生成的前端类型、`registerOperations()` 内对应行均删除。

## 4. 路由查询（内部，不暴露 HTTP）

`GetProvidersByEndpointAndModel` 改为读 `provider_models`：

```sql
SELECT
  $2::text AS model_name,
  p.id AS provider_id,
  pe.endpoint_path,
  COALESCE(pm ->> 'upstreamModelName', '') AS upstream_model_name,
  COALESCE((pm ->> 'priority')::int, 0)   AS priority,
  COALESCE(pm -> 'annotations', '{}'::jsonb) AS annotations,
  p.name AS provider_name,
  p.credentials AS provider_credentials,
  p.priority    AS provider_priority,
  pe.upstream_url,
  p.annotations AS provider_annotations
FROM provider AS p
JOIN provider_endpoint AS pe ON pe.provider_id = p.id
CROSS JOIN LATERAL (SELECT p.provider_models -> $2::text AS pm) sub
WHERE pe.endpoint_path = $1
  AND p.provider_models ? $2::text   -- GIN(jsonb_path_ops) 命中
  AND sub.pm IS NOT NULL
  AND (
    sub.pm -> 'endpoints' IS NULL
    OR jsonb_typeof(sub.pm -> 'endpoints') <> 'array'
    OR jsonb_array_length(sub.pm -> 'endpoints') = 0
    OR sub.pm -> 'endpoints' @> to_jsonb(ARRAY[pe.endpoint_path])
  );
```

返回行类型 `GetProvidersByEndpointAndModelRow` 字段名保持兼容（与现有路由代码对齐）。
