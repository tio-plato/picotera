# 设计：Gemini 响应 token 用量提取

## 根因分析（已实验验证）

### 记录侧 token 的来源

网关记录到 `request` 行的 token 列（`input_tokens` / `output_tokens` / `cache_read_tokens` / `cache_write_tokens` / TTFT）来自 `pkg/server/response_extractor.go` 的 `ResponseExtractor`：

- path 路由：`pipePathResponse`（`gateway_flow_success.go:182`）用 extractor 包裹**上游原始响应**。
- unified 路由：`unifiedStreamSuccess`（`gateway_unified_helpers.go:444`）同样用 extractor 读**上游原始格式**字节，注释明确："metrics 来自上游 native 格式，与是否桥接无关"。unified 的流式与非流式都走这一个入口（`handle_unified_gateway.go:55` 的 `HandleSuccess` 恒调用 `unifiedStreamSuccess`），都用 `extractor.Metrics()`（`gateway_unified_helpers.go:571`）。

### `ResponseExtractor` 缺 Gemini 支持

`response_extractor.go` 只实现了 OpenAI Chat、OpenAI Responses、Anthropic 三种格式：

- SSE：`extractOpenAISSE` / `extractOpenAIResponsesSSE` / `extractAnthropicSSE`，均读 `usage.*`。
- JSON：`extractJSONMetrics`，依次尝试 `usage.prompt_tokens` / `usage.input_tokens` / `usage.cache_*`。
- 模型推断 `inferModelField` 只看 `model` / `message.model`。

Gemini 的 token 在 `usageMetadata.{promptTokenCount,candidatesTokenCount,totalTokenCount,cachedContentTokenCount,thoughtsTokenCount}`，模型名在 `modelVersion`，都不被任何现有分支命中。实验直接喂 Gemini SSE 与 JSON 给 extractor，`InputTokens`/`OutputTokens`/`TTFTMs`/`InferredModel` 全为 `nil`/空。

**这同时是问题 1 与问题 2 记录侧的根因。**

### bridge 输出本身正确（无需改动）

实验用本节给出的真实 Gemini SSE，经 `BridgeStream` 转换到三种源格式，客户端收到的 usage 均正确：

- → OpenAI Chat：末块 `usage:{prompt_tokens:8,completion_tokens:11,total_tokens:19}`。
- → OpenAI Responses：`response.completed` 的 `usage:{input_tokens:8,output_tokens:11,total_tokens:19}`。
- → Anthropic：`message_delta` 的 `usage:{input_tokens:8,output_tokens:11}`。
- 非流式三种源格式 usage 同样正确；`AggregateStream` 聚合后的 Gemini 体 `totalTokenCount=19` 正确。

axonhub 的 Gemini outbound 在带 `finishReason` 的末块上调用 `convertGeminiToLLMResponseWithState`，`resp.Usage` 被正确填充（`outbound_convert.go:662`），下游各源格式 inbound 把 usage 透传到末事件。因此**桥接链路不需要修改**。

### 旁注：无 `alt=sse` 的 JSON 数组流

实验显示上游若以 JSON 数组（无 `alt=sse`）返回流，bridge 仅输出 `data: [DONE]`，内容与 usage 全丢。该问题由既有的 `2026-06-25-unified-gemini-alt-sse`（注入 `alt=sse`）解决，**不属于本次范围**。

## 方案

仅改 `pkg/server/response_extractor.go`，为其增加 Gemini 格式支持。不改 bridge、不改 axonhub。

### Token 语义（与 axonhub `convertToLLMUsage` 及 picotera 既有约定对齐）

Gemini `promptTokenCount` 含缓存命中部分；picotera 既有约定（`setOpenAIInputTokens`）是 `InputTokens = 总输入 - 缓存`，缓存单列。因此 Gemini 映射为：

| picotera 指标 | Gemini 字段 |
| --- | --- |
| `InputTokens` | `promptTokenCount - cachedContentTokenCount` |
| `OutputTokens` | `candidatesTokenCount + thoughtsTokenCount` |
| `CacheReadTokens` | `cachedContentTokenCount`（存在且 > 0 时） |
| `InferredModel` | `modelVersion`（来源 `response`） |
| `TTFTMs` | 首个含 `candidates.0.content.parts` 的 SSE 块 |

Gemini 无缓存写入概念，`CacheWriteTokens` / `CacheWrite1HTokens` 不设置。

### SSE 提取（`extractGeminiSSE`）

新增方法，在 `processSSEEvent` 中与其它 `extract*SSE` 并列调用：

- TTFT：未记录且 `candidates.0.content.parts` 存在（含文本或任意 part）时记录。
- Usage：`usageMetadata` 存在时，仅对**实际存在**的字段赋值（`gjson` `Exists()` 判定）。Gemini 早期块的 `usageMetadata` 只有 `trafficType` 没有 token 计数，必须跳过、不得写 0；末块带计数时覆盖写入（与 Anthropic `message_delta` 覆盖 `output_tokens` 的既有行为一致，末值生效）。

### JSON 提取（`extractJSONMetrics` 增加 Gemini 分支）

在现有 OpenAI/Anthropic 分支之后，对仍为 `nil` 的指标尝试 `usageMetadata.*`：

- `InputTokens == nil` 时读 `usageMetadata.promptTokenCount`（减 `cachedContentTokenCount`）。
- `OutputTokens == nil` 时读 `usageMetadata.candidatesTokenCount`（加 `thoughtsTokenCount`）。
- `CacheReadTokens == nil` 时读 `usageMetadata.cachedContentTokenCount`。

保持"已被前面格式命中则不覆盖"的既有惯例，确保对 OpenAI/Anthropic 响应零行为变化。

### 模型推断（`inferModelField` 增加 `modelVersion`）

把 `modelVersion` 加入候选路径：`["model", "message.model", "modelVersion"]`，首个非空命中并锁定。对非 Gemini 响应无影响（它们没有 `modelVersion`）。

### 错误检测

`detectStreamError` 已覆盖 Gemini 的 `error.message` 形态，无需改动。

## 测试

新增 Gemini 用例到 `pkg/server/response_extractor_test.go`：

1. `TestResponseExtractor_SSE_Gemini_Usage`：用本设计的三段式 SSE（前两块 `usageMetadata` 无计数、末块有计数 + `finishReason`），断言 `InputTokens=8`、`OutputTokens=11`、`TTFTMs` 已记录、`InferredModel="google/gemini-2.5-flash-lite"`。
2. `TestResponseExtractor_SSE_Gemini_EarlyUsageMetadataIgnored`：仅含前两块（无计数），断言 token 仍为 `nil`（不被写 0）。
3. `TestResponseExtractor_JSON_Gemini`：非流式体，断言 `InputTokens=8`、`OutputTokens=10`、`InferredModel` 正确。
4. `TestResponseExtractor_SSE_Gemini_CachedAndThoughts`：构造含 `cachedContentTokenCount` 与 `thoughtsTokenCount` 的末块，断言 `InputTokens = prompt - cached`、`OutputTokens = candidates + thoughts`、`CacheReadTokens = cached`。

这些用例在修复前应失败（token 为 `nil`），修复后通过。既有 OpenAI/Anthropic 用例必须保持通过，验证无回归。
