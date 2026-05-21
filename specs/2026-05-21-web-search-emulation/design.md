# Design: Web Search Emulation via Exa

## 总览

当客户端通过 `/api/picotera/v1/messages` 发送携带 Anthropic 原生 web search 工具（`web_search_20250305` / `web_search_20260209`）的请求，且所选上游供应商不支持原生 web search 时，网关在请求/响应边界做双向改写：

- **Outbound**：将 web search 服务端工具替换为普通 function tool，将历史消息中的 `server_tool_use` / `web_search_tool_result` 转换为 `tool_use` / `tool_result`。
- **Inbound**：将上游响应中的 `tool_use`（web_search function call）转换为 `server_tool_use`，在本地调用 Exa 搜索并注入 `web_search_tool_result`，将 `tool_use` stop_reason 改为 `pause_turn`。

不在网关内部做多轮循环。客户端收到 `pause_turn` 后按 Anthropic 标准协议重新发送请求，循环由客户端驱动。

## 数据模型

### Migration 024

`provider` 表新增一列：

```sql
ALTER TABLE provider ADD COLUMN supports_native_web_search BOOLEAN NOT NULL DEFAULT FALSE;
```

不动其它表。

### sqlc

`db/queries/routing.sql` 中 `GetProvidersByEndpointTypesAndModel` 查询的 SELECT 列表追加 `p.supports_native_web_search`。`GetProvidersByEndpointAndModel` 和 `GetProvidersByEndpoint` 不动（path-based gateway 不做 web search 改写）。

运行 `sqlc generate` 更新 `pkg/db/`。

### Contract / API

`ProviderView` 和相关请求体追加 `SupportsNativeWebSearch bool` 字段。`ToProviderView` / `FromProviderView` 映射新列。OpenAPI + TS 类型照常重新生成。

### 新增查询

`db/queries/endpoint.sql` 新增：

```sql
-- name: GetFirstEndpointByType :one
SELECT * FROM endpoint WHERE endpoint_type = $1 LIMIT 1;
```

用于 Exa 调用时查找 exaSearch 端点路径。

## Outbound 改写

新文件 `pkg/server/web_search.go`。

### 检测

`hasWebSearchTool(body []byte) bool` — 用 gjson 检查 `tools` 数组中是否有 `type` 为 `web_search_20250305` 或 `web_search_20260209` 的元素。

### 工具替换

`rewriteWebSearchTools(body []byte) ([]byte, webSearchToolDef)`

将匹配的 server tool 替换为暴露 Exa 主要参数的 function tool：

```json
{
  "name": "web_search",
  "description": "Search the web for current information. Returns relevant snippets from web pages.",
  "input_schema": {
    "type": "object",
    "properties": {
      "query": {"type": "string", "description": "The search query"},
      "numResults": {"type": "integer", "description": "Number of results to return (default 10, max 25)"},
      "category": {"type": "string", "enum": ["company", "research paper", "news", "personal site", "financial report", "people"], "description": "Optional category filter"},
      "includeDomains": {"type": "array", "items": {"type": "string"}, "description": "Restrict results to these domains"},
      "excludeDomains": {"type": "array", "items": {"type": "string"}, "description": "Exclude results from these domains"},
      "startPublishedDate": {"type": "string", "description": "ISO 8601 date; only results published after this"},
      "endPublishedDate": {"type": "string", "description": "ISO 8601 date; only results published before this"}
    },
    "required": ["query"]
  }
}
```

保留 tools 数组中其他工具不变。

### 历史消息转换

`rewriteWebSearchHistory(body []byte) []byte`

遍历 `messages` 数组，对每个 assistant 消息的 content blocks：

1. 遇到 `server_tool_use`（name = `web_search`）→ 转为 `tool_use`，ID 前缀 `srvtoolu_` → `toolu_`。归入当前 assistant 消息。
2. 遇到 `web_search_tool_result` → 拆成新的 user 消息中的 `tool_result`，content 为从 `web_search_result` 中提取的文本（url + title + encrypted_content 里的明文 highlights）。ID 同步转换。
3. `web_search_tool_result` 之后的 text blocks → 剥离 `citations` 字段（上游不认识 `web_search_result_location` 类型的 citations），归入新的 assistant 消息。
4. 其它 block 类型（text, tool_use, tool_result 等）→ 原样保留。

实现方式：将 `messages` 反序列化为 `[]json.RawMessage`，逐条解析 assistant content blocks，按上述规则拆分和转换后重新序列化。用 `sjson.SetBytes` 把转换后的 messages 写回 body。

### 集成点

