# Web Search Emulation via Exa

## 原始需求

Anthropic 的 web search tool（`web_search_20250305` / `web_search_20260209`）是服务端执行的。部分上游供应商不支持原生 web search，需要在网关层用 Exa 搜索 API 模拟出与 Anthropic 原生 web search 完全一致的客户端体验。

## 核心流程

仅对 `/api/picotera/v1/messages`（Anthropic Messages 统一路由）生效。

### 触发条件

1. 请求中的 tools 数组包含 `web_search_20250305` 或 `web_search_20260209` 类型的工具。
2. 所选上游供应商的 `supports_native_web_search` 字段为 `false`。

### Outbound 改写（请求 → 上游）

1. **工具替换**：将 `web_search_20250305` / `web_search_20260209` 服务端工具替换为普通 function tool，input_schema 暴露 Exa 支持的多个参数（`query` 必填，`numResults`、`category`、`includeDomains`、`excludeDomains`、`startPublishedDate`、`endPublishedDate` 可选）。LLM 决定要调用什么参数。
2. **历史消息转换**：如果 messages 中已有前几轮的 web search 结果（`server_tool_use` + `web_search_tool_result` 块），需要转换为上游能理解的 function tool 格式：
   - `server_tool_use` 块 → `tool_use` 块（ID 前缀 `srvtoolu_` → `toolu_`，input 原样保留，含全部 Exa 参数）
   - `web_search_tool_result` 块 → 拆分为新的 `user` 消息中的 `tool_result` 块（内容为搜索结果文本）
   - 如果 `web_search_tool_result` 之后还有内容（如带 citations 的文本），这些内容移到新的 `assistant` 消息中
3. `web_search_20260209` 的 dynamic filtering 直接忽略（不做特殊处理）。

### Inbound 改写（上游响应 → 客户端）

1. **Streaming**：正常 stream 上游响应给客户端。当遇到 LLM 返回的 `tool_use` 调用我们的 `web_search` function 时：
   - 暂停流，本地收集完整的 tool_use input 参数
   - 将 `tool_use` 转换为 `server_tool_use`（ID 前缀 `toolu_` → `srvtoolu_`），input 原样保留全部 Exa 参数，stream 给客户端。即使原生 `server_tool_use` 不允许这些额外字段，客户端不会因此报错（多余字段忽略即可）。
   - 用收集到的参数调用 Exa 搜索
   - 构造 `web_search_tool_result` 块，stream 给客户端
2. **Non-streaming**：同理，只是在完整 JSON 响应上做转换。
3. **stop_reason 映射**：
   - 如果响应中所有 `tool_use` 都是 web_search → stop_reason 从 `tool_use` 改为 `pause_turn`
   - 如果有混合（web_search + 其它工具）→ stop_reason 保持 `tool_use`
   - `end_turn` 等其它 stop_reason → 不变

### 循环由客户端驱动

不在网关内部做多轮循环。客户端收到 `pause_turn` 后会自动把响应追加到 messages 重新请求。每个请求只做一次上游调用 + 可能的 Exa 调用。

### Exa 调用方式

在网关进程内部构造一个到 exaSearch endpoint path 的请求，使用与原始请求相同的 API key 做认证，走完整的 path-based gateway 路由流程。这样 Exa 调用也会被记录为独立的 request 行。

### Exa 搜索结果

- 调用 Exa 时请求 `highlights`。
- 给 LLM 的 tool_result 放 highlights 文本。
- 返回给客户端的 `web_search_tool_result` 中，`encrypted_content` 放明文 highlights（不加密）。

### 数据库

- `provider` 表新增 `supports_native_web_search BOOLEAN NOT NULL DEFAULT FALSE` 列。
- 中间每一轮 LLM 调用和 Exa 调用都记录为独立的 request 行，就像客户端真的发了这些请求一样。
