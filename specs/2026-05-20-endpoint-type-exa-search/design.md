# Design

## 总览

`exaSearch` 是 `endpoint_type` 枚举的一个新成员，编号 `9`，字符串视图 `"exaSearch"`。它的语义是"这是一个 Exa 兼容的搜索端点"，仅作为 UI 标签与未来分析维度存在，不在网关运行时引入任何新分支。

它的运行时表现完全等同于一个 `model_path = ""` 的无模型端点（由 2026-05-19 已落地的"endpoint without model"机制承接）：跳过模型抽取，所有绑定该 path 的 provider 都是候选，meta/upstream 的 request 行 `model` / `upstream_model` 留空。

## 数据模型

不动 schema。`endpoint.endpoint_type` 列在 migration 009 里已经是 `INTEGER NOT NULL DEFAULT 1`，新枚举值仅需在 Go contract 与前端字面量映射处扩展。

## 后端

`pkg/contract/endpoint.go`：

- 增加常量 `EndpointType_ExaSearch int32 = 9`。
- `ToEndpointType` 增加 `case "exaSearch": return EndpointType_ExaSearch`。
- `FromEndpointType` 增加 `case EndpointType_ExaSearch: return "exaSearch"`。
- `EndpointView.EndpointType` 的 Huma `enum` 标签把 `exaSearch` 追加到尾部。

`pkg/server/handle_endpoint.go`、`pkg/server/handle_gateway.go`、`pkg/server/gateway_helpers.go`、`pkg/server/user_message_preview.go`、`pkg/server/response_aggregation.go` 一律不动 —— `exaSearch` 在所有 switch 里都落入 default：

- `user_message_preview.go::extractUserMessage`：default 分支会顺序尝试四种 LLM body 抽取，Exa 搜索请求体不命中任何模式，自然返回空 preview，行为正确。
- `response_aggregation.go::responseAggregationFormat`：default 返回 `FormatUnknown, false`，artifact 不做聚合，行为正确。
- `handle_gateway.go`：对 `exaSearch` 端点而言，路由解析、provider 候选、JS 钩子等全部走"普通端点"路径，由 `endpoint.ModelPath == ""` 触发 no-model 分支（见 step 5 / step 6a / step 7 / step 8a / step 8d 的现有空字符串短路）。

`pkg/server/handle_unified_gateway.go`：不动；unified 路由只服务四种 LLM 源格式，与 `exaSearch` 无关。

## 后端：CRUD 校验

`handleUpsertEndpoint` 增加一条服务端校验：当 `endpointType == "exaSearch"` 时，`modelPath` 必须为空字符串；否则返回 400（`errorx.InvalidArgument`，message：`exaSearch endpoint must have empty modelPath`）。

理由：`exaSearch` 是纯标签，但语义上它一定是无模型端点。前端会锁定输入框；后端做最后兜底，确保通过 OpenAPI 客户端或 `curl` 直接调用时也无法构造非法组合。

## OpenAPI / 类型生成

`EndpointView.endpointType` 的 Huma `enum` 标签扩展后，跑一遍 `mise run openapi` + `pnpm --dir dashboard generate-openapi`，前端 `EndpointType` 字面量类型会自动加上 `exaSearch`。

## 前端

### `dashboard/src/api/index.ts`

- `ENDPOINT_TYPES_MODEL_ROUTED` 不动（`exaSearch` 不是模型路由型）。
- `ENDPOINT_TYPE_LABELS` 追加 `exaSearch: 'Exa 搜索'`。

### `dashboard/src/components/EndpointForm.vue`

- `endpointTypeOptions` 保持现状（从 `ENDPOINT_TYPE_LABELS` 派生，自动包含 `exaSearch`）。
- 新增 `computed` `isModelPathLocked = computed(() => form.value.endpointType === 'exaSearch')`。
- "模型字段路径" 输入框：
  - 绑定 `:disabled="isModelPathLocked"`。
  - `placeholder` 动态：`isModelPathLocked` 为 `true` 时显示 `"Exa 搜索端点不解析模型"`，否则保持现状。
  - 增加 `watch(() => form.value.endpointType)`：切换到 `exaSearch` 时把 `form.value.modelPath` 清成 `""`（不保留之前输入的值，避免提交时残留）。
- 提交逻辑不变（`modelPath` 直接发出当前值，锁定下就是 `""`）。

### `dashboard/src/views/EndpointsView.vue`

- `endpointTypeVariant`：增加 `if (t === 'exaSearch') return 'more'`，让它在表格里以 `more` 样式显示（与 `unknown` 同色阶，区别于 `general` 的 muted 与模型路由型的 accent）。当前默认 `more` 分支已能渲染，但显式列出更清晰。

### `dashboard/src/components/ProviderModelsPanel.vue`

不动 —— `exaSearch` 不在 fetch-models 来源候选集里，现有过滤逻辑（限 `general` / `generalListModels`）已经把它排除。

## 不做的事

- 不引入 Exa 专属请求体解析、响应聚合、token 计费、artifact preview。
- 不为 Exa 引入新的 unified 路由（Exa 没有跨格式互转需求）。
- 不动 `endpoint_router.go`、`endpoint_router_test.go`、`project_extractor.go`。
- 不动数据库迁移（编号、列定义复用 migration 009）。
- 不写 Go 单元测试 —— 新增分支只在 enum 互转和一条 upsert 校验中，与既有同类逻辑同形，跑通 `go build` 即可。