在 `handleUnifiedGenerate` 的 retry loop 内部，`rewriteRequest` hook 之后、bridge 步骤之前：

```go
// 仅当 srcFormat == AnthropicMessages 且请求含 web search 工具
wsActive := false
if srcFormat == llmbridge.FormatAnthropicMessages && hasWebSearchTool(reqBody) && !side.supportsNativeWebSearch {
    reqBody = rewriteWebSearchTools(reqBody)
    reqBody = rewriteWebSearchHistory(reqBody)
    wsActive = true
    // 更新 req.Body 和 req.ContentLength
}
```

`side.supportsNativeWebSearch` 来自路由查询结果行中新增的列，在 sidecar 构建时存入。

## Inbound 改写

### 非流式

`transformWebSearchResponse(body []byte, server, apiKeyToken, metaID, metaCreatedAt) ([]byte, error)`

1. 用 gjson 解析 `content` 数组。
2. 遍历 content blocks：
   - `type: "tool_use"` 且 `name: "web_search"` → 转为 `server_tool_use`（ID 前缀 `toolu_` → `srvtoolu_`，input 原样保留全部 Exa 参数），紧随其后注入一个 `web_search_tool_result` block。
   - 其它 block → 保留。
3. 对每个 web_search tool_use，调用 Exa 获取搜索结果（见下方 Exa 调用），构造 `web_search_tool_result`。
4. 调整 content block 的 index。
5. 如果所有 tool_use 都是 web_search → `stop_reason` 改为 `"pause_turn"`。如果有混合 → 保持 `"tool_use"`。
6. 返回修改后的 JSON body。

### 流式

新建 `webSearchSSETransformer`，实现 `io.ReadCloser`。它包裹 bridge 输出（或 1:1 的 teedUpstream），拦截 Anthropic SSE 事件流。

状态机：

```
PASSTHROUGH → 遇到 tool_use(web_search) content_block_start → BUFFERING
BUFFERING   → 收集 content_block_delta，直到 content_block_stop → EMITTING
EMITTING    → 发出 server_tool_use + web_search_tool_result，回到 PASSTHROUGH
```

具体逻辑：

1. **PASSTHROUGH**：逐事件读取上游 SSE。非 web_search 相关事件原样写入输出 buffer。
2. **检测 web_search tool_use**：当 `content_block_start` 的 `content_block.type == "tool_use"` 且 `content_block.name == "web_search"` 时，进入 BUFFERING。
3. **BUFFERING**：抑制该 block 的所有 `content_block_start` / `content_block_delta` / `content_block_stop` 事件。累积 `input_json_delta` 中的 `partial_json` 拼成完整 JSON。
4. **EMITTING**（`content_block_stop` 到达时）：
   a. 解析累积的 JSON 得到 `query`。
   b. 生成 `content_block_start`（`type: "server_tool_use"`, ID 前缀改 `srvtoolu_`，完整 input）→ 写入输出 buffer + flush。
   c. 生成 `content_block_delta`（`input_json_delta`，完整 JSON 一次发完）→ 写入 + flush。
   d. 生成 `content_block_stop` → 写入 + flush。
   e. 调用 Exa（阻塞）。
   f. 生成 `content_block_start`（`type: "web_search_tool_result"`，含搜索结果）→ 写入 + flush。
   g. 生成 `content_block_stop` → 写入 + flush。
   h. 记录 index 偏移量（每个 web_search 增加 1，因为 tool_use → server_tool_use + web_search_tool_result 多了一个 block）。
5. **index 调整**：后续事件中所有 `index` 字段加上累积偏移量。
6. **message_delta**：如果所有 tool_use 都是 web_search → `stop_reason` 改为 `"pause_turn"`。
7. **message_stop**：原样传递。

实现采用 `io.Pipe()`：goroutine 从上游读取事件并写入 pipe，主循环从 pipe 读取。SSE 事件解析复用 `response_extractor.go` 中的行缓冲/事件边界检测逻辑，提取为内部 helper。

### 集成点

在 `unifiedStreamSuccess` 中，clientReader 构建完成后、写入循环之前：

```go
if wsActive {
    clientReader = newWebSearchSSETransformer(ctx, clientReader, wsCtx, h)
}
```

非流式同理，在 bridge 输出或 upstream body 上调 `transformWebSearchResponse`。

## Exa 调用

### 路由

在网关进程内部构造一个到 exaSearch endpoint 的 HTTP 请求，走完整的 path-based gateway 路由流程：

