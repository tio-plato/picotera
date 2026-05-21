# Design: Server-Side Web Search Loop

## 总览

把现有「web search emulation 单轮 + `pause_turn`」改成「网关侧自循环到非 `tool_use` 终态」。仅作用于 `POST /api/picotera/v1/messages` 路由，且仅在该次响应里至少触发了一次 web_search 模拟时启动循环。

实现方式：**循环放在最外层 handler 内部，扁平迭代**。第一轮走现有 retry loop / web_search 改写产出客户端可见的响应内容；如果停止原因是 `pause_turn`（即本轮至少一次 web_search 且没有任何非 web_search 的 tool_use），外层 handler 用 `httptest.NewRequest` 自调用 `/api/picotera/v1/messages` 跑下一轮，把子响应 content 续接进同一次 HTTP 输出流；如此循环到上游返回非 `pause_turn` 或达到上限。

每一轮自调用走完整的 chi 路由 + unified handler 链路（routing → sidecar → JS hooks → bridge → upstream），各自记录独立的 meta + upstream `request` 行。

**自调用的内层 handler 不会再触发 web_search 模拟**——因为外层在构造 sub-request body 时**已经把 `web_search_20250305` / `web_search_20260209` 改写成 function-tool 形式的 `web_search`**，并把历史里的 `server_tool_use` / `web_search_tool_result` 也已经改写成 `tool_use` / `tool_result`。内层 unified handler 进入 `hasWebSearchTool(reqBody)` 检查时返回 false，`wsCtx` 永远为 nil，走 vanilla 透传路径。不需要任何 ctx 哨兵、header 或显式 flag。

代价：内层不模拟，意味着 **round 2+ 上游返回的 `tool_use(web_search)`（function tool call）需要由外层 loop driver 自己转换成 `server_tool_use` + `web_search_tool_result`**。也就是说外层在每一轮 sub-call 之后都要复用 `transformWebSearchResponse`（非流式）或 `webSearchSSETransformer`（流式）来做一次 Anthropic-native 形态转换。这部分逻辑本来就是已有的，不需要新写——只是它的调用位置从「unified handler 内部 + 第一轮上游响应」扩展到「loop driver 内部 + 每一轮 sub-call 响应」。

## 终止条件 & 轮数

- **最大轮数**：硬编码 `webSearchMaxRounds = 10`（含第一轮）。
- **轮数追踪**：外层 handler / loop driver 用局部变量 `round int`（初值 1，第一轮跑完后递增）维护，完全不跨进程边界，所以不需要 header 或 ctx 传递。
- **达到上限**：`round >= webSearchMaxRounds` 时不再发起下一轮自调用，把当前累积响应（最后一轮 stop_reason 仍为 `pause_turn`）原样吐给客户端兜底。
- **终止判据**：本轮（含 web_search 转换）的响应 `stop_reason == "pause_turn"` 则发起下一轮；其他所有 stop_reason（`end_turn` / `max_tokens` / 真正的 `tool_use` / `stop_sequence`）直接结束。

## 数据流：非流式

`unifiedStreamSuccess` 在「非流式 + wsCtx.active」分支里，先按现有逻辑把上游 JSON 经 `transformWebSearchResponse` 转成「含 server_tool_use + web_search_tool_result」的源格式响应（这就是本轮的「accumulated」起点）。新增逻辑：

```text
accumulated := transformedRound1   // 第一轮已合并 web_search 结果的源格式响应
round       := 1

for accumulated.stop_reason == "pause_turn" && round < webSearchMaxRounds {
    subBody := buildSubBody(wsCtx.originalRequestBody, accumulated.content)
    subReq  := httptest.NewRequestWithContext(ctx, "POST", "/api/picotera/v1/messages", subBody)
    subReq.Header = forwardedHeaders(r) // Authorization / Accept=application/json / Session-Id
    rec := httptest.NewRecorder()
    h.Server.router.ServeHTTP(rec, subReq)

    if rec.Code != 200 {
        break // 兜底：把 accumulated 直接吐出去（stop_reason 仍为 pause_turn）
    }
    subBytes := rec.Body.Bytes()

    // 内层 handler 不模拟 web_search，所以 sub 里可能还有原始 tool_use(web_search)
    // 由外层用 transformWebSearchResponse 把它转成 server_tool_use + Exa 结果。
    subTransformed, err := h.transformWebSearchResponse(ctx, subBytes, wsCtx)
    if err != nil {
        break
    }
    sub := parseAnthropicMessage(subTransformed)

    accumulated.content       = append(accumulated.content, sub.content...)
    accumulated.stop_reason   = sub.stop_reason
    accumulated.stop_sequence = sub.stop_sequence
    accumulated.usage         = mergeUsage(accumulated.usage, sub.usage)
    round++
}

writeJSON(client, accumulated)
```

