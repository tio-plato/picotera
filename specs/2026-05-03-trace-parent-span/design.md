# Design — Trace Parent Span

## 背景

`request` 表的 `parent_span_id` 列（`db/migrations/001_initial.sql`）从设计之初就存在，View 与 dashboard 也已经能展示它（`pkg/contract/request.go`、`dashboard/src/components/RequestDetailsPanel.vue:242`），但写入路径从未填充——`pkg/server/handle_gateway.go` 在插入 meta 与 upstream 行时都把 `ParentSpanID` 设为 `Valid: false`。

外部客户端（Claude Code、其他对话型 SDK）通常会带一个会话标识到上游：

- `X-Claude-Code-Session-Id`：Claude Code CLI 注入的会话 ID。
- `conversation_id`：字面下划线写法，由部分会话 SDK 直接以非规范 header 形式发送。

把这两个头中的任意一个落到 `parent_span_id` 上，就能在 dashboard 里把同一会话内的多次请求串起来，便于排查、分析。

## 目标

- 进入网关的每条请求，从 header 中提取会话标识，作为 meta 行的 `parent_span_id` 写入。
- 同 meta 派生的 upstream 行继承同一个 `parent_span_id`。
- 不引入新的列、API 或迁移；不改动 dashboard。

## 整体方案

1. 在 `pkg/server/gateway_helpers.go` 增加纯函数 `extractParentSpanID(http.Header) string`：
   - 优先返回 `Header.Get("X-Claude-Code-Session-Id")` 去除首尾空白后的值。
   - 否则遍历 `http.Header` map（key 不走规范化，因 `conversation_id` 含下划线），按 `strings.EqualFold` 匹配 `conversation_id`，取第一项非空值。
   - 都不存在则返回空字符串。
2. 在 `handle_gateway.go` 的 `ServeHTTP` 入口，紧接读取 `metaReqHeader` 之后调用一次 `extractParentSpanID`，把结果存入局部变量 `parentSpanID`。
3. 把 `pgtype.Text{String: parentSpanID, Valid: parentSpanID != ""}` 同时传给：
   - meta 行的 `InsertRequestParams.ParentSpanID`（约 `handle_gateway.go:65`）。
   - 重试循环中每条 upstream 行的 `InsertRequestParams.ParentSpanID`（约 `handle_gateway.go:335`）。
4. 不改 sqlc 查询、Querier、合同、TS 类型——`InsertRequest` 早就接收 `ParentSpanID`，View 已经导出 `parentSpanId`。

## 关键决策

- **header 名直接 hardcode**：只识别这两个名字，避免引入「可配置 header 列表」过早抽象。
- **优先级 X-Claude-Code-Session-Id 在前**：更具体，命中即用；通用 `conversation_id` 作 fallback。
- **upstream 继承 meta 的 parent**：保持「同一外部 trace 下所有行共享同一 parent_span_id」的语义，dashboard 按 `parent_span_id` 聚合时不必再回查 meta。upstream 与 meta 自身的父子关系由 `span_id` 表达（upstream.span_id = meta.id），这一层不变。
- **下划线 header 用 map 遍历**：Go 的 `Header.Get` 会做 MIME 规范化，下划线 key 走不到。绕过规范化是必须的。

## 兼容性

- 旧请求行 `parent_span_id` 仍为 NULL，不会影响新逻辑。
- 不带相关 header 的请求继续写 NULL，行为与今天一致。
- 不修改任何对外 API 形状；dashboard 已有的渲染条件 (`v-if="selected.parentSpanId"`) 自动覆盖新数据。
