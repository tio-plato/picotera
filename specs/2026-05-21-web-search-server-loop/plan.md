# Plan: Server-Side Web Search Loop

实施按以下顺序推进；每一步独立可编译。

## Step 1: 常量 + webSearchContext 扩展

文件 `pkg/server/web_search.go`：

1. 顶部新增：
   ```go
   const webSearchMaxRounds = 10
   ```
2. `webSearchContext` 追加字段：
   ```go
   originalRequestBody []byte // 改写前的 client Anthropic Messages body（深拷贝）
   ```
   不再追加 `depth` 字段、不再加 ctx loop guard——内层自然不触发模拟。

## Step 2: 外层 handler 注入原始 body

文件 `pkg/server/handle_unified_gateway.go`：

1. 在 `wsCtx = &webSearchContext{...}` 构造处补：
   ```go
   originalRequestBody: append([]byte(nil), body...),
   ```

## Step 3: 共用 helper：构造 sub-request body

文件 `pkg/server/web_search.go`：

新方法 / 顶层函数（与现有 `rewriteWebSearchTools` / `rewriteWebSearchHistory` 同文件）：

```go
// buildWebSearchSubBody 构造下一轮自调用 /v1/messages 用的 body。
// originalBody = wsCtx.originalRequestBody（仍带 web_search_2025xxxx 工具）。
// accumulatedContent = 截至目前已经合并完成的、Anthropic-native 形态的 assistant content
//                      blocks（含 server_tool_use + web_search_tool_result）。
// 返回的字节：messages 已追加上述 assistant turn；tools 已通过 rewriteWebSearchTools
// 改为 function-tool 形态；历史已通过 rewriteWebSearchHistory 改为 tool_use / tool_result。
// 因此内层 unified handler 的 hasWebSearchTool 检查会返回 false。
func buildWebSearchSubBody(originalBody []byte, accumulatedContent []json.RawMessage) ([]byte, error)
```

实现要点：
- `body := append([]byte(nil), originalBody...)` 显式深拷贝，确保多轮调用不破坏 `wsCtx.originalRequestBody`。
- gjson 读 `messages` 数组；追加 `{role:"assistant", content: accumulatedContent}`；sjson `SetRawBytes("messages", newArr)` 写回。
- 调 `rewriteWebSearchTools(body)`。
- 调 `rewriteWebSearchHistory(body)`。
- 返回。

## Step 4: 非流式 loop driver

文件 `pkg/server/web_search.go`（可放新文件 `web_search_loop.go`）：

1. 新方法 `(h *gatewayHandler) loopWebSearchNonStream(ctx context.Context, accumulated []byte, wsCtx *webSearchContext, forwardedHeaders http.Header) ([]byte, error)`：
   ```go
   round := 1
   for {
       stopReason := gjson.GetBytes(accumulated, "stop_reason").Str
       if stopReason != "pause_turn" {
           return accumulated, nil
       }
       if round >= webSearchMaxRounds {
           return accumulated, nil // 兜底，stop_reason 仍为 pause_turn
       }

       // gjson 把 accumulated.content 解出来给 buildSubBody
       contentArr := gjsonContentToRawSlice(accumulated)
       subBody, err := buildWebSearchSubBody(wsCtx.originalRequestBody, contentArr)
       if err != nil { return accumulated, nil }

       subReq := httptest.NewRequestWithContext(ctx, "POST", "/api/picotera/v1/messages", bytes.NewReader(subBody))
       for k, vs := range forwardedHeaders { for _, v := range vs { subReq.Header.Add(k, v) } }
       subReq.Header.Set("Content-Type", "application/json")
       subReq.Header.Set("Accept", "application/json")
       subReq.Header.Set("Authorization", "Bearer "+wsCtx.apiKeyToken)
       if wsCtx.metaID != "" { subReq.Header.Set("X-Claude-Code-Session-Id", wsCtx.metaID) }

       rec := httptest.NewRecorder()
       h.Server.router.ServeHTTP(rec, subReq)
       if rec.Code != 200 { return accumulated, nil }
       subBytes := rec.Body.Bytes()

       // 内层不模拟 web_search，外层在这里补 transform。
       subTransformed, err := h.transformWebSearchResponse(ctx, subBytes, wsCtx)
       if err != nil { return accumulated, nil }

       accumulated = mergeNonStreamRound(accumulated, subTransformed)
       round++
   }
   ```

