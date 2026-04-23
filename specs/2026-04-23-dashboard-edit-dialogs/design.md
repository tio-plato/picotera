# Design: Dashboard 编辑对话框与映射端点字段修正

## Overview

本次变更分三部分：

1. 补齐 Provider（渠道）和 Model（模型）的 CRUD 后端接口，并在前端列表行提供编辑/删除入口，复用现有表单组件以支持编辑模式。
2. 修正数据模型：`provider_endpoint` 和 `model_provider_endpoint` 表中的 `endpoint_id INTEGER` 字段改为 `endpoint_path TEXT`，引用 `endpoint(path)`。API 字段同步从 `endpointId` 改名为 `endpointPath`，前端映射表单用下拉框从 endpoints 列表选择。
3. 修复 OverlayPanel 交互：禁止点击背景关闭弹窗（保留关闭按钮和取消按钮）。

## Database Schema Change

当前 `endpoint` 表以 `path TEXT` 为主键，而 `provider_endpoint.endpoint_id` 和 `model_provider_endpoint.endpoint_id` 却是 `INTEGER`，类型不一致，实际无法建立正确的外键关联。

项目尚在早期、数据可清空，直接修改 `db/migrations/001_initial.sql`：

- `provider_endpoint.endpoint_id INTEGER` → `endpoint_path TEXT NOT NULL`，主键改为 `(provider_id, endpoint_path)`。
- `model_provider_endpoint.endpoint_id INTEGER` → `endpoint_path TEXT NOT NULL`，主键改为 `(model_name, provider_id, endpoint_path)`。
- `request.endpoint_id INTEGER` → `endpoint_path TEXT NOT NULL`。

开发者需重建数据库（或 `docker compose down -v && up -d`）以重新应用。

## Backend API Additions

### Provider

- 新增 `PUT /api/picotera/providers`（UpsertProvider）：body 含可选 `id`。无 id 时走 `CreateProvider`；有 id 时走 `UpdateProvider`（所有 `set_*` 字段置 true）。
- 新增 `POST /api/picotera/providers/delete`：按 id 删除。
- 保留 `POST /providers`（Create）以免破坏现有前端，但本次会把前端统一切到 `PUT`。

### Model

- 新增 `POST /api/picotera/models/delete`：按 name 删除。`UpsertModel` 与 `PUT /models` 已存在。

### Endpoint Path 字段改名

所有 API 契约字段 `endpointId: int32` → `endpointPath: string`。影响：

- `ProviderEndpointView`
- `ModelProviderEndpointView`
- `ListModelProviderEndpointsRequest.EndpointID` 查询参数 → `EndpointPath`
- `GetModelProviderEndpoint` 路径参数 `{endpointId}` → `{endpointPath}`
- `DeleteProviderEndpointRequest` / `DeleteModelProviderEndpointRequest` body 字段
- DB 层 `ProviderEndpoint.EndpointID`、`ModelProviderEndpoint.EndpointID` → `EndpointPath`（sqlc 重新生成）

Huma 路径参数含 `/` 不安全，所以 `GetModelProviderEndpoint` 改为 query-style：`GET /model-provider-endpoints/get?modelName=...&providerId=...&endpointPath=...`。

## Frontend Changes

### 共享

- `OverlayPanel.vue`：移除 `@click.self="close"`，仅保留表单内 `×` 和取消按钮关闭。遮罩不再响应点击。
- `ProviderForm.vue` 改造为 create/edit 双用：props 接 `provider?: ProviderView`；有 provider 时标题为「编辑渠道」、`name` 字段禁用（与其他表单一致使用 name 作为显示 key，可编辑）、提交走 `PUT /providers`。
- `ModelForm.vue` 已经支持 edit，继续复用。
- 删除按钮的确认对话框沿用 `ConfirmDialog.vue`。

### 视图

- `ProvidersView.vue`：表格新增「操作」列，含编辑、删除按钮。
- `ModelsView.vue`：表格新增「操作」列，含编辑、删除按钮。
- `MappingsView.vue`：`endpointId` → `endpointPath`，显示 path 字符串。
- `MappingForm.vue`：加载 endpoints 列表，用 `<select>` 绑定 `form.endpointPath`，选项展示 `{path} — {name}`；新建模式可选，编辑模式禁用（主键）。

### 类型

前端 `@/api` 的类型由 OpenAPI 重新生成。OpenAPI spec 由 `go run ./cmd/picotera openapi` 产出，计划脚本执行一次并提交。

## Out of Scope

- 删除功能不做级联检查（如删除渠道时连带删除其映射）；如存在外键冲突由数据库报错，前端展示错误即可。
- 不修改 `request` 表的现有写入逻辑（本次尚无写入方）。