`buildSubBody(originalBody, accumulatedContent) []byte`：

1. 取 `originalBody` 副本（仍带 `web_search_20250305` 或 `web_search_20260209` 工具 + 历史中可能存在的 server_tool_use / web_search_tool_result）。
2. 用 sjson 把 `messages` 替换为 `original.messages + [{role:"assistant", content: accumulatedContent}]`（content 里此时已经是 server_tool_use + web_search_tool_result 形态，因为 accumulated 就是 outer-side 的结果）。
3. **应用 `rewriteWebSearchTools`**：把 tools 数组里的 web_search server tool 替换成 function-tool 形式的 `web_search`。
4. **应用 `rewriteWebSearchHistory`**：把 messages 里所有 assistant 消息中的 server_tool_use → tool_use、web_search_tool_result → 拆分到新的 user 消息中的 tool_result。
5. 返回的字节里**不再含任何 `web_search_2025xxxx` 类型**，所以内层 unified handler 进入 `hasWebSearchTool` 检查时一定返回 false。

要点：

- `accumulated.id` / `accumulated.model` / `accumulated.role` / `accumulated.type` 始终保持外层第一轮的值。客户端看到的是「同一个 message 长出一系列 content blocks」。
- `mergeUsage` 对 `input_tokens`、`output_tokens`、`cache_creation_input_tokens`、`cache_read_input_tokens`、`server_tool_use.web_search_requests` 等字段逐项累加；sub 缺失的字段保留 accumulated 的值。
- 数据库写入路径不变：外层 meta `request` 行的 usage 仍只取**第一轮**的 usage（在 loop 开始前的 `accumulated.usage` 快照写入 DB）；后续轮次由各自 sub-call 的 meta 行各自写入。
- 自调用失败 / 超限 / sub.transform 失败任一命中 → break，客户端拿到的就是当前 accumulated。

## 数据流：流式 (SSE)

最外层 handler 维护循环；transformer 需要扩展两件事：

1. 透传过程中累积「已经向客户端发出去的 content blocks 的最终对象形态」，以便结束时构造下一轮 sub-request body。
2. **吞掉** outer message_delta / message_stop（当本轮 stop_reason 会被改成 `pause_turn` 时），把转发权交回外层 handler——由 handler 决定发起下一轮 self-call 还是把吞下的两帧补发出来兜底。

### transformer 状态机扩展

现有 `ssePassthrough` / `sseBuffering` 不变。新增：

- 构造参数 `holdPauseTurn bool`：**只有最外层第一轮的 transformer 设为 true**。当 `holdPauseTurn == false` 时，transformer 行为与当前实现完全一致（message_delta 正常写出，含 pause_turn 改写）；sub-transformer 永远传 false。
- 字段 `outerContentBlocks []json.RawMessage`：transformer 一边往客户端写一边把已发出去的 content block（最终形态：text / tool_use / server_tool_use + web_search_tool_result / 其它）按输出 index 顺序累积起来；用作构造下一轮 sub-request body 的 assistant content。
- 字段 `blockBuilders map[int64]*streamingBlock`：输出 index → 增量累积器（passthrough block 的 delta 累积）。
- 字段 `outerUsage map[string]any`：累计本轮的 usage（取 `message_start.usage` 和 `message_delta.delta.usage`）。
- 字段 `pendingMessageDelta` / `pendingMessageStop`：**仅 `holdPauseTurn == true` 时使用**。当本轮判定 stop_reason 会被改成 `pause_turn` 时，把这两个事件**只累积进 outerUsage、不写客户端 pipe**，留给上层 loop driver 决定。
- 字段 `webSearchCalls`、`otherToolCalls`：已有，用于判定本轮 stop_reason 是否会被改写成 pause_turn。
- 方法 `(t *webSearchSSETransformer) HasPendingPauseTurn() bool`：暴露给外层 handler 看本轮要不要 loop。
- 方法 `(t *webSearchSSETransformer) PendingFrames() (delta, stop []byte)`：兜底时给 driver 用——取出 pending 的 message_delta（stop_reason 已改为 `pause_turn`）和 message_stop 帧字节。
- 方法 `(t *webSearchSSETransformer) Snapshot() (content []json.RawMessage, usage map[string]any)`：暴露累积状态给 loop driver 构造 sub-request。

