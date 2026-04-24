# Design

## Scope

两块工作：

1. **请求历史查看**：新增 `/requests` 管理 API 与 `RequestsView` 页面，支持按 `provider_id` / `endpoint_path` / `model` 过滤，使用游标分页向后翻页。点击一行在 `SidePanel` 中打开详情。
2. **`AutoDataTable` 原子组件**：新建 `src/ui/AutoDataTable.vue`，按 `columns + items` 自动渲染表头与行；现有 `DataTable` / `Tr` / `Th` / `Td` 原子组件保持不动，以兼容旧页面。

两块工作在同一个提交周期内完成：请求历史页面直接使用 `AutoDataTable`，作为新组件的首个使用者。

## 请求历史后端

### 数据源

复用 `request` 表（见 `db/migrations/001_initial.sql`）。当前字段已能承载列表所需信息：`id`, `provider_id`, `endpoint_path`, `model`, `status_code`, `error_message`, `input_tokens`, `cache_read_tokens`, `output_tokens`, `cache_write_tokens`, `ttft_ms`, `time_spent_ms`, `created_at`。

列表按 `created_at DESC, id DESC` 排序，最新请求在前。游标编码 `(createdAt, id)` 元组以保证稳定翻页。

> 注意：`request.api_key_id` 在 `002_make_api_key_id_nullable.sql` 后允许 NULL，但当前 `logRequest` 并不写入 api key。视图先不暴露这个字段。

### 查询

新增 `db/queries/request.sql`：

```sql
-- name: ListRequests :many
SELECT id, span_id, parent_span_id, provider_id, endpoint_path, api_key_id, model,
       input_tokens, cache_read_tokens, output_tokens, cache_write_tokens,
       status_code, error_message, ttft_ms, time_spent_ms, created_at
FROM request
WHERE
  (sqlc.narg('provider_id')::int IS NULL OR provider_id = sqlc.narg('provider_id'))
  AND (sqlc.narg('endpoint_path')::text IS NULL OR endpoint_path = sqlc.narg('endpoint_path'))
  AND (sqlc.narg('model')::text IS NULL OR model = sqlc.narg('model'))
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::text)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.narg('limit')::int;

-- name: GetRequest :one
SELECT * FROM request WHERE id = $1;
```

游标方向是 `<`（而非 `>`），因为主排序是 `DESC`。

### Contract / handler

- `pkg/contract/request.go`：`RequestView`、`ListRequestsRequest`（包含 `PaginationRequest` + `providerId` / `endpointPath` / `model` 过滤项）、`ListRequestsResponse = PaginatedResponse[RequestView]`、`GetRequestResponse`。
- `pkg/server/handle_requests.go`：`handleListRequests` / `handleGetRequest`。复用 `contract.EncodeCursor` / `DecodeCursor` 处理 `(createdAt, id)`。
- 在 `pkg/server/server.go` 的 `registerOperations()` 中注册两条 operation。
- 重新生成 `openapi.yaml`。

### Time 编码

`created_at` 字段在 pgx 中为 `pgtype.Timestamp`。在 Go 侧转 `time.Time.UTC()`，JSON 输出 RFC3339 字符串。游标里用 RFC3339Nano 字符串，避免 JSON number 精度丢失。

## 请求历史前端

### 路由与入口

- `src/router/index.ts` 新增 `{ path: '/requests', name: 'requests', component: () => import('@/views/RequestsView.vue') }`。
- `src/components/AppSidebar.vue` 在 `nav` 数组新增 `{ name: 'requests', label: '请求', icon: 'activity' }`（`activity` 图标需要在 `src/ui/icons/paths.ts` 里注册）。

### 页面结构

`src/views/RequestsView.vue`：

- 顶部工具条：左侧是过滤器区（三个 `Select`：渠道、端点、模型；值变化触发重新拉取并清空列表与游标）；右侧是计数文本。
- 过滤器数据来源：页面挂载时并行拉取 `/api/picotera/providers` / `/api/picotera/endpoints` / `/api/picotera/models`，填充 Select 选项。保留现有页面里「所有」= 空值的约定。
- 列表：`DataCard` 包 `AutoDataTable`，列定义见下。
- 分页：底部 `Button variant="ghost"` 「加载更多」，只有在 `hasMore` 为真时显示。
- 空态：`StateText`。
- 自动轮询不做，由用户手动刷新（新增一个刷新 `IconButton` 触发 `fetchRequests(undefined)`）。

