# Plan

## Phase 1 — 数据库与 sqlc

1. 新建 `db/migrations/006_inline_provider_models.sql`：
   - `Up`：清空 `provider_models` 为 `{}` + `DROP TABLE model_provider_endpoint` + `CREATE INDEX idx_provider_models_gin ON provider USING GIN (provider_models jsonb_path_ops)`。
   - `Down`：`DROP INDEX idx_provider_models_gin` + 重建 `model_provider_endpoint`（复用 001 的定义）+ 还原 `provider_models` 为 `[]`。
2. 删除 `db/queries/model_provider_endpoint.sql`。
3. 改写 `db/queries/routing.sql` 中 `GetProvidersByEndpointAndModel`：用 `provider_models ? $2`（命中 GIN 索引）+ `provider_models -> $2` 提取子对象，按 `endpoints` 过滤；行字段名保持向后兼容。
4. 运行 `sqlc generate`，确认 `pkg/db/` 重新生成。

## Phase 2 — Go contract / handlers

5. 改写 `pkg/contract/provider.go`：
   - 新增 `ProviderModelEntry` 类型；
   - `ProviderView.ProviderModels` 改为 `map[string]ProviderModelEntry`；
   - `CreateProviderRequest` / `UpsertProviderRequest` body 同步；
   - `ToProviderView` / `FromProviderView` 解 / 编 `provider_models` 为 map。
6. 删除 `pkg/contract/model_provider_endpoint.go`。
7. 删除 `pkg/server/handle_model_provider_endpoint.go`。
8. `pkg/server/server.go` `registerOperations()`：移除 4 个 model-provider-endpoint 注册；保留其他。
9. `pkg/errorx`：移除 `ModelProviderEndpointNotFound`（如仅被该 handler 使用）。
10. `pkg/server/handle_provider_endpoint.go` 的 `handleFetchModels`：删除写库分支；解析后直接返回 models 列表。

## Phase 3 — Gateway 适配

11. `pkg/server/gateway_helpers.go` `resolveProviders`：行类型保持兼容，无需改动；确认 `candidateUpstreamModel` / sortProviders 输入仍取自 `MPE` map。
12. `pkg/server/handle_gateway.go` 第 213–234 行候选构建：字段名不变，验证 row 中 `Annotations / UpstreamModelName / Priority` 仍是同名字段。
13. 手测：`go build ./...` 通过；启动后向已配置 provider 发请求能命中。

## Phase 4 — OpenAPI 与前端类型

14. 运行 `mise run openapi`，更新 `openapi.yaml`。
15. 运行 `pnpm --dir dashboard codegen`（即 `openapi-typescript`）刷新 `src/api.d.ts` / `src/openapi-types.d.ts`。
16. 调整 `src/api/index.ts` 暴露的 `ProviderView` 类型（如果是手写转出）。

## Phase 5 — 前端 UI

17. 新建 `dashboard/src/components/ProviderModelsPanel.vue`：
    - props：`providerId: number`、`providerName: string`；
    - 内部：加载该 provider 的最新数据 + 已绑定 endpoints；
    - UI：模型行列表（行内编辑 `upstreamModelName / priority / endpoints / annotations`），「从上游拉取」按钮，diff 弹层（勾选要删除的本地模型），「保存」整体 `PUT /providers`。
18. `ProvidersView.vue`：在行操作里加「模型」按钮，调用 `panel.toggle(ProviderModelsPanel, …)`；保留「端点绑定」「编辑」「删除」按钮。
19. `ProviderForm.vue`：移除 `ModelListEditor` 与 `providerModels` 字段；提交时若是新建，`providerModels` 默认 `{}`，编辑时保留现有值不动。
20. `ProviderEndpointsPanel.vue`：删除「拉取模型」按钮与相关状态；面板内只剩端点 CRUD。
21. 删除 `dashboard/src/components/ModelListEditor.vue`。
22. 删除 `dashboard/src/components/MappingForm.vue` 与 `dashboard/src/views/MappingsView.vue`。
23. `src/router/index.ts`：移除 `mappings` 路由。
24. `src/App.vue`：`pageMeta` 移除 `mappings` 条目。
25. `AppSidebar.vue`：移除「模型映射」入口。

## Phase 6 — 验证

26. `pnpm --dir dashboard type-check`、`pnpm --dir dashboard lint`、`pnpm --dir dashboard build` 全部通过。
27. `go build ./cmd/picotera/main.go` 通过。
28. 启动后端 + 前端，手测：
    - 创建 provider；
    - 在「模型」面板添加 1 条模型 + 1 条带 `endpoints` 限制的模型 + 1 条带 `upstreamModelName` 的模型；
    - 点击「从上游拉取」→ 验证新增合并、删除二次确认；
    - 保存后通过 gateway 路径请求模型，确认路由命中、`endpoints` 过滤生效；
    - 删除 model-provider-endpoints 资源后 `/model-provider-endpoints` 路径返回 404。
