# Plan — Request Detail Spans

## Task 1 — 后端 span 列表接口

**目标**：新增按 `span_id` 列举请求的查询、contract 与 handler。

**改动**

- `db/queries/request.sql`：新增 `ListRequestsBySpan` 查询（见 `api.md`）。
- `sqlc generate` 重新生成 `pkg/db/`。
- `pkg/contract/request.go`：
  - 新增 `ListRequestSpansRequest{ ID string \`path:"id"\` }`。
  - 新增 `ListRequestSpansResponse struct { Body []RequestView }`。
  - 新增 `OperationListRequestSpans`（`GET /requests/{id}/spans`，operationID `listRequestSpans`）。
- `pkg/server/handle_requests.go`：实现 `handleListRequestSpans`，复用 `ToRequestView`；空集时返回 `404 RequestNotFound`。
- `pkg/server/server.go`（或注册位置）：在 `registerOperations` 中注册新 operation。

**验证**

- `sqlc generate` 通过。
- `go build ./...` 通过。
- 启动后 `curl /api/picotera/requests/{已知-meta-id}/spans` 返回 meta + 关联 upstream。

## Task 2 — 重新生成 OpenAPI 与前端类型

**改动**

- 运行 `mise run openapi` 更新 `openapi.yaml`。
- 运行 `pnpm --dir dashboard type-check` 触发 openapi-typescript 重新生成 `dashboard/src/api.d.ts`（按现有流水线；若需要单独命令，参考 `dashboard/package.json`）。

**验证**

- `openapi.yaml` 出现 `/requests/{id}/spans` 路径。
- `dashboard/src/api.d.ts` 中包含 `listRequestSpans` 路径与类型。

## Task 3 — 列表默认筛选 meta

**改动**

- `dashboard/src/views/RequestsView.vue`：
  - `filters` 增加 `type: 'meta' | 'upstream' | 'all'`，默认 `'meta'`。
  - 顶部筛选区使用 `SegmentedControl` 或 `Select`，与现有筛选风格一致；标签：「全部 / 元请求 / 上游请求」，参考已用样式优先 `SegmentedControl`（更直观）。
  - `fetchRequests` 中：`'meta'` 传 `type: 0`、`'upstream'` 传 `type: 1`、`'all'` 不传。
  - 新增 type 列（在状态前）展示 `META` / `UP`徽章，便于「全部」视图下区分。

**验证**

- 默认进入页面只显示 meta。
- 切换到上游/全部时列表更新。

## Task 4 — 详情面板：span 卡片轨道

**改动**

- `dashboard/src/components/RequestDetailsPanel.vue`：
  - 删除原 `request` 单条状态，改为：
    - `spans = ref<RequestView[]>([])`
    - `selectedId = ref<string>('')`
    - `selected = computed(() => spans.value.find(s => s.id === selectedId.value))`
  - 数据流：`fetchSpans()` 调用 `GET /requests/{id}/spans`，把结果排序保证 meta（`r.id === r.spanId`）位列首位，其余 upstream 按 `createdAt` 升序；默认选中第一项（即 meta）。
  - 模板顶部新增 `SpanCardTrack` 子片段：
    - 包裹：`<div class="flex gap-2 overflow-x-auto -mx-1 px-1 pb-1">`。
    - 单卡：`<button>` 渲染，`min-w-44 max-w-56 p-2.5 rounded-md border` + 选中时 `border-accent` / `bg-accent-faint`，未选中 `border-line` / `hover:bg-surface-50`。
    - 卡片内容：
      - 顶部行：左侧角标（META / `#n`），右侧 `Tag` 显示状态码（沿用 `statusVariant`）。
      - 中间行：provider 名（`providerLabel(s.providerId)`，meta 阶段为空时显示 `—`）。
      - 底部行：耗时（`formatTimeSpent(s.timeSpentMs)`）。
  - 详情主体保留现有 sections，但全部改为读 `selected.value`；如未加载或为空，显示 `StateText`。
  - 「基本信息」section 增加：`Type`（`META` / `UPSTREAM`）和 `Status`（pending/header/completed/failed 文本映射）。
  - 取消 `loading`/`error` 单行视图；保留全局 `loading`，错误展示在 `SidePanel` 的 `#error` slot。
- 需要 provider id → name 映射：在 `RequestsView` 拥有 providers 列表，最简单做法是把它注入 panel：
  - 方案：在 `RequestsView.openDetails` 调 `panel.toggle(RequestDetailsPanel, { requestId: r.id, providers: providers.value }, ...)`；面板将 providers 转成 Map。
  - 若 provider 缺失，回退显示 `#${id}`。

**验证**

- 打开任意 meta 请求：顶部至少出现 1 张 META 卡 + N 张 upstream 卡。
- 点击不同卡片，下方详情切换。
- 数据库中 upstream 数为 0 时（meta pending / 解析失败），仅显示 META 卡，无错误。

## Task 5 — 端到端冒烟测试

**步骤**

1. `docker compose up -d && mise run server`
2. 触发若干请求（包括失败重试场景）写入数据库。
3. `mise run web`，访问请求页：
   - 默认列表只见 meta 行。
   - 切换「上游」「全部」筛选，列表正确变化。
   - 点击一条 meta：卡片轨道显示 META + 各 upstream；点击切换详情。
   - 失败重试场景下，可看到多张 upstream 卡，状态码不同。
4. 直接打开一条 upstream（在「全部」/「上游」筛选下）：当前实现按 `requestId` 调 `/requests/{id}/spans` —— 后端按 `span_id` 查询，能返回该 upstream 所属 meta 的全套 span（因为传入的是 upstream id 不是 meta id 时，匹配为空）。

**注意**：上游请求作为列表行被点开时，需要能拿到所属 meta 的全部 span。后端查询条件 `span_id = $1` 应改为 `span_id = (SELECT span_id FROM request WHERE id = $1)` 以兼容；或前端在列表中只对 type=meta 行启用点击。**采用前者（后端兼容）**：在 Task 1 的 sqlc 查询中改为：

```sql
-- name: ListRequestsBySpan :many
WITH anchor AS (
  SELECT span_id FROM request WHERE id = $1
)
SELECT r.id, r.span_id, r.parent_span_id, r.type, r.status, r.provider_id, r.endpoint_path,
       r.api_key_id, r.model, r.input_tokens, r.cache_read_tokens, r.output_tokens,
       r.cache_write_tokens, r.status_code, r.error_message, r.ttft_ms, r.time_spent_ms,
       r.created_at
FROM request r, anchor
WHERE r.span_id = anchor.span_id
ORDER BY r.created_at ASC, r.id ASC;
```

这样无论传入 meta id 还是 upstream id，都返回同一个 span 组。

## Task 6 — 提交

- 后端改动：`feat(request): list spans by id endpoint`
- 前端改动：`feat(dashboard): request detail span switcher`
