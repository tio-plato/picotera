# Design

## 总览

`endpoint.model_path = ''` 把该 endpoint 标记为 "no-model endpoint"：网关不再做模型抽取，不再做按模型的 provider 过滤，仅按 `provider_endpoint` 绑定把所有 enabled provider 拿来做候选。下游 hook / 重试循环 / 上游请求构造逻辑保持现状，只是 model 维度被收敛成空值，MPE 维度退化成"只剩 endpointPath + providerId 的兼容对象"。

## 数据模型

不动 schema。`endpoint.model_path` 已经是 `TEXT NOT NULL`（migration 001），把空字符串作为有意义的取值即可。所有现存 endpoint 行的 `model_path` 都是非空（早先 `extractModel` 在空时直接 400），保留这条约束语义：空字符串只能由 upsert 显式设置，不会自然出现。

## 后端：路由解析分支

新增一条 sqlc query `GetProvidersByEndpoint`（`db/queries/routing.sql`），返回绑定到给定 `endpoint_path` 的所有非 disabled provider，每个 provider 一行。返回列尽量贴近 `GetProvidersByEndpointAndModel` 的形状，但抹平模型相关字段：

- `model_name`        → `''::text`
- `upstream_model_name` → `''::text`
- `priority`          → `0::int`（per-entry priority；用 provider.priority 排序就够）
- `annotations`       → `'{}'::jsonb`（entry annotations 不存在）
- `model_annotations` → `'{}'::jsonb`
- `provider_id`、`endpoint_path`、`provider_name`、`provider_credentials`、`provider_priority`、`upstream_url`、`send_credentials_resolver`、`proxy_url`、`provider_annotations`：照常从 join 取。

`WHERE pe.endpoint_path = $1 AND p.disabled = FALSE`。

`pkg/server/gateway_helpers.go` 引入内部聚合类型 `providerCandidateRow`，把 `GetProvidersByEndpointAndModelRow` 和 `GetProvidersByEndpointRow` 都映射到它，避免下游 handler 同时操心两种 sqlc 类型。`resolveProviders(endpointPath, modelName)` 在 `modelName == ""` 时走新 query；否则走原 query。两支都返回 `[]providerCandidateRow`，按 `provider_priority + priority` 排序后用 `upstream_url != ""` 与 `provider_credentials != ""` 做最小有效性过滤——与现状一致。

注：不为 unified gateway 引入对称改动。`/v1/messages`、`/v1/responses`、`/v1/chat/completions`、Gemini 两条路由总是从请求体或路径变量里能拿到 model，本特性只针对路径式 endpoint。

## 后端：网关 handler 分支

`pkg/server/handle_gateway.go`：

1. **跳过 extractModel**：当 `endpoint.ModelPath == ""` 时直接置 `modelName = ""`，不调用 `extractModel`。
2. **rewriteModel 钩子**：保持调用，输入 `model: ""`。返回值非空时直接 `failHook` 走错误路径（错误消息：`rewriteModel returned non-empty model on no-model endpoint`），状态码 502。
3. **modelAnno**：no-model 端点不查 `model` 表，`modelAnno = nil`（空 map）。`modelJS` 仍构造为 `*ModelSummary{Name: "", Annotations: nil}`，让 hook 输入字段结构稳定；JS 侧拿到 `model.name === ""` 即可识别。
4. **candidate 构造**：`Candidate.MPE` 用 `{ProviderID, EndpointPath, ModelName:"", UpstreamModelName:"", Priority:0, Annotations:{}}`。`candidate.annotations` 通过 `annoBuilder.merge(providerAnnotations, nil)` 得到（无 entry layer）。
5. **upstreamModel fallback 链**：现有逻辑 `dec.UpstreamModel → MPE.UpstreamModelName → modelName` 三层都会回退到 `""`。`buildUpstreamRequest` 既有的 `if upstreamModel != ""` 门已经保证不去改 body，无须新增分支。
6. **request 行**：meta 行的 `Model/UpstreamModel` 保持 `pgtype.Text{Valid:false}`；upstream 行同样不写。`updateRequestModel` 在 `modelName == ""` 时也是 invalid，保持现状。
7. **pricing**：`costsFor(originalModelName="", providerID, …)` 在模型为空时不会命中任何定价规则，返回的 cost 列均为 NULL —— 与既有 pricing match 行为一致。

`Server.fetchModelAnnotations` 在 model 为空时直接返回 nil（已存在的快速短路）。

## 后端：CRUD 校验

`handleUpsertEndpoint` 不增加额外校验：modelPath 是任意字符串。`POST /endpoints/delete` 不变。OpenAPI 契约 `EndpointView.modelPath` 已经是普通 `string`，不需要标记 optional。

## Dashboard

`dashboard/src/components/EndpointForm.vue`：

- 删除 `modelPathRequired` 这条 computed 以及其上的 `submit()` 校验分支，让 `modelPath` 永远可选。
- 仍保留输入框；placeholder 改为 "可选，留空表示该端点不解析模型"。

`dashboard/src/views/EndpointsView.vue` 的 `'—'` 占位渲染已经能正确显示空 `modelPath`，无须改动。

## 不做的事

- 不改 OpenAPI（modelPath 仍是 string）。
- 不引入新的 `endpoint_type` 常量；不写迁移；不动 `ENDPOINT_TYPES_MODEL_ROUTED`。
- 不在 unified gateway / simulate handler 中支持无模型路径。
- 不为已有的"模型必填"端点写兼容/迁移逻辑——只是新允许 `model_path = ""`。
