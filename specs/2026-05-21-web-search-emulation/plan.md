# Plan: Web Search Emulation via Exa

## Step 1: Database Migration + sqlc

1. 新建 `db/migrations/024_provider_supports_native_web_search.sql`：
   ```sql
   -- +goose Up
   ALTER TABLE provider ADD COLUMN supports_native_web_search BOOLEAN NOT NULL DEFAULT FALSE;
   -- +goose Down
   ALTER TABLE provider DROP COLUMN supports_native_web_search;
   ```
2. `db/queries/routing.sql`：`GetProvidersByEndpointTypesAndModel` 的 SELECT 追加 `p.supports_native_web_search`。
3. `db/queries/endpoint.sql`：新增 `GetFirstEndpointByType` 查询。
4. 运行 `sqlc generate`。

## Step 2: Contract + Provider CRUD

1. `pkg/db/models.go`（sqlc 生成）：确认 `Provider` struct 有 `SupportsNativeWebSearch bool`。
2. `pkg/contract/provider.go`：
   - `ProviderView` 追加 `SupportsNativeWebSearch bool` 字段。
   - `CreateProviderRequest.Body`、`UpsertProviderRequest.Body` 追加同名字段。
   - `ToProviderView` / `FromProviderView` 映射新列。
3. `pkg/server/handle_provider.go`（如有）：CRUD handler 不需要特殊逻辑，字段自动流转。
4. `mise run openapi && pnpm --dir dashboard generate-openapi`。

## Step 3: Sidecar 传播

1. `pkg/server/handle_unified_gateway.go`：
   - `unifiedSidecar` struct 追加 `supportsNativeWebSearch bool`。
   - sidecar 构建循环中，从 `GetProvidersByEndpointTypesAndModelRow` 读取新字段并存入 sidecar。
2. `unifiedStreamArgs` struct 追加 `wsActive bool` 和 `wsCtx *webSearchContext`（后续步骤用）。

## Step 4: Outbound 改写（`pkg/server/web_search.go`）

新文件。实现：

1. `webSearchContext` struct：
   - `active bool`
   - `apiKeyToken string`（用于 Exa 调用的客户端 credential）
   - `metaID string`、`metaCreatedAt time.Time`（用于 Exa 调用的 trace 关联）
   - `toolCallCount int`（响应中 web_search tool_use 计数）
   - `otherToolCallCount int`（响应中非 web_search tool_use 计数）

2. `hasWebSearchTool(body []byte) bool`：gjson 检查 `tools.#(type=="web_search_20250305")` 或 `tools.#(type=="web_search_20260209")`。

3. `rewriteWebSearchTools(body []byte) []byte`：
   - 遍历 `tools` 数组，替换匹配的 server tool 为 function tool 定义（input_schema 暴露 `query`、`numResults`、`category`、`includeDomains`、`excludeDomains`、`startPublishedDate`、`endPublishedDate`）。
   - 用 sjson 操作。

4. `rewriteWebSearchHistory(body []byte) []byte`：
   - 解析 `messages` 为 `[]json.RawMessage`。
   - 遍历 assistant 消息的 content blocks，按规则拆分和转换。
   - 重新序列化 messages 数组，用 `sjson.SetRawBytes` 写回 body。

5. 在 `handleUnifiedGenerate` 的 retry loop 中，`rewriteRequest` hook 之后、bridge 之前插入调用。

## Step 5: Exa 调用（`pkg/server/web_search.go` 续）

1. 定义 `ExaSearchRequest` 和 `ExaSearchResponse` Go structs（请求侧含 `query` 必填及 `numResults`、`category`、`includeDomains`、`excludeDomains`、`startPublishedDate`、`endPublishedDate` 可选字段；响应侧只保留需要的结果字段）。

2. `callExa(ctx, toolInput json.RawMessage, apiKeyToken, parentSpanID string) (*ExaSearchResponse, error)`：
   - `GetFirstEndpointByType(ctx, EndpointType_ExaSearch)` 找 endpoint path。
   - 将 LLM 给的 `toolInput`（含 query + 可选 Exa 参数）反序列化进 `ExaSearchRequest`，再补 `contents.highlights = true`。
   - 构造 `httptest.NewRequest("POST", path, body)`，设 `Authorization: Bearer <apiKeyToken>` 和 `Content-Type: application/json`。
   - `httptest.NewRecorder()` 捕获响应。
   - `gatewayHandler{h.Server}.ServeHTTP(rec, req)` 走完整 path-based gateway。
   - 解析 `rec.Result()` 为 `ExaSearchResponse`。

3. `buildWebSearchToolResult(toolUseID string, exaResp *ExaSearchResponse) json.RawMessage`：
   - 将 Exa 结果映射为 `web_search_tool_result` JSON。

4. `buildWebSearchToolResultError(toolUseID string, errorCode string) json.RawMessage`：
   - 构造错误格式的 `web_search_tool_result`。

