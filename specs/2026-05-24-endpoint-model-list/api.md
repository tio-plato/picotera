# API: 端点"模型列表"类型

## 网关端点

### GET/HEAD `<modelList 端点路径>`

通过 `endpointRouter` 匹配到 `endpointType=modelList` 的端点时触发。

**认证**：使用端点配置的 `credentialsResolver` 解析 API key。

**非 GET/HEAD 请求**：返回 404。

**响应**（200 OK）：

```json
{
  "object": "list",
  "data": [
    {"id": "model-name-1", "object": "model"},
    {"id": "model-name-2", "object": "model"}
  ]
}
```

`data` 数组包含所有有可用上游的模型，按名称字母序排列。每个条目只包含 `id`（模型名）和 `object`（固定为 `"model"`）两个字段。

## 管理 API 变更

### PUT `/api/picotera/endpoints`

`endpointType` 枚举新增 `"modelList"` 值。当 `endpointType` 为 `"modelList"` 时，`modelPath` 必须为空，否则返回 400。

### PUT `/api/picotera/provider-endpoints`

当目标端点的 `endpointType` 为 `"modelList"` 时，返回 400 错误："modelList endpoint cannot have provider bindings"。