### content 累积

每发一个 `content_block_*` 给客户端时，transformer 同步往 `outerContentBlocks` 里塞对应的「完整 block 对象」：

- 普通 passthrough block（text / 非 web_search tool_use / …）：`content_block_start` 时拿到 `content_block` 子对象作为该 index 的占位 builder；`content_block_delta` 累积到对应字段（text 累积进 `text` 字符串、tool_use 的 `partial_json` 累积进字符串 buffer）；`content_block_stop` 时把 builder 序列化进 `outerContentBlocks[outIdx]`（tool_use 的 partial_json 解析成 JSON 对象写回 `input`，没有 delta 则 input 留空对象）。
- 模拟出来的 server_tool_use / web_search_tool_result：transformer 自己构造的完整对象，直接塞进对应 index。

### 触发交接

以下行为**仅当 `holdPauseTurn == true` 时生效**；`holdPauseTurn == false` 的 sub-transformer 走现有逻辑（pause_turn 正常写出 pipe，pipe 正常关闭）。

`message_delta` 事件到达 → 判定 webSearchCalls > 0 && otherToolCalls == 0：

- **是**：把 `delta.usage` 累加进 `outerUsage`；把这一帧暂存 `pendingMessageDelta`（stop_reason 改成 `pause_turn` 的版本），**不写 pipe**。
- **否**：按原逻辑透传（含 usage 累加，便于客户端看到完整 output_tokens）。

`message_stop` 到达 → 如果 `pendingMessageDelta != nil`，同样暂存 `pendingMessageStop`，**不写 pipe**，然后**关闭 pipe writer**——reader 看到 EOF，loop driver 通过 `HasPendingPauseTurn()` 判断下一步。

`message_stop` 到达且无 pending → 透传，关 pipe writer，transformer 自然结束。

两种结局 pipe writer 都关闭。loop driver 只需读 pipe 到 EOF，再检查 `HasPendingPauseTurn()`。不需要 `drained` channel。

### 外层 loop driver（在 unifiedStreamSuccess 流式分支里）

`unifiedStreamSuccess` 在调用 `newWebSearchSSETransformer` 之后、把 `clientReader` 抛给写入循环之前，**不再直接拿 transformer 当 clientReader**。改为：

```go
// pseudocode
loopDriver := &webSearchSSELoopDriver{
    transformer:      newWebSearchSSETransformer(..., holdPauseTurn: true), // 第一轮 outer transformer
    server:           h.Server,
    wsCtx:            a.wsCtx,
    forwardedHeaders: forwardedHeaders(r),
    ctx:              ctx,
}
clientReader = loopDriver  // implements io.ReadCloser
```

`loopDriver.Read` 内部 pipe 输出由几段串接（loop driver 自己有一个 outPipe + goroutine 拼接）：

1. **阶段 A**：把当前 transformer 的 pipe 全读完并写进 outPipe（`io.Copy`，transformer 关闭 pw 后 reader 收到 EOF，copy 返回）。
2. **阶段 B**（决策）：检查 transformer 是否处于 `HasPendingPauseTurn` 状态：
   - 不是 → transformer 已经把 message_delta / message_stop 写过了，loop driver 关 outPipe 结束。
   - 是 → 进入阶段 C 循环。
