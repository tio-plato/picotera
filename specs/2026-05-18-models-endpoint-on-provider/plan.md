# Plan

## 1. 数据库迁移

1. 新建 `db/migrations/023_provider_models_endpoint.sql`：
   - `ALTER TABLE provider ADD COLUMN models_endpoint_url TEXT NOT NULL DEFAULT ''`
   - `ALTER TABLE provider ADD COLUMN models_endpoint_resolver INTEGER NOT NULL DEFAULT 0`
   - 回填语句：`UPDATE provider SET ... FROM (SELECT DISTINCT ON (provider_id) ...) fb WHERE id = fb.provider_id`，源数据为 `provider_endpoint JOIN endpoint ON endpoint_type = 6`，按 `endpoint_path` 字典序取首条。`COALESCE(NULLIF(pe.credentials_resolver, 0), e.credentials_resolver)` 保证 binding 自带 resolver 优先于 endpoint 自带 resolver。
   - `DELETE FROM provider_endpoint WHERE endpoint_path IN (SELECT path FROM endpoint WHERE endpoint_type = 6)`
   - `DELETE FROM endpoint WHERE endpoint_type = 6`
   - `-- +goose Down` 仅回滚 `ALTER`，删表数据不还原。

## 2. sqlc / Go DB 层

1. 修改 `db/queries/provider.sql`：
   - `CreateProvider` 参数列表新增 `models_endpoint_url`、`models_endpoint_resolver`。
   - `UpdateProvider` 新增 `set_models_endpoint_url`/`models_endpoint_url`、`set_models_endpoint_resolver`/`models_endpoint_resolver` 两组开关。
2. 跑 `sqlc generate`，更新 `pkg/db/provider.sql.go`、`pkg/db/models.go`、`pkg/db/querier.go`。

## 3. Contract

1. `pkg/contract/endpoint.go`：
   - 删除 `EndpointType_GeneralListModels` 常量。
   - `ToEndpointType` / `FromEndpointType` 移除 `generalListModels` 分支。
   - `EndpointView.EndpointType` 的 `enum:"..."` 标签去掉 `generalListModels`。
2. `pkg/contract/provider.go`：
   - `ProviderView` 增加 `ModelsEndpointUrl string`、`ModelsEndpointResolver string`（带 enum 标签）。
   - 同步 `CreateProviderRequest.Body`、`UpsertProviderRequest.Body`。
   - `ToProviderView` 把 `provider.ModelsEndpointUrl`、`FromCredentialsResolver(provider.ModelsEndpointResolver)` 写入视图。
   - `FromProviderView` 反向（暂时未被调用，保持对称即可）。
3. `pkg/contract/provider_endpoint.go`：
   - 移除 `FetchModelsRequest.Body.EndpointPath` 字段。
   - `OperationFetchModels.Path = "/providers/fetch-models"`。

## 4. Server handlers

1. `pkg/server/handle_providers.go`：
   - `handleCreateProvider` 和 `handleUpsertProvider` 把 `ModelsEndpointUrl` / `ToCredentialsResolver(ModelsEndpointResolver)` 传进 `CreateProviderParams` / `UpdateProviderParams`，开 `SetModelsEndpointUrl`/`SetModelsEndpointResolver`。
2. `pkg/server/handle_provider_endpoint.go::handleFetchModels`：
   - 删除 `GetProviderEndpoint` / `GetEndpointByPath` 两次查询。
   - 在拿到 `provider` 之后：
     ```go
     if provider.ModelsEndpointUrl == "" {
       return nil, huma.Error400BadRequest("provider has no models endpoint configured")
     }
     ```
   - `http.NewRequestWithContext(fetchCtx, http.MethodGet, provider.ModelsEndpointUrl, nil)`。
   - `applyCredentials(req, provider.Credentials, provider.ModelsEndpointResolver, nil)`。
   - JS 钩子调用里 `EndpointPath` 字段删除。