## Step 6: Inbound 改写 — 非流式（`pkg/server/web_search.go` 续）

`transformWebSearchResponse(body []byte, wsCtx *webSearchContext, h *gatewayHandler) ([]byte, error)`：

1. gjson 解析 `content` 数组。
2. 遍历 content blocks，识别 web_search tool_use。
3. 对每个 web_search tool_use：调 `callExa`，转换 block type + ID，注入 `web_search_tool_result`。
4. 调整所有 index。
5. 修改 `stop_reason`（全是 web_search → `pause_turn`）。
6. 用 sjson 重写 `content` 和 `stop_reason`。

## Step 7: Inbound 改写 — 流式 SSE transformer

新文件 `pkg/server/web_search_stream.go`。

1. 提取 SSE 解析辅助函数（从 `response_extractor.go` 的行缓冲逻辑抽取，或独立实现简易 SSE parser）。

2. `webSearchSSETransformer` struct，实现 `io.ReadCloser`：
   - 内部用 `io.Pipe()`。
   - 后台 goroutine 从上游 ReadCloser 逐事件读取 SSE 事件。
   - 状态机：PASSTHROUGH / BUFFERING / EMITTING。
   - PASSTHROUGH：事件原样写入 pipe writer（调整 index 如有偏移）。
   - BUFFERING：抑制 web_search tool_use 的 content_block 事件，累积 input JSON。
   - EMITTING：构造 server_tool_use SSE 事件 + 调用 Exa + 构造 web_search_tool_result SSE 事件 → 写入 pipe。
   - message_delta 事件：修改 stop_reason → 写入。
   - goroutine 结束时关闭 pipe writer。

3. SSE 事件编码辅助函数：`encodeSSEEvent(eventType, data string) []byte`，格式 `event: TYPE\ndata: DATA\n\n`。

## Step 8: 集成到 `handle_unified_gateway.go`

### Outbound 集成

在 retry loop 的 `buildRequestFromPending` 之后、`beforeTransform` hook 之前：

```go
var wsCtx *webSearchContext
if srcFormat == llmbridge.FormatAnthropicMessages &&
   hasWebSearchTool(reqBody) &&
   !side.supportsNativeWebSearch {
    reqBody = rewriteWebSearchTools(reqBody)
    reqBody = rewriteWebSearchHistory(reqBody)
    wsCtx = &webSearchContext{
        active:      true,
        apiKeyToken: clientToken,
        metaID:      metaID,
        metaCreatedAt: metaCreatedAt,
    }
    // 更新 req.Body, req.ContentLength, req.GetBody
    req.Body = io.NopCloser(bytes.NewReader(reqBody))
    req.ContentLength = int64(len(reqBody))
    req.GetBody = func() (io.ReadCloser, error) {
        return io.NopCloser(bytes.NewReader(reqBody)), nil
    }
}
```

### Inbound 集成（unifiedStreamSuccess）

```go
// 在 clientReader 构建完成后
if wsCtx != nil && wsCtx.active {
    if streamMode {
        clientReader = newWebSearchSSETransformer(ctx, clientReader, wsCtx, h)
    } else {
        // 非流式：读取全部 → transform → 包装为 reader
        allBytes, _ := io.ReadAll(clientReader)
        clientReader.Close()
        transformed, _ := transformWebSearchResponse(allBytes, wsCtx, h)
        clientReader = io.NopCloser(bytes.NewReader(transformed))
    }
}
```

需要把 `wsCtx` 从 retry loop 传递到 `unifiedStreamSuccess`。在 `unifiedStreamArgs` 中加字段，或在调用 `unifiedStreamSuccess` 前设入闭包变量。

### clientToken 传递

`authenticateClient` 返回 `*db.ApiKey`。从 request header 中提取原始 token（`extractClientToken`）并保存到闭包变量，后续传入 `wsCtx.apiKeyToken`。

## Step 9: 前端

1. `dashboard/src/components/ProviderForm.vue`：新增 checkbox 字段 "支持原生 Web 搜索"，绑定 `form.supportsNativeWebSearch`。
2. `dashboard/src/api/client.ts`：provider CRUD fetcher 无需额外改动（类型自动从 openapi-types 继承）。
3. 重新生成 OpenAPI + TS 类型：`mise run openapi && pnpm --dir dashboard generate-openapi`。

## Step 10: 验证

1. `go build ./cmd/picotera` 确认编译通过。
2. `pnpm --dir dashboard type-check` 确认前端类型正确。
3. `pnpm --dir dashboard lint` 确认 lint 通过。
4. 手动测试：
   - 配置一个 provider（`supports_native_web_search = false`）+ 一个 exaSearch endpoint + provider binding。
   - 发送带 `web_search_20250305` 工具的请求。
   - 验证响应中有 `server_tool_use` + `web_search_tool_result` + `pause_turn` stop_reason。
   - 验证 request 表中有独立的 Exa 调用记录。