3. **阶段 C**（self-call 循环）：
   ```
   accumulatedBlocks, accumulatedUsage = transformer.Snapshot()
   indexOffset = len(accumulatedBlocks)
   round = 1
   while round < webSearchMaxRounds:
       subBody = buildSubBody(wsCtx.originalRequestBody, accumulatedBlocks)
       // buildSubBody 已经做过 rewriteWebSearchTools + rewriteWebSearchHistory
       // 所以子 body 里没有 web_search_2025xxxx，内层 handler 不会做模拟。
       subReq = httptest.NewRequestWithContext(ctx, "POST", "/api/picotera/v1/messages", subBody)
       subReq.Header = forwardedHeaders (Accept=text/event-stream 强制流式)
       
       rec = newStreamingResponseRecorder()
       go func() { defer rec.Close(); h.Server.router.ServeHTTP(rec, subReq) }()
       <-rec.StatusReady()
       if rec.StatusCode() != 200:
           fallbackPauseTurn()
           return
       
       // 把内层的 raw SSE 包一层 webSearchSSETransformer（holdPauseTurn=false）：
       // 内层不模拟 web_search，所以它的 SSE 流里仍然带着 tool_use(web_search) 原始 block，
       // 由 sub-transformer 现场转换成 server_tool_use + web_search_tool_result。
       // holdPauseTurn=false 让 sub-transformer 把 pause_turn 帧正常写出 pipe。
       subTransformer = newWebSearchSSETransformer(ctx, rec.Reader(), wsCtx, h, holdPauseTurn=false)
       shouldLoop = forwardSubStream(subTransformer, indexOffset, &accumulatedBlocks, &accumulatedUsage)
       
       round++
       indexOffset = len(accumulatedBlocks)
       if !shouldLoop:
           return  // sub-call 已落地为非 pause_turn 终态，driver 关 pipe 结束
   // 出循环 = 达到上限
   fallbackPauseTurn(accumulatedUsage)
   ```
4. 任一 sub-call 失败（读流错、帧解析错、subTransformer 出错）→ 走 `fallbackPauseTurn` 兜底。

### sub-call SSE 改写规则

sub-transformer 设 `holdPauseTurn=false`，所以它的 pipe 输出里 pause_turn 帧正常存在。`forwardSubStream` 从 sub-transformer 的 pipe 逐帧读取（复用 `parseSSEFrame`，按 `\n\n` 切帧），按下面规则改写后写进 outPipe：

1. **丢弃**子流的 `message_start`——客户端整次响应只有最外层第一个 message_start。
2. **content_block_start / content_block_delta / content_block_stop**：`index` 字段 += `indexOffset` 后写出。
3. **message_delta**：把 `delta.usage` 累加进 `accumulatedUsage`。
   - 若 `delta.stop_reason == "pause_turn"`：**不输出本帧**，return `shouldLoop = true`。
   - 否则：把 `delta.usage` 改写为 `accumulatedUsage`，输出，return `shouldLoop = false`。
4. **message_stop**：和上一步匹配——pause_turn 路径丢弃；落地路径输出后结束本轮 forward。
5. **其它事件**（`ping` 等）：透传。

**content block 累积**：`forwardSubStream` 结束后，从 sub-transformer 调 `Snapshot()` 获取本轮产出的 `content []json.RawMessage`，追加到 driver 的 `accumulatedBlocks` 里。不在 driver 侧维护独立的 block builder——transformer 内部的 `outerContentBlocks` 是唯一权威来源。

### streamingResponseRecorder

`httptest.NewRecorder()` 是缓冲式的，会等子调用 handler 完全 ServeHTTP 返回后才暴露完整 body——对 SSE 不够好。自定义 `streamingResponseRecorder`：实现 `http.ResponseWriter` + `http.Flusher`，内部用 `io.Pipe`，`Write` 直接写 pipe，`Flush` no-op，`Close` 关 pipe writer。loop driver 在 goroutine 里跑 `h.Server.router.ServeHTTP(rec, subReq)`，主流程从 rec 的 pipe reader 实时读帧。