3. 路由注册：`registerOperations()` 中 `OperationFetchModels` 的路径变更跟随 contract，无需额外改动。

## 5. JS Hook

1. `pkg/jsx/types.go`：从 `RewriteProviderModelsInput` 删除 `EndpointPath` 字段。
2. `pkg/jsx/sdk.js`：若 SDK 注释提到 `endpointPath`，更新文档。
3. `pkg/jsx/engine_test.go`：若有断言 `endpointPath`，删除。
4. `docs/example-scripts/*.js`：grep `endpointPath`，把命中位置改写或删除（zenmux-models.js 不依赖该字段，无需改）。

## 6. 前端类型与常量

1. `mise run openapi` 重新生成 `openapi.yaml`。
2. `pnpm --dir dashboard generate-openapi` 重新生成 `dashboard/src/openapi-types.d.ts` 和 `dashboard/src/api/openapi.ts`。
3. `dashboard/src/api/index.ts`：
   - 删除 `ENDPOINT_TYPES_DIRECT` 导出。
   - `ENDPOINT_TYPE_LABELS` 删除 `generalListModels` 键。
4. `dashboard/src/api/client.ts::fetchProviderModels`：
   - 路径改为 `/api/picotera/providers/fetch-models`。
   - `body` 类型变更跟随 openapi-types 自动同步。

## 7. 前端 UI

1. `dashboard/src/components/ProviderForm.vue`：
   - `form` 增加 `modelsEndpointUrl: props.provider?.modelsEndpointUrl ?? ''`、`modelsEndpointResolver: props.provider?.modelsEndpointResolver ?? 'generalApiKey'`。
   - 模板「代理 URL」字段下方加两个 `Field`：URL 输入框 + resolver `<Select>`。
   - `submit()` 的 `body` 加上 `modelsEndpointUrl` 和 `modelsEndpointResolver`（URL 为空字符串也按原值传，保持后端可清空）。
2. `dashboard/src/components/ProviderModelsPanel.vue`：
   - 删除 `fetchEndpointPath` 状态、`groupedFetchSources` 计算属性、相应模板里的 `<Select>` 和 `<optgroup>`。
   - `fetchFromUpstream` 调 `fetchModelsMutation.mutateAsync({ providerId: props.providerId })`。
   - 拉取按钮 `disabled = fetching || !provider.modelsEndpointUrl`；按钮旁条件渲染提示「请先在渠道编辑表单配置模型列表 URL」。
   - 移除 `useQueries` 第三项 `listEndpoints` —— 不再需要 endpoint 元信息。
3. `dashboard/src/components/EndpointForm.vue`：无需手改，`ENDPOINT_TYPE_LABELS` 收缩后下拉自动减项。
4. `dashboard/src/views/ModelsView.vue`：
   - 删除 `routablePathSet` 里 `.filter((e) => e.endpointType !== 'generalListModels')`，直接 `endpoints.value.map((e) => e.path)`。

## 8. Go 测试调整

1. `pkg/server/handle_unified_gateway_test.go`：把 `contract.EndpointType_GeneralListModels` 替换为 `contract.EndpointType_Unknown`，保持「非 LLM 格式」断言语义。
2. `pkg/server/user_message_preview_test.go`：同上。

## 9. 验收

1. `go build ./cmd/picotera` 通过。
2. `go test ./pkg/...` 通过。
3. `pnpm --dir dashboard build` 通过（含 `vue-tsc` 类型检查）。
4. `pnpm --dir dashboard lint` 无新错误。
5. 跑一次本地 docker compose + `mise run server`，做手测：
   - 新建 provider，填写 URL + resolver，保存后从 `ProviderModelsPanel` 点拉取按钮，应该按新 URL 发起请求并拉回模型列表。
   - 留空 URL 时按钮禁用，提示渲染正确。
   - `Endpoints` 视图里 `generalListModels` 选项消失，新建端点不能再选这个类型。
   - 老库迁移后：旧 `generalListModels` 行已消失，provider 上的 URL/Resolver 已正确回填。
