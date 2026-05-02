# Plan: Disable flag for routing targets

## 1. 数据库迁移

1.1 新建 `db/migrations/007_disable_flag.sql`：
- Up：
  - `ALTER TABLE provider ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE;`
  - `ALTER TABLE model ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE;`
- Down：
  - `ALTER TABLE model DROP COLUMN disabled;`
  - `ALTER TABLE provider DROP COLUMN disabled;`

## 2. sqlc 查询

2.1 `db/queries/provider.sql`：
- `CreateProvider`：参数列表加 `disabled` 列。
- `UpdateProvider`：新增 `set_disabled` / `disabled` 入参，沿用 `CASE WHEN @set_disabled::bool ...` 模式。

2.2 `db/queries/model.sql`：
- `UpsertModel`：列表加 `disabled`，`ON CONFLICT DO UPDATE SET ... disabled = $5`。

2.3 `db/queries/routing.sql` 重写 `GetProvidersByEndpointAndModel`：
- `JOIN model AS m ON m.name = sqlc.arg('model_name')::text`
- 在 WHERE 中追加：`AND p.disabled = FALSE AND m.disabled = FALSE AND COALESCE((sub.pm ->> 'disabled')::boolean, false) = false`

2.4 运行 `sqlc generate`，确认 `pkg/db/` 重新生成无误。

## 3. Backend 类型与 handler

3.1 `pkg/contract/provider.go`：
- `ProviderModelEntry` 加 `Disabled bool \`json:"disabled,omitempty"\``。
- `ProviderView`、`CreateProviderRequest.Body`、`UpsertProviderRequest.Body` 加 `Disabled bool \`json:"disabled"\``。
- `ToProviderView` / `FromProviderView` 透传 `Disabled`。

3.2 `pkg/contract/model.go`：
- `ModelView` 加 `Disabled bool \`json:"disabled"\``。
- `ToModelView` 返回 `Disabled: model.Disabled`。

3.3 `pkg/server/handle_providers.go`：
- `Create`、`Upsert` 调用 sqlc 时传入 `Disabled`。
- `Update` 路径设置 `SetDisabled: true, Disabled: input.Body.Disabled`。

3.4 `pkg/server/handle_models.go`：
- `Put` 调用 sqlc 时传入 `Disabled`。

3.5 运行 `go build ./...` 确认编译通过。

## 4. OpenAPI 与前端类型

4.1 `mise run openapi` 重新生成 `openapi.yaml`。
4.2 前端运行 `pnpm --dir dashboard build`（或开发服务器自动）触发 `openapi-typescript`，确认 `dashboard/src/api.d.ts` 含 `disabled` 字段。

## 5. 前端 UI

5.1 `dashboard/src/components/ProviderForm.vue`：
- 表单 state 加 `disabled` 字段（默认从 `props.provider?.disabled ?? false`）。
- 加 `Field label="状态"` 复选框，文案「禁用此渠道（不参与调度）」。
- submit 时透传 `disabled`。

5.2 `dashboard/src/components/ModelForm.vue`：
- 同 5.1，复选框文案「禁用此模型（不参与调度）」。

5.3 `dashboard/src/components/ProviderModelsPanel.vue`：
- 每个 model entry 卡片右上角加禁用 IconButton（`eye-off` / `eye`）；编辑态展开区加复选框。
- 切换按钮：直接修改 `providerModels[modelName].disabled` 并 `PUT /providers` 全量保存。
- 禁用条目卡片加 `opacity-55` 视觉降级，名称旁渲染 `Badge variant="muted"` 显示「已禁用」。

5.4 `dashboard/src/views/ProvidersView.vue`：
- 操作列首位加禁用切换 IconButton；点击调用 `PUT /providers` 透传当前 provider 全字段并翻转 `disabled`。
- 已禁用行：行整体 `opacity-55`，名称旁加 `Badge variant="muted"`「已禁用」。

5.5 `dashboard/src/views/ModelsView.vue`：
- 操作列首位加禁用切换 IconButton；点击调用 `PUT /models` 透传全字段并翻转 `disabled`。
- 名称右侧追加 `<span class="text-ink-faint">（已禁用）</span>`（仅当 `disabled = true`）。

5.6 `dashboard/src/ui/icons/paths.ts` 与 `IconName`：如 `eye-off` 未注册，从 `@tabler/icons-vue` 拷贝路径补充。

## 6. 验证

6.1 `pnpm --dir dashboard type-check` 与 `pnpm --dir dashboard lint` 通过。
6.2 `go build ./...` 通过。
6.3 手动验证：
- 启动 docker-compose，跑后端 + dashboard。
- 三处禁用切换（provider 行、provider_models entry、model 行）UI 即时反映状态。
- 通过被禁用 provider/model/entry 发起网关请求，确认路由不到该候选；恢复后能再次命中。
- `PUT` 请求 body 含 `disabled`，`GET` 响应含 `disabled`。
