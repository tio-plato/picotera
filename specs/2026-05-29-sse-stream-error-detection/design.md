# 设计：SSE 流内错误识别

## 背景

网关在上游返回 HTTP 200 后会把响应体边读边回写客户端。部分上游（典型为 Anthropic Messages）会在 200 流中以一个 `error` 事件的形式报告失败，例如：

```
data: {"type":"error","error":{"type":"server_error","message":"upstream connect error ...","param":null},"sequence_number":3}
```

此时网关把请求记为成功（`completed`），导致监控与请求详情误判。本设计在不影响字节回写的前提下，识别 SSE data 块中的 `error.message`，把对应的 meta 行与上游行记为失败。

## 关键观察

两条网关成功路径都把上游响应统一经过同一个 `ResponseExtractor`（`pkg/server/response_extractor.go`）做边流式边解析：

- 路径网关：`streamSuccess` → `pipePathResponse` 构造 `NewResponseExtractor(...)`，循环结束后由 `completeGatewaySuccess` 写完成行。
- 统一网关：`unifiedStreamSuccess` 内联构造 `NewResponseExtractor(...)` 包裹**上游原生格式**字节，循环结束后内联写完成行。

`ResponseExtractor` 仅在 SSE 模式（`Content-Type` 含 `text/event-stream`）下逐事件调用 `processSSEEvent`。这天然满足「仅检测 SSE」的决策——把错误识别加在这里，非流式 JSON 路径不受影响。

`ResponseExtractor` 是只读透传（`Read` 转发原始字节，仅做旁路解析），在此处加错误识别**不会改变回写给客户端的字节**，因此客户端仍会原样收到上游的 error 事件。

对统一网关的桥接场景：extractor 包裹的是上游原生字节，所以识别的是上游格式（如 Anthropic 上游 + OpenAI 源）的 error 事件，正是错误的来源处。`error.message` 路径在 Anthropic / OpenAI Chat / Gemini 原生 error 形态中一致。

## 设计

### 1. `ResponseExtractor` 增加错误捕获

新增字段 `streamError string` 与访问器 `StreamError() string`（空串表示无错误）。

在 `processSSEEvent` 解析出 `payload` 后（跳过 `[DONE]` 之后、现有 metric 提取旁），调用新方法 `detectStreamError(payload)`：

- `gjson.Get(payload, "error.message")`，若存在且为非空字符串，且 `streamError` 尚未设置，则记录之（**首个**错误为准）。

错误识别独立于 token / TTFT 提取——已经从流中提取到的 metrics 仍照常记录。

### 2. 新增 finish_reason 常量

`pkg/db/request_constants.go` 增加：

```go
FinishReasonStreamError = 6
```

无需迁移：`request.finish_reason` 已是 `INTEGER` 列，只是新增取值语义。

### 3. 完成路径分支

两条路径在写「完成」行时统一遵循：

- 若 `extractor.StreamError() != ""`：
  - `Status = RequestStatusFailed`
  - `ErrorMessage = <error.message>`
  - `FinishReason = FinishReasonStreamError`
  - `StatusCode` 仍为真实上游 HTTP 码（200）
  - token / 成本 / TTFT 字段照常写入
- 否则维持现有 `completed` 行为。

meta 行与上游行采用相同判定（两者都标记为错误）。

**路径网关**：`completeGatewaySuccess` 新增参数 `streamErr string`，内部对两行做上述分支；调用处从 extractor 取值传入。

**统一网关**：`unifiedStreamSuccess` 在 `m := extractor.Metrics()` 处取 `streamErr := extractor.StreamError()`，对内联的两个 `UpdateRequestOnCompleteParams` 做同样分支。

### 4. 仪表盘 finish_reason 文案

`dashboard/src/components/RequestDetailsContent.vue` 的 `finishReasonLabel` 增加 `case 6 → '流式错误'`。`finishReasonVariant` 对 6 返回非 `ok` 变体（沿用 `default`）。失败状态本身已由 `status` 徽标体现红色，本项仅补充原因文案。`finishReason` 已作为 `number` 经 OpenAPI 暴露，无需改动契约或重生成类型。

## 不做的事

- 不处理非流式 JSON 200-with-error（按决策）。
- 不处理 `error.message` 以外的 error 形态（如 OpenAI Responses 顶层 message）。
- 不引入重试：错误发生时响应体已开始回写，无法重试，仅记录。
- 不改 OpenAPI 契约 / sqlc 查询：`status` / `error_message` / `finish_reason` 列与 `UpdateRequestOnComplete` 查询已存在。