额外暴露 `StatusReady() <-chan struct{}`：`WriteHeader(code)` 时关闭该 channel。loop driver 在创建 sub-transformer 之前先 `<-rec.StatusReady()`，非 200 直接 fallback，避免把 JSON error 喂进 SSE parser。

### 错误降级

任何一处 sub-call 失败（status != 200、读流出错、帧解析错误、未及时收到 message_stop）：

- 已经向客户端发出去的字节不能撤回。
- 把 loop driver 当前手里的 pending `pause_turn` message_delta + message_stop（含本轮累计 usage）写出，结束 driver。
- 不向上层冒泡错误——客户端拿到 `pause_turn` 兜底，仍然是一个语义合法的终止。

## 原始请求体的来源

外层 handler 已经在 retry loop 里持有原始 `body` 变量（即客户端最初的 Anthropic Messages JSON）。在创建 `wsCtx` 时把这份 body 的 **副本**（防止后续 sjson 操作影响）存进 `webSearchContext` 新字段 `originalRequestBody []byte`。非流式 loop driver 用它构造 sub-request body；流式 loop driver 同样从 `wsCtx` 拿到它。

注意：是改写**之前**的 body（包含原始 web_search server tool + 历史 server_tool_use / web_search_tool_result），不是改写之后送给上游的 body。子调用走的是 `/api/picotera/v1/messages`，它会自己重新执行 outbound 改写。

`buildWebSearchSubBody` 内部必须先对 `originalBody` 做 `append([]byte(nil), originalBody...)` 深拷贝再操作，确保多轮调用不会破坏 `wsCtx.originalRequestBody`。

## API key & 鉴权

子调用通过 `Authorization: Bearer <wsCtx.apiKeyToken>` 走完整鉴权链路；与 `callExa` 的实现完全一致。鉴权失败由 unified handler 自己返回 401/403——视作子调用失败，按上节降级处理。

## 配置 & 常量

- 在 `pkg/server/web_search.go` 顶部加：
  ```go
  const webSearchMaxRounds = 10
  ```
- 不引入 `pkg/configx` 字段、HTTP header、ctx 哨兵——内层 unified handler 看到的 sub-body 里就不存在 `web_search_2025xxxx`，自然不会再触发 loop 路径。

## 数据库 & Schema

无变化。所有自循环产生的 LLM 调用通过现有 `/api/picotera/v1/messages` 入口写入 `request` 表，每轮各自走 `insertRequest` + `updateRequestOnComplete`，与外部客户端真的发了 N 次请求毫无区别。外层 meta 行的 usage 仅反映第一轮。

## 测试策略

分两层：

**纯函数 / 小组件单测**（不需要 postgres）：
- `buildWebSearchSubBody`：body 构造正确性 + 不篡改输入。
- `mergeNonStreamRound` / `mergeUsage`：content 追加、stop_reason 替换、usage 累加边界。
- `webSearchSSETransformer` 的 `holdPauseTurn` 行为：true 时 pipe 不含 message_delta/stop 且 `HasPendingPauseTurn()` 为 true；false 时 pause_turn 正常写出。
- `streamingResponseRecorder` 的 `StatusReady` 信号。

**handler 集成测试**（需 postgres test harness，目前不存在，列为 TODO）：
- 两轮循环、深度上限、非 web_search 提前终止、SSE 等价。
- 不需要真正打到 Exa；mock `callExa` 返回固定 ExaSearchResponse。

## 不做的事

- 不在 path-based gateway 上做此循环。
- 不为非 Anthropic Messages 源格式（OpenAI Responses / OpenAI Chat Completions / Gemini GenerateContent）启用循环；这些格式不存在「server-side web search 工具」的概念，原 spec 已说明仅 Anthropic Messages 触发 emulation。
- 不优化跨轮 prompt 缓存命中——子请求自带的 cache_control 仍然来自原始客户端 body，由各轮自然透传。
- 不在 transformer 里做并发——self-call 永远串行，下一轮等上一轮 SSE 流结束再发起。
- 不修改 `request` 表结构、不修改 `Querier` 接口、不加新的 sqlc query。
- 不在前端做任何改动；从 dashboard 看就是一次客户端请求触发了 N 条 request 行（trace 视图天然能展开）。
