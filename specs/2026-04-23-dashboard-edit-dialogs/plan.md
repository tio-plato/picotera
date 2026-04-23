# Plan: Dashboard 编辑对话框与映射端点字段修正

## 1. 数据库迁移

直接编辑 `db/migrations/001_initial.sql`（项目早期，数据可丢弃，迁移不做向后兼容）：

- `provider_endpoint` 表：`endpoint_id INTEGER` → `endpoint_path TEXT NOT NULL`，主键改为 `(provider_id, endpoint_path)`。
- `model_provider_endpoint` 表：`endpoint_id INTEGER` → `endpoint_path TEXT NOT NULL`，主键改为 `(model_name, provider_id, endpoint_path)`。
- `request` 表：`endpoint_id INTEGER` → `endpoint_path TEXT NOT NULL`。

执行完后开发者需重建数据库：`docker compose down -v && docker compose up -d`，下次启动自动跑迁移。

## 2. SQL 查询更新

更新 `db/queries/` 中所有引用 `endpoint_id` 的语句改为 `endpoint_path`：

- `provider_endpoint.sql`：`UpsertProviderEndpoint`、`DeleteProviderEndpoint`、`ListProviderEndpoints` 排序改为 `ORDER BY endpoint_path`。
- `model_provider_endpoint.sql`：`GetModelProviderEndpoint`、`ListModelProviderEndpoints`（含 narg + 游标三元组中 endpoint 字段）、`UpsertModelProviderEndpoint`、`DeleteModelProviderEndpoint`。
- `routing.sql`：`GetProvidersByEndpointAndModel` 的 `mpe.endpoint_id = $1` 改为 `mpe.endpoint_path = $1`（参数类型变为 text）。
- `provider.sql`：新增 `UpsertProvider`（INSERT ... ON CONFLICT (id) DO UPDATE）——改为在 handler 层分派 Create/Update，无需新增 SQL。

运行 `sqlc generate` 重新生成 `pkg/db/*.sql.go` 与 `querier.go`。

## 3. Contract 更新（`pkg/contract/`）

### provider.go

- 新增 `UpsertProviderRequest`（body 同 `CreateProviderRequest`，加可选 `ID int32`）、`UpsertProviderResponse`（body = `ProviderView`）。
- 新增 `DeleteProviderRequest`（body `{ID int32}`）。
- 新增 `OperationUpsertProvider`（PUT /providers）、`OperationDeleteProvider`（POST /providers/delete）。
- 保留现有 Create/Get/List。

### model.go

- 新增 `DeleteModelRequest`（body `{Name string}`）、`OperationDeleteModel`（POST /models/delete）。

### endpoint path 改名

- `provider_endpoint.go`：`ProviderEndpointView.EndpointID` → `EndpointPath string`；`FromProviderEndpointView`、`ToProviderEndpointView`、`DeleteProviderEndpointRequest` 同步。
- `model_provider_endpoint.go`：`ModelProviderEndpointView.EndpointID` → `EndpointPath`；`ListModelProviderEndpointsRequest.EndpointID` → `EndpointPath string`；`GetModelProviderEndpointRequest` 改为 query 字段 `EndpointPath`，`OperationGetModelProviderEndpoint.Path` 改为 `/model-provider-endpoints/get`；Upsert/Delete 同步。

## 4. Handlers

### `pkg/server/handle_providers.go`

- 新增 `handleUpsertProvider`：
  - 若 `Body.ID == 0` → 调用 `CreateProvider`。
  - 否则调用 `UpdateProvider`，所有 `SetXxx` 置 true。未找到返回 404 `PROVIDER_NOT_FOUND`。
  - 返回 `UpsertProviderResponse`。
- 新增 `handleDeleteProvider`：调用 `DeleteProvider`，返回 204。

### `pkg/server/handle_models.go`

- 新增 `handleDeleteModel`：调用 `DeleteModel(ctx, name)`。

### `pkg/server/handle_provider_endpoint.go` / `handle_model_provider_endpoint.go`

- 所有引用 `EndpointID` 改为 `EndpointPath`；路径参数解析适配。

### `pkg/server/server.go`

- `registerOperations` 注册三个新 operation：`OperationUpsertProvider`、`OperationDeleteProvider`、`OperationDeleteModel`。

## 5. 重新生成 OpenAPI 与前端类型

- 执行 `go run ./cmd/picotera openapi` 产出 `openapi.json`（或现有路径）。
- `pnpm --dir dashboard run generate:api`（或现有脚本）刷新 `dashboard/src/api.d.ts`。

## 6. 前端改动

### `dashboard/src/components/OverlayPanel.vue`

- 将 `<div v-if="visible" class="overlay-backdrop" @click.self="close">` 改为 `<div v-if="visible" class="overlay-backdrop">`（移除点击关闭）。

### `dashboard/src/components/ProviderForm.vue`

- 新增 `props.provider?: ProviderView`，`isEdit = !!props.provider`。
- 用 provider 值初始化 `form`。
- 标题改为 `isEdit ? '编辑渠道' : '新增渠道'`；提交按钮文案切换。
- 改为调用 `api.PUT('/api/picotera/providers', { body: { id: props.provider?.id, ...form } })`。

### `dashboard/src/views/ProvidersView.vue`

- 表格添加「操作」列，包含编辑与删除图标按钮（复用 EndpointsView 样式）。
- `openEdit(p)` → `overlay.open(ProviderForm, { provider: p, onSave: fetchProviders })`。
- `confirmDelete(p)` → `overlay.open(ConfirmDialog, { title: '删除渠道', message, onConfirm })` 调用 `POST /providers/delete`。

### `dashboard/src/views/ModelsView.vue`

- 表格添加「操作」列（编辑、删除）。
- `openEdit(m)` → `overlay.open(ModelForm, { model: m, onSave: fetchModels })`。
- 删除调用 `POST /models/delete`。

### `dashboard/src/components/MappingForm.vue`

- `form.endpointId` → `form.endpointPath`（string）。
- 新增 `endpoints = ref<EndpointView[]>([])`，`onMounted` 并行拉取 endpoints。
- 替换端点字段为 `<select v-model="form.endpointPath" :disabled="isEdit">`，option 显示 `{{ e.path }} — {{ e.name }}`。
- 提交 body 的 `endpointId` 改为 `endpointPath`。

### `dashboard/src/views/MappingsView.vue`

- 表头「端点」列显示 `m.endpointPath`。
- `confirmDeleteMapping` body 字段改为 `endpointPath`；列表 key、openEdit 参数同步。

## 7. 验证

- `go build ./...` 确保后端编译通过。
- `docker compose up -d` 启动数据库，`go run ./cmd/picotera` 确保迁移执行并启动。
- 手动 smoke：
  - 打开 Providers 视图 → 新增、编辑、删除各一次。
  - 打开 Models 视图 → 新增、编辑、删除各一次。
  - 打开 Mappings 视图 → 新建映射时端点下拉选择 path 成功、列表显示 path、删除成功。
  - 打开任一弹窗，点击背景遮罩，确认不关闭；点击 × 或取消可关闭。
