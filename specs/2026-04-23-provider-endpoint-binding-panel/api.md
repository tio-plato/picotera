# API: 渠道端点绑定侧边栏

本次变更不引入新 API，全部复用已有接口。所有路径在 `/api/picotera` 下。

## 复用接口

### GET /provider-endpoints?providerId={id}

列出指定渠道的所有已绑定端点。响应体：`ProviderEndpointView[]`。
侧边栏打开、切换渠道、新增/删除/更新绑定后均调用此接口刷新。

### PUT /provider-endpoints

Upsert 单个绑定。Body:

```json
{
  "providerId": 1,
  "endpointPath": "/v1/chat/completions",
  "upstreamUrl": "https://api.example.com/v1/chat/completions"
}
```

- 侧边栏「新增」：首次写入 `(providerId, endpointPath)`。
- 侧边栏「就地编辑 upstreamUrl」：同一路径再次 PUT，走 ON CONFLICT DO UPDATE。

### POST /provider-endpoints/delete

删除绑定。Body `{ providerId, endpointPath }`。侧边栏删除按钮使用。

### GET /endpoints

获取全量端点列表，供侧边栏「新增」下拉渲染（过滤掉已绑定的 path）。

## MappingForm 相关

- 原先预加载的 `GET /endpoints` 不再使用；改为在选择 `providerId` 后调用 `GET /provider-endpoints?providerId=...`，只暴露已绑定的 endpointPath。
- `PUT /model-provider-endpoints` body 保持不变。

## 无变更

- 后端 Go 代码、contract、sqlc、migrations 均不改动。
- OpenAPI spec 与生成的 `dashboard/src/api.d.ts` 不需要重新生成。
