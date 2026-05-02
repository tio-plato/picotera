# Plan

## Phase 1 — 数据库 / sqlc

1. 新建 `db/migrations/008_provider_models_array.sql`：
   - **Up**：把 `provider_models` 形态由对象转为数组（见 design.md），保留全部已有 entry。
   - **Down**：用 `jsonb_object_agg` 把数组聚回对象；同一 `model` 多 entry 时仅保留最后一条（已知精度损失）。
2. 改写 `db/queries/routing.sql` 中 `GetProvidersByEndpointAndModel`：
   - 用 `CROSS JOIN LATERAL jsonb_array_elements(p.provider_models) AS elem` 展开；
   - 新增 `p.provider_models @> jsonb_build_array(jsonb_build_object('model', $1))` 命中 GIN 索引；
   - 用 `elem ->> 'model' = $1` 精确匹配；
   - 输出列名与现状一致。
3. 运行 `sqlc generate`，确认 `pkg/db/routing.sql.go` 重新生成、列签名未变。

## Phase 2 — Go contract / handlers

4. 修改 `pkg/contract/provider.go`：
   - `ProviderModelEntry` 新增 `Model string \`json:"model"\``（必填、不带 omitempty）；
   - `ProviderView.ProviderModels` 改为 `[]ProviderModelEntry`；
   - `CreateProviderRequest.Body.ProviderModels` / `UpsertProviderRequest.Body.ProviderModels` 同步改为切片；
   - `ToProviderView`：把 jsonb 解到 `[]ProviderModelEntry{}`，nil 保护为空切片；
   - `FromProviderView`：把切片 marshal 回 jsonb；nil 切片落库为 `[]`。
5. 修改 `pkg/server/handle_providers.go`：
   - `handleCreateProvider` / `handleUpsertProvider`：在 marshal 前若 `input.Body.ProviderModels == nil` 则取 `[]ProviderModelEntry{}`。
   - 其余字段无变化。
6. `go build ./...` 通过；保留所有既有 handler 行为。

## Phase 3 — Gateway 兼容性核对

7. `pkg/server/handle_gateway.go` 第 218–239 行候选构建：行字段名未变（`ModelName`、`UpstreamModelName`、`Priority`、`Annotations`），无需改动；JS hook 看到的 `mpe` shape 不变。
8. `pkg/server/gateway_helpers.go` `resolveProviders` / `candidateUpstreamModel` 无改动；同一 provider+model 多行时，按现有 sort 逻辑自然展开为多个候选 entry。

## Phase 4 — OpenAPI 与前端类型

9. 运行 `mise run openapi`，刷新 `openapi.yaml`。
10. 运行 dashboard 的类型生成（`pnpm --dir dashboard …` 当前 codegen 流程）刷新 `src/api.d.ts` 与 `src/openapi-types.d.ts`，确认 `ProviderView.providerModels` 已变为 `ProviderModelEntry[]` 且 `ProviderModelEntry.model` 必填。

## Phase 5 — 前端组件

11. `dashboard/src/components/ProviderModelsPanel.vue`：
    - `rowsFromProvider`：从数组构造 `Row[]`，按 `(model asc, priority desc)` 排序；
    - `rowsToObject` 重命名为 `rowsToList`：返回 `ProviderModelEntry[]`，过滤 `model` 为空的行；保留各字段 omit-empty 规则；
    - `addModel`：删除 `已存在` 校验，允许同名 entry；
    - 「从上游拉取」「未在上游」检测改为按 `Array.from(new Set(rows.map(r => r.modelName)))` 比较；删除「本地缺失」时按名字批量删行；
    - `save()` 提交 `providerModels: rowsToList(rows.value)`；
    - `:key="row.uid"` 已是合成键，不必改。
12. `dashboard/src/views/ModelsView.vue`：
    - `upstreamIndex` 改为遍历 `provider.providerModels`（数组）：每条 entry 产出一条 `Upstream`，键为 `entry.model`；
    - `orphanRows` 仍按 entry.model 聚合（用 `Set` 去重）。
13. `dashboard/src/components/ModelUpstreamsPanel.vue`：
    - `:key="u.providerId"` 改为 `:key="`${u.providerId}:${i}`"`，新增 `i` 索引避免同 provider 多 entry 冲突。
14. `dashboard/src/views/ProvidersView.vue`：
    - `modelNames(p)` 改为 `Array.from(new Set(p.providerModels?.map(e => e.model) ?? []))`。
15. `dashboard/src/components/ProviderForm.vue`：
    - `providerModels: props.provider?.providerModels ?? []`（默认空数组）。
16. 全仓 grep `providerModels` 确认其他地方（`docs/superpowers/`、`specs/`）不影响运行时；运行时无遗漏。

## Phase 6 — 验证

17. 在干净 docker 环境下：
    - 启服务，触发 `008` 迁移；
    - SQL 抽查：`SELECT id, jsonb_typeof(provider_models), jsonb_array_length(provider_models) FROM provider;` 全部返回 `array`。
18. `pnpm --dir dashboard type-check`、`pnpm --dir dashboard lint`、`pnpm --dir dashboard build` 通过。
19. `go build ./cmd/picotera` 通过。
20. 手测：
    - 在 `ProviderModelsPanel` 为同一 provider 添加 2 条同名 entry（不同 `upstreamModelName` / `priority`），保存后刷新读回，2 条均存在；
    - 在 gateway 通过该 model 发起请求，确认两条 entry 都进入候选列表并按 priority 排序；
    - 删除其中一条，剩余的仍可路由；
    - `ModelsView` 的「上游」列与抽屉正确显示同一 model 多上游；
    - 触发回滚（goose down 至 007）后，`provider_models` 形态恢复为对象，且未抛错（多 entry 同 model 时保留一条，是预期行为）。
