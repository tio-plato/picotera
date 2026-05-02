# API

## Schema 变更

`ProviderModelEntry`

| 字段              | 类型                          | 说明                              |
| ----------------- | ----------------------------- | --------------------------------- |
| `model`           | `string` (required)           | 本地模型名（同一 provider 内可重复） |
| `upstreamModelName` | `string` (optional)         | 上游模型名；缺省时使用 `model`        |
| `endpoints`       | `string[]` (optional)         | 仅在所列 endpoint 上启用；空数组等同未限定 |
| `priority`        | `int32` (optional)            | entry 内优先级，与 provider 优先级相加排序 |
| `annotations`     | `map[string]string` (optional) | entry 级标注                      |
| `disabled`        | `boolean` (optional)          | 单条 entry 禁用                   |

`ProviderView.providerModels` / `CreateProviderRequestBody.providerModels` / `UpsertProviderRequestBody.providerModels`：

- 旧：`{ [modelName: string]: ProviderModelEntry }`
- 新：`ProviderModelEntry[]`

## 端点不变

无新增 / 删除端点。仅 schema 变化：

- `GET /api/picotera/providers` → `ProviderView[]`
- `GET /api/picotera/providers/{id}` → `ProviderView`
- `POST /api/picotera/providers` (create) → 接受 `CreateProviderRequestBody`
- `PUT /api/picotera/providers` (upsert) → 接受 `UpsertProviderRequestBody`
- `POST /api/picotera/providers/delete`、`/api/picotera/provider-endpoints/*` 等其他端点不动。

## 行为约定

- `providerModels` 接受空数组 `[]`，表示该 provider 暂未配置任何上游映射；不接受 `null`，服务端会规范化为 `[]`。
- 同一 `(model, upstreamModelName, endpoints)` 三元组重复出现时，服务端不去重；按 `priority` 顺序参与路由。
- `model` 字段为空字符串的 entry 会在前端提交前被丢弃，不会写入数据库。
