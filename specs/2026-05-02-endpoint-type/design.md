# Design: Endpoint type field

## 背景

`endpoint` 表当前只有 `path / name / model_path / credentials_resolver` 四列。需要新增"端点类型"语义信号，区分上游 API 的协议形态：

- **直传/列表型**：`general`、`generalListModels`——picotera 不解析请求体，端点本身不依赖模型名。fetch-models 流程消费这两类。
- **模型路由型**：`openaiChatCompletions`、`openaiResponses`、`anthropicMessages`、`anthropicCountTokens`——picotera 需要从请求体里抽出模型名做调度，因此 `model_path` 必填。

随之放宽 `model_path` 的"必填"语义，并把前端 fetch-models 来源端点选择器收紧到合适的子集。

## 数据模型

### `endpoint` 表变更

```sql
ALTER TABLE endpoint
    ADD COLUMN endpoint_type INTEGER NOT NULL DEFAULT 1;  -- general
```

存量行 `endpoint_type` 默认进入 `general`（=1）。`model_path` 列定义保持不变（`TEXT NOT NULL`）；直传/列表型端点用空字符串占位。

### 枚举定义（`pkg/contract/endpoint.go`）

复用现有 `CredentialsResolver_*` 模式：

```go
const (
    EndpointType_Unknown              int32 = 0
    EndpointType_General              int32 = 1
    EndpointType_OpenAIChatCompletions int32 = 2
    EndpointType_OpenAIResponses      int32 = 3
    EndpointType_AnthropicMessages    int32 = 4
    EndpointType_AnthropicCountTokens int32 = 5
    EndpointType_GeneralListModels    int32 = 6
)

func ToEndpointType(s string) int32   // string → int32（默认 Unknown）
func FromEndpointType(t int32) string // int32 → string（默认 "unknown"）
```

JSON 字符串值：`unknown / general / openaiChatCompletions / openaiResponses / anthropicMessages / anthropicCountTokens / generalListModels`。

### `EndpointView` 调整

```go
type EndpointView struct {
    Name                string `json:"name"`
    Path                string `json:"path"`
    ModelPath           string `json:"modelPath"`
    CredentialsResolver string `json:"credentialsResolver" enum:"generalApiKey,bearerToken,xApiKey,unknown"`
    EndpointType        string `json:"endpointType" enum:"general,openaiChatCompletions,openaiResponses,anthropicMessages,anthropicCountTokens,generalListModels,unknown"`
}
```

`ModelPath` 仍是 `string`，允许空串；`EndpointType` 走字符串与整型互转。

## 后端

- `db/queries/endpoint.sql`：`UpsertEndpoint` 增加 `endpoint_type` 列；`model_path` 不动。
- `pkg/server/handle_endpoint.go`：upsert 时把 view 的 `EndpointType string` 转成 sqlc 入参；get 时反向拼成 view。
- `pkg/server/gateway_helpers.go::extractModel`：在入口处 `if modelPath == ""` 时直接返回 `gatewayError`（status 400，code `errorx.ModelNotFound`，message 改为 `endpoint has no model path configured`），避免把空路径喂给 gjson。
- `pkg/server/handle_provider_endpoint.go`：`handleFetchModels` 不强约束端点类型，前端先做过滤；后端不读 `ModelPath`，无需改动。

## 前端

### 共享：`endpointType` 字面量

新增 `dashboard/src/api/index.ts` 导出 `EndpointType` 类型，从 `components['schemas']['EndpointView']['endpointType']` 推导，避免散落字符串。

### `EndpointForm.vue`

- 表单 state 增加 `endpointType`（默认 `general`）。
- 类型选择器使用 `Select`，列出全部七个枚举（`unknown` 保留兜底）。
- 模型字段路径输入框：根据当前 `endpointType` 计算 `modelPathRequired`：
  - `openaiChatCompletions / openaiResponses / anthropicMessages / anthropicCountTokens` → 必填，`<Field>` 标签加 `required`，输入框 `required`，placeholder 不变。
  - 其它 → 选填，标签去掉 `required`，placeholder 替换为 `可选`。
- submit 时：选填时若为空字符串，`modelPath` 字段提交为 `null`（或 omit）；必填时校验非空。

### `EndpointsView.vue`

- 列表新增"类型"列，展示 `<Tag>`（直传/列表型用 `muted`，模型路由型用 `accent`）。
- "模型字段"列：当 `modelPath` 为空时显示 `—`。

### `ProviderModelsPanel.vue`（fetch-models 来源选择器）

`fetchEndpointPath` 选择器目前列出所有已绑定端点。改为：

- 加载时与 `provider-endpoints` 并行 `GET /endpoints`，获得 `endpointType` 元信息；按 `endpointType` 把 provider 已绑定的端点分组。
- `Select` 内部使用 `<optgroup>`：
  - 第一组 `label="模型列表"`：仅包含 `generalListModels` 类型端点。
  - 第二组 `label="通用"`：仅包含 `general` 类型端点。
- 其它类型（`openai*` / `anthropic*` / `unknown`）从下拉中剔除。
- 默认选中策略：优先选第一个 `generalListModels`，否则第一个 `general`，否则空（按钮禁用，与现有行为一致）。

## 风险与权衡

- **空字符串 vs 可空列**：保留 `NOT NULL`，业务层用空串约定可省，避免 sqlc 类型从 `string` 切到 `pgtype.Text` 的扇出改动。
- **枚举字符串与整型双向映射**：保持与 `CredentialsResolver` 一致的 switch 模式，未引入新工具。
- **前端类型过滤本地完成**：避免后端 `/provider-endpoints` 接口扩展查询参数；endpoint 列表数据量小（与 endpoints 视图复用），开销可忽略。
