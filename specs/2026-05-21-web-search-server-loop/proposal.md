# Server-Side Web Search Loop

## 原始需求

观察现在的 web 搜索模拟功能（spec `2026-05-21-web-search-emulation`）。当前实现只跑一轮上游请求：Exa 结果注入后用 `pause_turn` 终止流，由客户端拼接历史重新请求来驱动循环。

但事实上，几乎没有客户端真正支持 Anthropic 的 `pause_turn` 协议（Claude Code 之外的大部分实现都把它当成终态丢出）。

把这个行为扩展一下：当 Exa 返回结果之后，**网关在本地继续往自己（`/api/picotera/v1/messages`）发送新的 LLM 请求**，请求体里包含新生成的 `server_tool_use` + `web_search_tool_result`，把新一轮响应的 content block 续接进同一次客户端响应里，直到上游返回的 stop_reason **不再是** `tool_use`（即模型不再调用 web_search，自然终止：`end_turn`、`max_tokens`、非 web_search 的 `tool_use` 等）。

## 关键决策（已确认）

- **最大轮数**：硬编码 10 轮上游 LLM 调用，无配置项。超过即降级为旧的 `pause_turn` 行为（把当前响应原样返回客户端兜底）。
- **`pause_turn` 兼容**：不保留；正常情况下客户端永远看不到 `pause_turn`，loop 完全跑在网关侧。
- **子请求实现**：HTTP self-call。在网关进程内用 `httptest.NewRequest` 构造 `POST /api/picotera/v1/messages`，通过 `s.router.ServeHTTP` 走完整链路。每一轮自循环都产生一组独立的 meta + upstream `request` 行，trace 视图能完整呈现展开过程。
- **`usage` 聚合**：
  - 返回给客户端的最终 `usage`（非流式 JSON 的 `message.usage`，流式的 `message_start.usage` + `message_delta.delta.usage`）是各轮 usage 字段的累加。
  - 写入数据库的每一轮 `request` 行的 `usage_*` 列只记录该轮自己的 usage（也就是说，最外层 meta request 的 usage 仍然只反映第一轮）。
  - 这样上层基于 `request` 表做汇总统计时，把每轮独立行相加自然得到与客户端看到一致的总量，不需要修改任何汇总逻辑。

## 不在范围内

- 不引入新的配置项、新的 provider 字段、新的数据库列。所有变更纯粹是 handler 层逻辑。
- 不修改非 Anthropic Messages 路由的行为；本扩展只作用于 `POST /api/picotera/v1/messages`。
- 不修改 `pkg/server/web_search.go` 里 outbound 改写、Exa 调用、`web_search_tool_result` 构造、SSE 状态机本身的工具替换逻辑；只在它之外加一层「续写循环」。
- 不修改 `endpoint` 表里的 exaSearch endpoint 配置流程。
- 不重新计费/扣额度；自循环产生的额外 upstream 调用按现有规则各自计入 `request` 表。
