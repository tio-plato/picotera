# Plan

## 1. 后端：请求历史 API

1. **新增错误码** `pkg/errorx/errors.go`：`var RequestNotFound = ErrorCode("REQUEST_NOT_FOUND")`。
2. **新建查询文件** `db/queries/request.sql`：`ListRequests :many` 与 `GetRequest :one`，见 `design.md`。
3. **运行** `sqlc generate`，在 `pkg/db/` 下产出 `request.sql.go`、`ListRequestsParams`、`GetRequest` 等。
4. **新建 contract** `pkg/contract/request.go`：
   - `RequestView` 结构体（字段与 `design.md` 对齐；时间字段 `CreatedAt string json:"createdAt"`）。
   - `ToRequestView(*db.Request) *RequestView`（`pgtype.Timestamp` → `time.Time.UTC().Format(time.RFC3339Nano)`，其余 `pgtype.Text` / `pgtype.Int4` 按 `Valid` 判断是否省略）。
   - `ListRequestsRequest { PaginationRequest; ProviderID int32 query:"providerId"; EndpointPath string query:"endpointPath"; Model string query:"model" }`。
   - `ListRequestsResponse = PaginatedResponse[RequestView]`。
   - `GetRequestRequest { ID string path:"id" }`、`GetRequestResponse { Body RequestView }`。
   - `OperationListRequests` (`GET /requests`)、`OperationGetRequest` (`GET /requests/{id}`)。
5. **新建 handler** `pkg/server/handle_requests.go`：
   - `handleListRequests`：解码 cursor `(createdAt string, id string)` → `pgtype.Timestamp` / `pgtype.Text`；构建过滤参数；`fetchLimit = limit + 1`；转视图；`hasMore` 时对最后一条 `EncodeCursor("createdAt", last.CreatedAt, "id", last.ID)`。
   - `handleGetRequest`：`GetRequest`，`pgx.ErrNoRows` → `huma.Error404NotFound("request not found", errorx.RequestNotFound)`。
6. **注册 operations** 在 `pkg/server/server.go` 的 `registerOperations()` 尾部追加 `ListRequests` 与 `GetRequest`。
7. **重新生成 OpenAPI**：`mise run openapi`。

## 2. 前端：`AutoDataTable` 组件

1. **新建** `dashboard/src/ui/AutoDataTable.vue`：按 `design.md` 的 API 实现。内部复用 `DataTable`、`Th`、`Td`、`Tr`。
2. 默认取值工具 `get(row, path)`：纯函数，支持点分字符串；文件内私有，不需要单独抽出。
3. `defineSlots` / `defineProps` 使用泛型 `<Row>`。Vue 3.5 泛型语法 `<script setup lang="ts" generic="Row">` 已在项目中可用（检查 `tsconfig` 的 `vue` 版本），若出现编译问题退回 `Row = Record<string, unknown>` 并在调用方 `as any` 传入。
4. **barrel 导出** `dashboard/src/ui/index.ts` 追加 `AutoDataTable` 与 `AutoDataTableColumn` 类型。
5. **更新** `dashboard/DESIGN_SYSTEM.md` 的 Data display 小节，追加 `AutoDataTable` 用法与 `columns / items / rowKey / selected / onRowClick` 说明。

## 3. 前端：请求历史页面

1. **Regenerate types**：`pnpm --dir dashboard exec openapi-typescript openapi.yaml -o src/api.d.ts`（遵循项目现有流程；如果有 package.json 脚本优先走脚本）。
2. **路由** 在 `dashboard/src/router/index.ts` 新增 `/requests` → `RequestsView`。
3. **侧边栏** 在 `dashboard/src/components/AppSidebar.vue` 的 `nav` 追加 `{ name: 'requests', label: '请求', icon: 'activity' }`。
4. **图标** 在 `dashboard/src/ui/icons/paths.ts` 追加 `activity` → `IconActivity`（Tabler）。
5. **新建** `dashboard/src/views/RequestsView.vue`：
   - 挂载时并行：`api.GET('/api/picotera/providers')`、`/endpoints`、`/models`，结果缓存为 `providersMap`（`id → name`）等，供过滤器 `Select` 与单元格显示使用。
   - 状态：`requests`、`loading`、`hasMore`、`nextCursor`、`filters = { providerId, endpointPath, model }`。
   - `fetchRequests(cursor?)`：`GET /api/picotera/requests`，带 `limit: 30` 与 `filters` 非空字段。`cursor` 为空时清空 `requests.value`，否则 push。
   - `watch(filters, ...)` deep：触发 `fetchRequests(undefined)`。
   - 列使用 `AutoDataTable`，`#cell-*` 插槽自定义 status / tokens / createdAt 渲染。
   - 行点击：`panel.toggle(RequestDetailsPanel, { requestId: r.id }, { key: 'request:' + r.id })`。
   - `selected` 传 `(row) => panel.isActive('request:' + row.id)`。
   - 底部「加载更多」按钮：`Button variant="ghost" @click="fetchRequests(nextCursor)"`。
6. **新建** `dashboard/src/components/RequestDetailsPanel.vue`：
   - Props：`requestId: string`。
   - `onMounted` → `api.GET('/api/picotera/requests/{id}', { params: { path: { id: requestId }}})`。
   - 用 `SidePanel`，`title = '请求详情'`，`kicker = id` 短哈希。
   - 内容分组：基本信息、性能、Token、错误信息。布局用 `Field`（`as="div"`）包装每个只读字段。错误信息用 `<pre class="font-mono text-xs whitespace-pre-wrap bg-surface-50 border border-line-soft rounded-md p-3">`。

## 4. 验证

1. `sqlc generate` 通过，`pkg/db/request.sql.go` 生成。
2. `go build ./...` 通过。
3. 手动：`docker compose up -d` → `mise run server` → 发几条网关请求（或在 psql 插入 mock 数据）→ `mise run web` → 打开 `/requests`，验证：
   - 列表按时间倒序。
   - 过滤器三项独立生效，联合生效。
   - 「加载更多」不重复、不漏。
   - 点击行弹出详情面板，显示 tokens / 错误信息。
4. `pnpm --dir dashboard type-check` 通过。
5. `pnpm --dir dashboard lint` 通过。

## 5. 超出范围

以下项目当前不做，留作后续：

- 请求历史按时间区间过滤（日期选择器）。
- 按成功 / 失败 / 状态码分段的快速开关。
- 实时推送 / 自动刷新。
- 导出 CSV。
- 全文搜索 `error_message`。
