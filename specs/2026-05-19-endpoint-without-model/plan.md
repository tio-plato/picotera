# Plan

## 1. SQL & sqlc

- 在 `db/queries/routing.sql` 末尾追加 `GetProvidersByEndpoint`：
  - 参数：`endpoint_path text`。
  - 列与 `GetProvidersByEndpointAndModel` 对齐，模型相关列输出常量（`''::text`、`0::int`、`'{}'::jsonb`）。
  - `WHERE pe.endpoint_path = $1 AND p.disabled = FALSE`，不 join `model`、不展开 `provider_models`。
- 运行 `sqlc generate`，确认 `pkg/db/routing.sql.go` 新增 `GetProvidersByEndpoint` / `GetProvidersByEndpointParams` / `GetProvidersByEndpointRow`，`Querier` 接口同步扩展。

## 2. 后端 helper 重构

`pkg/server/gateway_helpers.go`：

- 新增内部 `providerCandidateRow`，字段为 handler 现在用到的全集（providerID、providerName、providerCredentials、providerPriority、upstreamURL、sendCredentialsResolver、proxyURL、providerAnnotations、modelAnnotations、modelName、upstreamModelName、entryPriority、entryAnnotations、endpointPath）。
- 新增 `fromModelRoutedRow(db.GetProvidersByEndpointAndModelRow) providerCandidateRow` 与 `fromNoModelRow(db.GetProvidersByEndpointRow) providerCandidateRow`。
- 把 `resolveProviders` 的签名改为返回 `[]providerCandidateRow`：
  - `modelName == ""`：调用 `GetProvidersByEndpoint`，对每行 `fromNoModelRow`。
  - `modelName != ""`：现状 query，对每行 `fromModelRoutedRow`。
  - 两支共用过滤（upstreamURL & credentials 非空）与排序逻辑（`providerPriority + entryPriority` 降序）。
  - 空结果路径继续返回 `gatewayError{404, "no provider available", NoProviderAvailable}`。

## 3. 网关 handler 适配

`pkg/server/handle_gateway.go`：

- 在 `extractModel` 调用前判断 `endpoint.ModelPath`；空则直接 `modelName = ""` 跳过抽取，不再触发现状的 400 报错。
- `rewriteModel` 钩子调用之后：若 `endpoint.ModelPath == ""` 且 `newModel != ""`，调用 `failHook(errors.New("rewriteModel returned non-empty model on no-model endpoint"))` 并 return；否则按现状继续（此时 `newModel == "" == modelName`，不进入 `sjson.SetBytes` 分支）。
- `modelAnno` 在 `modelName == ""` 时保持 nil；不再触发 providers[0].ModelAnnotations 的刷新分支（providers 行的 `ModelAnnotations` 是 `{}`，解码后是空 map，无差异）。
- candidate 构造改为读 `providerCandidateRow`。`CandidateMPE.ModelName`、`UpstreamModelName` 直接取 row 的字段（无模型端点下都是 `""`）。
- `modelJS` 始终构造为 `&jsx.ModelSummary{Name: originalModelName, Annotations: modelAnno}`；调用方不需要 nil 检查。
- 其余调用链（`session.RunSortHook`、`RunBeforeRequestHook`、`RunRewriteHook`、`buildUpstreamRequest`、`updateRequestModel`、`updateRequestOnHeader`、`updateRequestOnComplete`、`costsFor`）不动；这些函数遇到空字符串自然降级。

## 4. CRUD / 契约

- `pkg/contract/endpoint.go` 与 `pkg/server/handle_endpoint.go` 不改。
- 运行 `mise run openapi`，对比 `openapi.yaml` 没有意义变化（应该没有）。

## 5. Dashboard

- `dashboard/src/components/EndpointForm.vue`：
  - 删除 `modelPathRequired` 计算和 `submit()` 中 `if (modelPathRequired.value && !form.value.modelPath.trim())` 分支。
  - `<Field label="模型字段路径">` 去掉 `:required`，`<Input>` 去掉 `:required`，`placeholder` 固定为 `"可选，留空表示该端点不解析模型"`。
- `dashboard/src/views/EndpointsView.vue`：保持现状（`{{ e.modelPath || '—' }}` 已能渲染空值）。
- 运行 `pnpm --dir dashboard generate-openapi`（即便 openapi 无变化也走一遍以确保类型同步）。
- `pnpm --dir dashboard type-check` + `pnpm --dir dashboard lint`。

## 6. 验证

- `go build ./...` 通过。
- `go test ./pkg/server/... ./pkg/llmbridge/...` 通过（既有用例不依赖模型路径，应当无回归）。
- 手工：本地建一条 `model_path = ""` 的 endpoint，绑定 1–2 个 provider，向该路径发请求，确认：
  - meta + upstream 两条 request 行的 `model` / `upstream_model` 为 NULL。
  - 上游收到的请求体未被改写 model 字段。
  - 启用一条脚本，订阅 `sortProviders` / `beforeRequest`，确认 `candidate.mpe.modelName === ""`、`candidate.mpe.endpointPath` 正常。
  - 注册一条 `rewriteModel` 返回 `"foo"` 的脚本，确认网关返回 502 错误，错误消息提及 `rewriteModel returned non-empty model on no-model endpoint`。