2. `mergeNonStreamRound(outer, sub []byte) []byte`：
   - `outer.content` 追加 `sub.content`（gjson Array → 拼接 → sjson SetRawBytes）。
   - `outer.stop_reason = sub.stop_reason`，`outer.stop_sequence = sub.stop_sequence`。
   - `outer.usage` 字段逐项累加（`input_tokens`、`output_tokens`、`cache_creation_input_tokens`、`cache_read_input_tokens`、`server_tool_use.web_search_requests`，sub 缺失字段保留 outer 值）。
   - `outer.id` / `outer.model` / `outer.role` / `outer.type` 不动。

3. 在 `unifiedStreamSuccess` 非流式分支里：
   ```go
   transformed, terr := h.transformWebSearchResponse(ctx, allBytes, a.wsCtx)
   if terr == nil {
       transformed, terr = h.loopWebSearchNonStream(ctx, transformed, a.wsCtx, buildForwardedHeaders(r))
   }
   ```

## Step 5: SSE transformer 改造（pending pause_turn 解耦）

文件 `pkg/server/web_search_stream.go`：

1. 修改构造函数签名，新增 `holdPauseTurn bool` 参数：
   ```go
   func newWebSearchSSETransformer(ctx context.Context, upstream io.ReadCloser,
       wsCtx *webSearchContext, h *gatewayHandler, holdPauseTurn bool) *webSearchSSETransformer
   ```
   返回类型改为具名 `*webSearchSSETransformer`（原来返回 `io.ReadCloser`），让 driver 能访问 `HasPendingPauseTurn` / `Snapshot` 等方法。调用处（`handle_unified_gateway.go` 里现有的 non-loop 路径）传 `holdPauseTurn=false` 保持行为不变。

2. `webSearchSSETransformer` 追加字段：
   ```go
   holdPauseTurn       bool                       // 仅 outer transformer 设为 true
   outerContentBlocks  []json.RawMessage          // 输出 index → 最终 block 对象
   blockBuilders       map[int64]*streamingBlock  // 输出 index → 增量累积器
   outerUsage          map[string]any             // 累积 message_start.usage + message_delta.delta.usage
   pendingMessageDelta []byte                     // pause_turn 版本的 message_delta 帧字节（仅 holdPauseTurn 时）
   pendingMessageStop  []byte                     // 配对的 message_stop 帧字节（仅 holdPauseTurn 时）
   ```

3. 透传 content_block_* 时同步喂 `blockBuilders` / 写回 `outerContentBlocks`（按 design 中"content 累积"小节实现）。

4. `writeMessageDelta` 改造：
   - 始终把 `delta.usage` 累加进 `outerUsage`。
   - 若 `holdPauseTurn && webSearchCalls > 0 && otherToolCalls == 0`：构造 stop_reason=pause_turn 的帧字节存进 `pendingMessageDelta`；**不写 pipe**；return。
   - 否则：原逻辑透传（含 stop_reason 改写）。

5. 主循环遇到 `message_stop`：
   - 若 `pendingMessageDelta != nil`：把帧字节存进 `pendingMessageStop`，**不写 pipe**。`run()` 正常退出，defer 里 `pw.Close()` 把 pipe writer 关掉——reader 收到 EOF。
   - 否则透传，`run()` 正常退出，`pw.Close()` 关 pipe，自然结束。
   - 两种路径 pipe writer 都通过 `run()` 的 defer 关闭，无需额外 channel。

6. 暴露：
   ```go
   func (t *webSearchSSETransformer) HasPendingPauseTurn() bool
   func (t *webSearchSSETransformer) Snapshot() (content []json.RawMessage, usage map[string]any)
   func (t *webSearchSSETransformer) PendingFrames() (delta, stop []byte) // 兜底时给 driver 用
   ```

## Step 6: streamingResponseRecorder

新文件 `pkg/server/streaming_response_recorder.go`：

```go
type streamingResponseRecorder struct {
    header      http.Header
    code        int
    pr          *io.PipeReader
    pw          *io.PipeWriter
    once        sync.Once
    statusReady chan struct{} // WriteHeader 后关闭
}

// 实现 http.ResponseWriter + http.Flusher：
//   Header()        → header
//   WriteHeader(c)  → once.Do: 保存 code，关闭 statusReady channel
//   Write(p)        → 若 once 未触发则先 WriteHeader(200)；pw.Write(p)
//   Flush()         → no-op
//
// 暴露：
//   func (r *streamingResponseRecorder) Reader() *io.PipeReader
//   func (r *streamingResponseRecorder) StatusCode() int
//   func (r *streamingResponseRecorder) StatusReady() <-chan struct{} // WriteHeader 后可读
//   func (r *streamingResponseRecorder) Close() error  // 关 pw
```

