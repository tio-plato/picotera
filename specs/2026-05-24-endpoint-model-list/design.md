# Design: 端点"模型列表"类型

## 端点类型

新增 `EndpointType_ModelList int32 = 10`，字符串表示为 `"modelList"`。与 `exaSearch` 类似，`modelList` 端点的 `modelPath` 必须为空。

## 网关分发

在 `gatewayHandler.ServeHTTP` 中，端点路由匹配成功后、读取 body 之前，检查 `endpoint.EndpointType`：若为 `EndpointType_ModelList` 则直接跳转到 `handleModelList`，不走标准网关流程（不插入 meta request、不提取模型名、不运行 JS hooks、不转发上游）。

选择在读取 body 之前分流的原因：`modelList` 端点只处理 GET/HEAD 请求，没有 body 需要读取；提前分流避免了不必要的 body 读取和 meta request 插入。

## 可用模型查询

新增 sqlc 查询 `ListAvailableModelNames`，通过一条 SQL 语句返回所有"有至少一个可用上游"的模型名。判断可用的条件：

- `model.disabled = false`
- `provider.disabled = false`
- `provider_models` 中对应条目的 `disabled` 字段为 false
- `provider.credentials` 非空
- `provider_endpoint.upstream_url` 非空
- `provider_models` 条目的 `endpoints` 过滤允许该端点（为空或包含该端点路径）

查询不按端点类型过滤——只要模型在任意端点上有可用上游即算可用。

## 绑定限制

在 `handleUpsertProviderEndpoint` 中，查询目标端点的类型：若为 `modelList` 则拒绝绑定，返回 400 错误。

## 响应格式

```json
{
  "object": "list",
  "data": [
    {"id": "claude-sonnet-4-20250514", "object": "model"},
    {"id": "gpt-4o", "object": "model"}
  ]
}
```

与 OpenAI `/v1/models` 响应格式兼容。`data` 数组按模型名字母序排列。
