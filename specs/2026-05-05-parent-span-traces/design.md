# Design — Parent Span Traces

## 背景

`request.parent_span_id` 已经存在于数据库、后端 `RequestView` 和 dashboard 请求详情中。请求列表目前支持按类型、渠道、端点、模型和上游模型筛选，但不支持按 `parent_span_id` 筛选，也没有按 `parent_span_id` 聚合的控制台页面。

本功能新增一个“追踪”页面，按 `parent_span_id` 聚合同一追踪下的请求，并提供跳转到“请求”页面的精确筛选入口。

## 目标

- 新增管理 API，分页列出所有非空 `parent_span_id`。
- 每个追踪行返回请求数、token 总数、按币种聚合的模型成本、按币种聚合的上游成本和最后请求时间。
- 请求列表 API 新增 `parentSpanId` 精确筛选参数。
- dashboard 侧新增“追踪”导航页面，展示聚合列表和分页。
- 点击追踪行跳转到 `/requests?parentSpanId=<value>`，请求页面复用现有表格，只显示指定 `parent_span_id` 的请求。

## 后端设计

在 `db/queries/request.sql` 增加两个 sqlc 查询：

- `ListRequestTraces`：按 `parent_span_id` 聚合非空追踪，返回 `parent_span_id`、`COUNT(*)`、各 token 列合计、按币种聚合后的 `model_costs`、按币种聚合后的 `upstream_costs`、最近 `created_at`。按最近请求时间倒序、`parent_span_id` 倒序排序，用 keyset cursor 分页。
- 扩展 `ListRequests`：增加 `parent_span_id = sqlc.narg('parent_span_id')` 条件，仅在 query 参数存在时启用。

聚合 token 总数定义为：

```sql
COALESCE(input_tokens, 0)
+ COALESCE(cache_read_tokens, 0)
+ COALESCE(output_tokens, 0)
+ COALESCE(cache_write_tokens, 0)
```

成本总和分别计算 `model_cost` 与 `upstream_cost`，并按各自 currency 字段分组聚合。后端返回货币与价格数组，不把不同币种的金额相加。前端同时展示模型成本和上游成本，让用户直接比较面向模型定价的成本与实际上游成本。

成本数组元素格式为：

```json
{ "currency": "USD", "amount": 3.5 }
```

聚合 SQL 保证每个数组内同一 currency 只出现一次，并按 currency 升序输出，提供稳定展示顺序。

不新增数据库迁移。`parent_span_id` 已有列，聚合直接从现有 `request` 表读取。

## 合同设计

在 `pkg/contract/request.go` 中新增：

- `ParentSpanID string` 到 `ListRequestsRequest`，query 名为 `parentSpanId`。
- `RequestTraceView`，字段包含 `parentSpanId`、`requestCount`、`totalTokens`、`modelCosts`、`upstreamCosts`、`lastRequestAt`。
- `ListRequestTracesRequest` 复用 `PaginationRequest`。
- `ListRequestTracesResponse` 复用 `PaginatedResponse[RequestTraceView]`。
- `OperationListRequestTraces`，路径为 `GET /request-traces`。

`RequestTraceView` 不提供单个 cost/currency 字段。混合币种数据保持为数组，由前端根据用户当前货币偏好决定展示方式。

## 前端设计

新增 `dashboard/src/views/TracesView.vue`：

- 使用 `AutoDataTable`、`DataCard`、`Button`、`IconButton`、`MoneyDisplay` 等本地 primitive。
- 列包含最近时间、`parent_span_id`、请求数、token 总数、模型成本、上游成本。
- 成本展示遵循 dashboard 现有货币偏好：
  - `useCurrency().targetCurrency` 为 `null` 时，直接展示数组中的每个币种，例如 `$3.50 + ¥52.23`。
  - `useCurrency().targetCurrency` 有值时，前端使用 `useCurrency().convert` 把数组中每个金额换算到目标货币，再加总后展示一个目标货币金额。
- 点击行跳转到 `{ name: 'requests', query: { parentSpanId: row.parentSpanId } }`。
- 使用现有分页模式：`limit=30`、`cursor`、`hasMore`、`nextCursor`、加载更多。

修改 `dashboard/src/router/index.ts` 和 `dashboard/src/components/AppSidebar.vue`：

- 新增 `/traces` 路由，name 为 `traces`。
- 在侧边栏加入“追踪”入口。

修改 `dashboard/src/views/RequestsView.vue`：

- 从 route query 读取 `parentSpanId`，写入现有 `filters`。
- 调用请求 API 时携带 `parentSpanId`。
- 当存在 `parentSpanId` 时，在页面顶部展示一个紧凑筛选状态，可清除并回到 `/requests`。
- 将请求页筛选状态同步到 URL query，保证刷新页面后筛选仍然生效。

## OpenAPI 工作流

因为本功能新增 API 和 query 参数，完成后必须运行：

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

dashboard 只使用生成后的 OpenAPI 类型和现有 `openapi-fetch` 客户端。

## 非目标

- 不新增数据库列或迁移。
- 不实现模糊搜索、大小写不敏感匹配或输入纠错。
- 不为 `parent_span_id` 做兼容别名；API query 参数只接受 `parentSpanId`。
- 不在“追踪”页面展开请求详情；详情仍在跳转后的“请求”页面中查看。
- 不引入第三方 UI 库或聚合图表库。