Loop driver 在创建 sub-transformer 之前先 `<-rec.StatusReady()`；非 200 直接 fallback，避免把 JSON error response 喂进 SSE parser。

## Step 7: webSearchSSELoopDriver

新文件 `pkg/server/web_search_stream_loop.go`：

```go
type webSearchSSELoopDriver struct {
    transformer      *webSearchSSETransformer // 第一轮 transformer
    wsCtx            *webSearchContext
    h                *gatewayHandler
    ctx              context.Context
    forwardedHeaders http.Header

    outPR *io.PipeReader
    outPW *io.PipeWriter

    accumulatedBlocks []json.RawMessage
    accumulatedUsage  map[string]any
    indexOffset       int64
    round             int
}

func newWebSearchSSELoopDriver(...) *webSearchSSELoopDriver { ... }
func (d *webSearchSSELoopDriver) Read(p []byte) (int, error) { return d.outPR.Read(p) }
func (d *webSearchSSELoopDriver) Close() error               { return d.outPR.Close() }
```

`run()` goroutine（构造时启动）：

1. **阶段 A**：`io.Copy(d.outPW, d.transformer)` 把 outer transformer 的 pipe 全读完写进 outPipe。Transformer 的 `run()` 结束后 defer 关 pw，reader 看到 EOF，copy 返回。
2. **阶段 B**：
   - `if !d.transformer.HasPendingPauseTurn()` → 关 `d.outPW` 结束。
   - 否则：
     - `d.accumulatedBlocks, d.accumulatedUsage = d.transformer.Snapshot()`
     - `d.indexOffset = int64(len(d.accumulatedBlocks))`
     - `d.round = 1`
     - 进入阶段 C。
3. **阶段 C 循环**：
   ```go
   for d.round < webSearchMaxRounds {
       subBody, err := buildWebSearchSubBody(d.wsCtx.originalRequestBody, d.accumulatedBlocks)
       if err != nil { d.fallbackPauseTurn(); return }

       subReq := httptest.NewRequestWithContext(d.ctx, "POST", "/api/picotera/v1/messages", bytes.NewReader(subBody))
       d.applyForwardedHeaders(subReq) // Authorization / X-Claude-Code-Session-Id 等
       subReq.Header.Set("Content-Type", "application/json")
       subReq.Header.Set("Accept", "text/event-stream") // 强制流式

       rec := newStreamingResponseRecorder()
       go func() { defer rec.Close(); d.h.Server.router.ServeHTTP(rec, subReq) }()

       <-rec.StatusReady()
       if rec.StatusCode() != 200 { d.fallbackPauseTurn(); return }

       // 内层 unified handler 不模拟 web_search（sub body 里没有 web_search_2025xxxx），
       // 但响应里仍可能含 tool_use(web_search)——外层套一层 sub-transformer 现场转换。
       // holdPauseTurn=false：sub-transformer 把 pause_turn 帧正常写出 pipe。
       subTransformer := newWebSearchSSETransformer(d.ctx, rec.Reader(), d.wsCtx, d.h, false)

       shouldLoop, err := d.forwardSubStream(subTransformer)
       if err != nil { d.fallbackPauseTurn(); return }

       // 从 sub-transformer 获取本轮累积的 content blocks
       subBlocks, subUsage := subTransformer.Snapshot()
       d.accumulatedBlocks = append(d.accumulatedBlocks, subBlocks...)
       mergeUsageInto(d.accumulatedUsage, subUsage)

       d.round++
       d.indexOffset = int64(len(d.accumulatedBlocks))
       if !shouldLoop { d.outPW.Close(); return }
   }
   d.fallbackPauseTurn()
   ```
4. `forwardSubStream(sub *webSearchSSETransformer) (shouldLoop bool, err error)`：
   - 从 sub-transformer 的 pipe 按 `\n\n` 切 SSE 帧（复用 `parseSSEFrame`）。
   - by `data.type`：
     - `message_start`：丢弃。
     - `content_block_start` / `content_block_delta` / `content_block_stop`：`index += d.indexOffset`，写 `d.outPW`。
     - `message_delta`：
       - `delta.stop_reason == "pause_turn"` → **不输出**，return `(true, nil)`。
       - 否则 → 把 `delta.usage` 改写为 `d.accumulatedUsage`（含本轮），输出，return `(false, nil)`。
     - `message_stop`：仅在「非 pause_turn 路径」与 message_delta 配对 → 输出一次；pause_turn 路径丢弃。
     - `ping` 等：透传。
   - 收到 EOF 但还没看到 message_delta → return `(false, errUnexpectedEOF)`。
   - **注意**：content block 不在 forwardSubStream 里累积——从 sub-transformer 的 `Snapshot()` 获取（见上方循环体）。