```go
func (h *gatewayHandler) callExa(ctx context.Context, query string, apiKeyToken string, parentSpanID string) (*ExaSearchResult, error) {
    // 1. 查找 exaSearch endpoint path
    ep, err := h.queries.GetFirstEndpointByType(ctx, contract.EndpointType_ExaSearch)
    // ep.Path → e.g. "/exa/search"

    // 2. 构造 Exa 请求 body
    body := ExaSearchRequest{
        Query:    query,
        Contents: ExaContents{Highlights: true},
    }

    // 3. 构造 HTTP 请求
    req := httptest.NewRequest("POST", ep.Path, jsonReader(body))
    req.Header.Set("Content-Type", "application/json")
    // 用原始客户端的 API key 认证
    req.Header.Set("Authorization", "Bearer " + apiKeyToken)
    // 传递 parent span ID 以建立 trace 关联
    req.Header.Set("X-Parent-Span-Id", parentSpanID)

    // 4. 调用 path-based gateway handler
    rec := httptest.NewRecorder()
    gw := &gatewayHandler{h.Server}
    gw.ServeHTTP(rec, req)

    // 5. 解析响应
    resp := rec.Result()
    if resp.StatusCode != 200 { ... }
    var exaResp ExaSearchResponse
    json.NewDecoder(resp.Body).Decode(&exaResp)
    return &exaResp, nil
}
```

这样 Exa 调用：
- 通过同一个 API key 认证
- 由 path-based gateway 做 provider 解析、重试、日志记录
- 自动生成独立的 request 行（meta + upstream）
- 支持 proxy、JS hooks 等现有功能

### 请求格式

LLM 的 tool_use input 中的 Exa 参数直接合并进 Exa 请求 body，再补上固定的 `contents.highlights: true`：

```json
{
  "query": "搜索词",
  "numResults": 10,
  "category": "news",
  "includeDomains": ["..."],
  "excludeDomains": ["..."],
  "startPublishedDate": "...",
  "endPublishedDate": "...",
  "contents": {"highlights": true}
}
```

未指定的字段省略。`query` 必填，其余可选。

### 响应解析

从 Exa 响应中提取 `results[].{url, title, highlights[], publishedDate}`。

## web_search_tool_result 构造

每个搜索结果 → 一个 `web_search_result` 对象：

```json
{
  "type": "web_search_result",
  "url": "https://example.com",
  "title": "Page Title",
  "encrypted_content": "Highlight 1\n\nHighlight 2\n\nHighlight 3",
  "page_age": "May 20, 2026"
}
```

- `encrypted_content`：Exa highlights 用 `\n\n` 拼接，明文存放。
- `page_age`：从 `publishedDate` 格式化为人类可读字符串（`"January 2, 2006"` Go 格式）。没有则省略。
- 外层 `web_search_tool_result` 的 `tool_use_id` 对应 `server_tool_use` 的 `id`。

### 给 LLM 的 tool_result（历史消息转换时）

当下一轮请求携带 `web_search_tool_result` 历史时，转换为 tool_result 的 content 格式：

```
Search results:

1. Page Title
   URL: https://example.com
   Highlight 1
   Highlight 2

2. Another Page
   URL: https://example2.com
   Highlight 3
```

从 `web_search_result` 的 `title`、`url`、`encrypted_content` 字段提取。

## 错误处理

- **无 exaSearch 端点**：`callExa` 返回错误 → 构造 `web_search_tool_result` 的 error 格式：`{"type": "web_search_tool_result_error", "error_code": "unavailable"}`。
- **Exa 调用失败（非 200）**：同上，`error_code: "unavailable"`。
- **provider 支持原生 web search**：不做任何改写，请求/响应原样透传。

## 前端

### ProviderForm

新增 `SupportsNativeWebSearch` checkbox，标签如 "支持原生 Web 搜索"。只在编辑 / 新建 provider 时显示。

### ProvidersView

provider 列表不显示此字段（非核心信息，不需要占列宽）。

## 不做的事

- 不支持 `web_search_20260209` 的 dynamic filtering（直接忽略）。
- 不把原生 web search tool 的参数（`max_uses`、`allowed_domains` 等）转译到 Exa；客户端构造的工具会被整个替换，Exa 参数完全由 LLM 通过我们暴露的 function tool input 给出。
- 不在 path-based gateway（`handle_gateway.go`）中做 web search 改写，仅 unified `/v1/messages` 路由。
- 不实现 citations（上游 LLM 不产生 `web_search_result_location` 类型的引用标注）。
- 不加密 `encrypted_content`。
- 不修改 `response_extractor.go`（token 计量仍从上游原始格式提取，web search 改写在 extractor 之后）。
