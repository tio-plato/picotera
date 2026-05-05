# Plan — Parent Span Traces

## 1. 后端查询

文件：`db/queries/request.sql`

- 在 `ListRequests` 的 `WHERE` 条件中加入 `parent_span_id` 精确筛选：

  ```sql
  AND (sqlc.narg('parent_span_id')::text IS NULL OR parent_span_id = sqlc.narg('parent_span_id'))
  ```

- 新增 `ListRequestTraces :many`：
  - 只聚合 `parent_span_id IS NOT NULL` 且 `parent_span_id <> ''` 的请求。
  - 返回字段：`parent_span_id`、`request_count`、`total_tokens`、`model_costs`、`upstream_costs`、`last_request_at`。
  - `model_costs` 使用 `jsonb_agg` 返回按 `model_cost_currency` 聚合的 `{currency, amount}` 数组，排除 cost 或 currency 为 NULL 的行。
  - `upstream_costs` 使用 `jsonb_agg` 返回按 `upstream_cost_currency` 聚合的 `{currency, amount}` 数组，排除 cost 或 currency 为 NULL 的行。
  - 使用 CTE 先 group，再对聚合结果做 cursor 条件和排序。
  - 排序：`last_request_at DESC, parent_span_id DESC`。
  - 分页：`LIMIT sqlc.narg('limit')::int`。

运行 `sqlc generate` 更新 `pkg/db/` 生成代码。

## 2. 后端合同

文件：`pkg/contract/request.go`

- 给 `ListRequestsRequest` 增加：

  ```go
  ParentSpanID string `query:"parentSpanId,omitempty"`
  ```

- 新增 `RequestTraceView`：

  ```go
  type TraceCostView struct {
      Currency string  `json:"currency"`
      Amount   float64 `json:"amount"`
  }

  type RequestTraceView struct {
      ParentSpanID  string          `json:"parentSpanId"`
      RequestCount  int64           `json:"requestCount"`
      TotalTokens   int64           `json:"totalTokens"`
      ModelCosts    []TraceCostView `json:"modelCosts"`
      UpstreamCosts []TraceCostView `json:"upstreamCosts"`
      LastRequestAt string          `json:"lastRequestAt,omitempty"`
  }
  ```

- 新增 `ToRequestTraceView(*db.ListRequestTracesRow) *RequestTraceView`。
- 新增解析 `jsonb_agg` 成 `[]TraceCostView` 的 helper。解析失败按 500 错误处理，不静默丢弃成本数据。
- 新增 `ListRequestTracesRequest`、`ListRequestTracesResponse` 和 `OperationListRequestTraces`。

## 3. 后端 handler

文件：`pkg/server/handle_requests.go`

- 在 `handleListRequests` 中把 `input.ParentSpanID` 转为 `pgtype.Text`，传给 `db.ListRequestsParams.ParentSpanID`。
- 新增 `handleListRequestTraces`：
  - 按现有分页 handler 模式处理 `limit`、`fetchLimit`、`cursor`。
  - cursor 解码字段为 `lastRequestAt` 和 `parentSpanId`。
  - 调用 `s.queries.ListRequestTraces`。
  - 多取一条判断 `hasMore`。
  - 用最后一行编码 `nextCursor`。
  - 返回 `PaginatedBody[RequestTraceView]`。

文件：`pkg/server/server.go`

- 在请求相关 operation 旁注册 `OperationListRequestTraces`。

## 4. OpenAPI 与 dashboard 类型

运行：

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

提交更新后的：

- `openapi.yaml`
- `dashboard/src/openapi-types.d.ts`

## 5. 前端路由和导航

文件：`dashboard/src/router/index.ts`

- 新增：

  ```ts
  { path: '/traces', name: 'traces', component: () => import('@/views/TracesView.vue') }
  ```

文件：`dashboard/src/components/AppSidebar.vue`

- 在“请求”附近加入：

  ```ts
  { name: 'traces', label: '追踪', icon: 'route' }
  ```

如果 `route` 图标不存在，先在 `dashboard/src/ui/icons/paths.ts` 注册一个合适的 Tabler 图标。

## 6. 新增追踪页面

文件：`dashboard/src/views/TracesView.vue`

- 创建页面级组件。
- 状态：
  - `traces: RequestTraceView[]`
  - `loading`
  - `hasMore`
  - `nextCursor`
- `fetchTraces(cursor?: string)` 调用 `GET /api/picotera/request-traces`，`limit=30`。
- 页面挂载时调用 `useExchangeRatesStore().fetch()`，保证特定货币展示有汇率数据。
- 表格列：
  - `lastRequestAt`：最近时间，格式复用请求页的时间展示函数。
  - `parentSpanId`：monospace，显示完整值或在窄布局中用 CSS 截断。
  - `requestCount`：请求数，tabular。
  - `totalTokens`：token 合计，tabular。
  - `modelCosts`：展示模型成本数组。
  - `upstreamCosts`：展示上游成本数组。
- 新增本页局部格式化函数：
  - 当 `useCurrency().targetCurrency` 为 `null` 时，逐项调用 `useCurrency().format(amount, currency)` 并用 ` + ` 连接。
  - 当 `useCurrency().targetCurrency` 有值时，逐项调用 `useCurrency().convert(amount, currency)`，只把成功换算到目标货币的结果相加。
  - 如果任一条成本因为缺少汇率无法换算到目标货币，展示原生货币表达式，不展示部分换算后的错误总和。
  - 空数组显示 `—`。
- 行点击：

  ```ts
  router.push({ name: 'requests', query: { parentSpanId: row.parentSpanId } })
  ```

- 底部使用现有“加载更多”按钮模式。

## 7. 请求页面 URL 筛选

文件：`dashboard/src/views/RequestsView.vue`

- 引入 `useRoute`、`useRouter`。
- 在 `filters` 中新增：

  ```ts
  parentSpanId: '',
  ```

- 初始化时从 `route.query.parentSpanId` 读取。只接受单个 string；数组或非 string 视为无效并忽略。
- `fetchRequests` 中携带 `parentSpanId`。
- watcher 覆盖 `filters.parentSpanId`，变化时重新加载。
- 增加 URL query 同步：
  - `parentSpanId` 有值时写入 query。
  - 清除时移除 query。
  - 使用 `router.replace`，不污染浏览器历史。
- 页面顶部在存在 parent span 筛选时展示当前追踪 ID 和清除按钮；点击清除设置 `filters.parentSpanId = ''`。
- `activeFilterCount` 和 `clearAllFilters` 包含 `parentSpanId`。

## 8. 验证

运行：

```bash
go build ./...
pnpm --dir dashboard type-check
pnpm --dir dashboard build
```

手动验证：

- `GET /api/picotera/request-traces` 返回非空 `parentSpanId` 聚合和分页字段。
- `/traces` 表格可加载、刷新、加载更多。
- 点击追踪行进入 `/requests?parentSpanId=<id>`。
- 刷新该 URL 后仍然只显示对应 `parentSpanId` 的请求。
- 清除筛选后 URL 回到 `/requests`，请求列表恢复未按追踪过滤。