5. `fallbackPauseTurn()`：取 outer transformer 的 `PendingFrames`（如果还有）或自己构造一帧带 `accumulatedUsage` 的 pause_turn message_delta + message_stop，写出，关 `d.outPW`。

## Step 8: 接入 unifiedStreamSuccess

文件 `pkg/server/handle_unified_gateway.go`：

**流式分支**：
```go
if a.wsCtx != nil && a.wsCtx.active {
    transformer := newWebSearchSSETransformer(ctx, clientReader, a.wsCtx, h, true) // holdPauseTurn=true
    driver := newWebSearchSSELoopDriver(ctx, transformer, a.wsCtx, h, buildForwardedHeaders(r))
    clientReader = driver
}
```

**现有 non-loop 调用点**（同文件，流式但非 loop 的路径如果有）：如果未来有不走 loop 的 SSE 路径，传 `holdPauseTurn=false`。当前改完后所有流式 web search 都走 loop driver，所以只有上面一个调用点。

`buildForwardedHeaders(r)` 复制以下 header：
- `Authorization`（sub-call 鉴权）
- `X-Claude-Code-Session-Id`（session 追踪，如果存在）
其余 header 不带；`Content-Type`、`Accept` 由 loop driver 自行设置。

## Step 9: 测试

现有 handler 测试只覆盖纯函数 helper（因为 Server 依赖 postgres 实例，无 test harness）。本步分两层：

### 9a: 纯函数 / 小组件单测

文件 `pkg/server/web_search_loop_test.go`（新建）：

- `TestBuildWebSearchSubBody`：验证输出 body 的 messages 正确追加 assistant turn、tools 已改为 function-tool 形式、历史已 rewrite。
- `TestBuildWebSearchSubBody_DoesNotMutateInput`：连续调两次，断言 `originalBody` 未被篡改。
- `TestMergeNonStreamRound`：验证 content 追加、stop_reason 替换、usage 逐项累加。
- `TestMergeUsage`：边界用例（sub 缺失某些 usage 字段、server_tool_use.web_search_requests 累加）。

文件 `pkg/server/web_search_stream_test.go`（新建）：

- `TestSSETransformer_HoldPauseTurn`：构造一段含 tool_use(web_search) + message_delta(tool_use) + message_stop 的 mock SSE 流，`holdPauseTurn=true`。断言：pipe 输出不含 message_delta/message_stop；`HasPendingPauseTurn()` 返回 true；`Snapshot()` 返回正确的 content blocks + usage。
- `TestSSETransformer_NoHoldPauseTurn`：同上但 `holdPauseTurn=false`。断言：pipe 输出包含 stop_reason=pause_turn 的 message_delta + message_stop。
- `TestStreamingResponseRecorder_StatusReady`：验证 `StatusReady()` 在 `WriteHeader` 后关闭；隐式 `Write` 也触发。

### 9b: handler 集成测试（需 postgres test harness）

这些测试需要实例化 Server，目前 picotera 还没有 test harness。暂列为 **TODO**，等 harness 就绪后补：

- `TestUnifiedWebSearchLoop_NonStream_TwoRounds`
- `TestUnifiedWebSearchLoop_NonStream_DepthLimit`
- `TestUnifiedWebSearchLoop_NonStream_StopsAtNonWebSearch`
- `TestUnifiedWebSearchLoop_Stream_TwoRounds`
- `TestUnifiedWebSearchLoop_Stream_DepthLimit`

## Step 10: 验证

1. `go build ./cmd/picotera`。
2. `go test ./pkg/server/...`。
3. 手动 E2E：配置 1 个 `supports_native_web_search=false` 的 provider + 1 个 Exa endpoint + 1 个 api key，分别发 `Accept: application/json` 和 `Accept: text/event-stream` 的带 `web_search_20250305` 工具的请求；验证客户端单次响应里多轮 web_search 结果按顺序拼接，trace 视图能看到多条 meta 行各自独立。
4. `pnpm --dir dashboard type-check` & `lint`（前端无变化，跑一次确认 CI 不挂）。