### 详情侧边栏

- `src/components/RequestDetailsPanel.vue`：通过 `useSidePanel` 打开，`key = 'request:<id>'`。
- 接收 `requestId: string`，挂载时 `GET /api/picotera/requests/{id}` 拉取完整 `RequestView`。
- 展示：基本信息（id、span_id、parent_span_id、provider、endpoint、model、status、created_at）、性能指标（ttft_ms、time_spent_ms）、token 明细（input/output/cache_read/cache_write）、完整 `error_message`（预格式化，`<pre class="font-mono text-xs whitespace-pre-wrap">`）。
- 列表行点击整行打开详情，同时 `Tr` 用 `selected` 状态标记当前激活行。

### 列定义

| key | 表头 | 渲染 |
| --- | --- | --- |
| `createdAt` | 时间 | 本地时间 `HH:mm:ss`，下方灰色日期 |
| `providerId` | 渠道 | 映射成 provider 名称（通过已拉取的 providers map）；未知时展示 `#<id>` |
| `endpointPath` | 端点 | `font-mono text-ink-faint` |
| `model` | 模型 | `font-mono`；为空展示 `—` |
| `status` | 状态 | 复合：`StatusBadge`（2xx=ok，4xx=warn，5xx / 0=err） + 数字 |
| `tokens` | Token | `in/out` 紧凑显示，cache 用小 Tag 表示 |
| `timeSpentMs` | 耗时 | `Xms` 或 `X.Xs`，tabular-nums |

## `AutoDataTable` 组件

### API

```ts
interface AutoDataTableColumn<Row> {
  key: string                        // 用于 cell slot 名字与 :key
  header?: string                    // 表头文本；缺省为空
  field?: keyof Row | string         // 默认取值路径；点分字符串支持浅层嵌套
  actions?: boolean                  // 透传给 Th/Td 的 actions 属性
  align?: 'left' | 'right'           // 可选，默认 left
  headerClass?: string
  cellClass?: string
}

defineProps<{
  columns: AutoDataTableColumn<Row>[]
  items: Row[]
  rowKey: (row: Row, index: number) => string | number
  selected?: (row: Row) => boolean    // 返回 true 的行加 selected 样式
  hoverable?: boolean                  // 默认 true
  onRowClick?: (row: Row, event: MouseEvent) => void
}>()
```

### 插槽

- **`#header-<key>`**（可选）：覆盖某列表头，默认渲染 `column.header`。
- **`#cell-<key>`**（可选）：覆盖某列单元格；作用域 `{ row, value, index }`。
- **`#empty`**（可选）：没有数据时的内容。默认不渲染 `<tbody>` 里的空白——空态由调用方用 `StateText` 自行处理（沿用现有页面惯例）。

### 内部实现

使用现有的 `DataTable` + `Th` + `Td` + `Tr`，避免重复样式：

```html
<DataTable>
  <thead>
    <tr>
      <Th v-for="col in columns" :actions="col.actions">
        <slot :name="`header-${col.key}`">{{ col.header }}</slot>
      </Th>
    </tr>
  </thead>
  <tbody>
    <Tr v-for="(row, i) in items" :selected="selected?.(row)" @click="onRowClick?.(row, $event)">
      <Td v-for="col in columns" :actions="col.actions">
        <slot :name="`cell-${col.key}`" :row="row" :value="get(row, col.field)" :index="i">
          {{ defaultFormat(get(row, col.field)) }}
        </slot>
      </Td>
    </Tr>
  </tbody>
</DataTable>
```

- `get(row, path)`：支持点分路径；`undefined` / `null` 返回 `''`；字符串、数字直接 `String()`。
- `defaultFormat`：`null | undefined | ''` 显示空字符串（不是「—」，保持中性）。
- 当 `onRowClick` 存在时，`Tr` 加 `cursor-pointer`；阻止 actions 列内部 `click` 冒泡由调用方在 cell slot 里 `.stop` 负责。

### 导出

`src/ui/index.ts` 新增：

```ts
export { default as AutoDataTable } from './AutoDataTable.vue'
export type { AutoDataTableColumn } from './AutoDataTable.vue'
```

现有 `DataTable` / `Tr` / `Th` / `Td` 保持导出。`DESIGN_SYSTEM.md` 的「Data display」小节追加 `AutoDataTable` 条目。

## 第三方依赖

无。全部复用项目现有库（Huma、sqlc、pgx、Vue、Tailwind、openapi-fetch、Tabler icons）。
