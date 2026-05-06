# SSE/JSONL Stream 聚合设计

## 结论

采用后端聚合。PicoTera 在响应完成后用当前已经引入的 `github.com/looplj/axonhub/llm` 聚合器解析已捕获的 stream chunks，把聚合后的 JSON 写进 response artifact。Dashboard 停止维护 provider 协议级的手写聚合逻辑，只展示后端给出的聚合 JSON，并保留 Raw 与 Events 视图用于诊断。

聚合 JSON 的格式是该响应 artifact 所代表协议的 non-streaming response 格式：

| Format | 聚合 JSON |
| --- | --- |
| OpenAI Chat Completions | `chat.completion` JSON |
| OpenAI Responses | `response` JSON |
| Anthropic Messages | `message` JSON |
| Gemini StreamGenerateContent | `GenerateContentResponse` JSON |

## 为什么不用前端 Vercel AI SDK 聚合 provider SSE

Vercel AI SDK 的公开前端协议面向 AI SDK 自己的 UI message stream 和 data stream，不是通用的 OpenAI / Anthropic / Gemini provider SSE 聚合器。它适合消费 `toUIMessageStreamResponse()` 产出的 UI stream，或者在应用层维护 `UIMessage` / `ModelMessage`，但不能直接替代当前 artifact viewer 对上游原始 SSE 的协议级恢复。

把 provider SSE 转成 AI SDK UI message stream 也需要先正确理解 provider SSE。这个解析步骤仍然要放在后端或另一个 transformer 中完成，因此本方案直接复用 axonhub 已有 transformer 和聚合器。

## axonhub 能力边界

当前仓库已经在 `pkg/llmbridge/` 使用 axonhub 做 unified gateway 的请求和响应格式转换。axonhub 本地依赖中已经提供以下 stream aggregation 能力：

- `transformer/openai.AggregateStreamChunks`
- `transformer/openai/responses.AggregateStreamChunks`
- `transformer/anthropic.AggregateStreamChunks`
- `transformer/gemini.AggregateStreamChunks`

这些聚合器由 axonhub 的 transformer 测试覆盖，支持比当前前端手写逻辑更完整的协议细节，包括 tool call argument 拼接、reasoning 内容、Responses API output item / content part / function call events、Anthropic content block indexing、Gemini thought/tool parts 和 usage metadata。PicoTera 补充 stream wire decoder：SSE 交给 axonhub 默认 decoder，Gemini JSONL/NDJSON 在 `pkg/llmbridge` 内解析成 axonhub `StreamEvent`。

PicoTera 通过 `pkg/llmbridge` 暴露自己的小接口，不让 `pkg/server` 或 dashboard 直接依赖 axonhub transformer 包。

## Artifact 数据模型

扩展 `artifacts.Payload`：

```go
type Payload struct {
    Method       string              `json:"method,omitempty"`
    URL          string              `json:"url,omitempty"`
    StatusCode   int                 `json:"statusCode,omitempty"`
    Headers      http.Header         `json:"headers"`
    Body         string              `json:"body"`
    BodyEncoding string              `json:"bodyEncoding"`
    Aggregated   *AggregatedResponse `json:"aggregated,omitempty"`
    Logs         []LogEntry          `json:"logs,omitempty"`
}

type AggregatedResponse struct {
    Format       string          `json:"format"`
    Body         json.RawMessage `json:"body"`
    BodyEncoding string          `json:"bodyEncoding"`
    Error        string          `json:"error,omitempty"`
}
```

`AggregatedResponse.Format` 使用 PicoTera 的格式名：

- `openaiChatCompletions`
- `openaiResponses`
- `anthropicMessages`
- `geminiStreamGenerateContent`

`AggregatedResponse.BodyEncoding` 固定为 `json`。`Body` 存储聚合后的 JSON object bytes。聚合失败时不写半成品 JSON，只写 `format` 与 `error`，Dashboard 显示失败状态并保留 Raw / Events。

该扩展是 artifact JSON 的向前扩展；旧 artifact 没有 `aggregated` 字段时，Dashboard 显示“无后端聚合结果”，不再尝试协议级手写聚合。

## 后端聚合流程

新增 `pkg/llmbridge.AggregateStream`：

```go
func AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error)
```

行为：

1. 调用方先用 `(format, contentType)` 判断是否需要 stream 聚合；普通 JSON 响应不调用该函数。
2. 根据 `contentType` 选择 stream decoder，把 wire body 解成 `[]*httpclient.StreamEvent`。
3. `text/event-stream` 使用 axonhub `httpclient.NewDefaultSSEDecoder`。
4. `application/jsonl`、`application/x-ndjson`、`application/jsonlines` 和 `application/ndjson` 使用 PicoTera 在 `pkg/llmbridge` 内实现的严格 JSONL decoder：逐行读取，每个非空行必须是完整 JSON object，生成 `StreamEvent{Data: line}`。
5. Gemini upstream 若返回 `application/json` 但 `format == FormatGeminiStreamGenerateContent`，按 JSONL decoder 处理，因为该 endpoint 仍然是 stream format；每行仍必须是完整 JSON object。
6. 未识别的 stream media type 返回明确错误并写入 `aggregated.error`。
7. 使用 `outboundFor(format, profile).AggregateStreamChunks(ctx, req, chunks)` 聚合。
8. 返回聚合后的 JSON bytes。

stream 聚合判定表：

| Format | Content-Type | 行为 |
| --- | --- | --- |
| `FormatAnthropicMessages` | `text/event-stream` | SSE decoder + Anthropic 聚合 |
| `FormatOpenAIChatCompletions` | `text/event-stream` | SSE decoder + OpenAI Chat 聚合 |
| `FormatOpenAIResponses` | `text/event-stream` | SSE decoder + OpenAI Responses 聚合 |
| `FormatGeminiStreamGenerateContent` | `text/event-stream` | SSE decoder + Gemini 聚合 |
| `FormatGeminiStreamGenerateContent` | `application/jsonl` / `application/x-ndjson` / `application/jsonlines` / `application/ndjson` / `application/json` | JSONL decoder + Gemini 聚合 |
| `FormatGeminiGenerateContent` | 任意 | 不聚合 |
| 其它 format 或普通 JSON media type | 任意 | 不聚合 |

`profile` 沿用 unified bridge 已有的 `OutboundProfile`。路径网关没有 `beforeTransform` profile 时使用 `DefaultOutboundProfileForFormat(format)`。这样 OpenRouter / DeepSeek / Fireworks 等 OpenAI-compatible profile 的聚合可以复用对应 outbound transformer 的 provider-specific 聚合行为。

## Format 来源

路径网关的 endpoint row 已经带 `endpoint.EndpointType`。响应 artifact 聚合时按 endpoint type 映射到 `llmbridge.Format`：

- `AnthropicMessages` -> `FormatAnthropicMessages`
- `OpenAIChatCompletions` -> `FormatOpenAIChatCompletions`
- `OpenAIResponses` -> `FormatOpenAIResponses`
- `GeminiStreamGenerateContent` -> `FormatGeminiStreamGenerateContent`

`GeminiGenerateContent` 是非流式 endpoint type，不映射到 stream aggregation format。非 generation endpoint type 不聚合。

Unified gateway 已经明确区分 meta response artifact 与 upstream response artifact：

- upstream artifact 使用 `upFormat` 和 upstream bytes 聚合，表示上游原始协议的 non-streaming JSON。
- meta artifact 使用 `srcFormat` 和写给 client 的 bytes 聚合，表示客户端调用协议的 non-streaming JSON。

当 unified bridge 使用 non-stream 响应时不会进入 stream 聚合；该响应本身已经是 JSON，Dashboard 继续通过普通 JSON 视图展示。

## Dashboard 行为

Dashboard 删除 `aggregateOpenAIChat`、`aggregateOpenAIResponses`、`aggregateAnthropic` 这些协议级聚合函数。`useSSEParser.ts` 保留通用 SSE event display parser 和 markdown 内容提取所需的轻量提取逻辑。

`ResponseArtifactView.vue` 的聚合 tab 改为读取 `payload.aggregated`：

- `payload.aggregated.body` 存在时用 `JsonArtifactViewer` 展示。
- `payload.aggregated.error` 存在时显示错误。
- `payload.aggregated` 缺失时显示“无后端聚合结果”。

渲染 tab 的内容提取优先使用聚合 JSON：

- OpenAI Chat: `choices[0].message.content` 和 `reasoning_content`
- OpenAI Responses: `output[].content[].text` 和 reasoning summary
- Anthropic: `content[].text` 和 `content[].thinking`
- Gemini stream: `candidates[].content.parts[].text`，`thought: true` 归为 thinking

没有聚合 JSON 时，渲染 tab 显示无可渲染内容。Raw 与 Events 仍然用于诊断旧 artifact 或聚合失败的响应。

## 失败处理

聚合失败不影响 gateway 对客户端的响应，不改变 request row 的完成状态，也不触发重试。失败只记录在 artifact 的 `aggregated.error` 和 server log 中。

聚合仅在完整响应 bytes 已经捕获后执行，因此不会增加首 token 延迟，不改变流式转发行为。失败必须可观测，但不能影响主链路。

## 依赖

不新增前端依赖。继续使用后端已有的 `github.com/looplj/axonhub/llm`，并将聚合封装在 `pkg/llmbridge` 内。

不引入 Vercel AI SDK 作为 dashboard dependency。后续如果需要提供 AI SDK UI stream 给客户端，应作为新的 gateway output format 或 transformer profile 设计，而不是 artifact viewer 的聚合器。

## 验证

后端测试覆盖：

- OpenAI Chat SSE 聚合为 `object: "chat.completion"`，content、tool_calls、usage 正确。
- OpenAI Responses SSE 聚合为 `object: "response"`，output item、reasoning summary、usage 正确。
- Anthropic SSE 聚合为 `type: "message"`，content blocks、tool use input、usage 正确。
- Gemini streamGenerateContent SSE 和 JSONL/NDJSON 聚合为 Gemini non-stream response，thought text、function calls、usage metadata 正确。
- malformed SSE、malformed JSONL 或不支持 format 时 artifact 写入 `aggregated.error`，原始 body 不变。

前端测试和构建覆盖：

- artifact 有 `aggregated.body` 时聚合 tab 展示 JSON tree。
- artifact 有 `aggregated.error` 时展示错误。
- 旧 artifact 无 `aggregated` 时展示无后端聚合结果。
- `pnpm --dir dashboard type-check`
- `pnpm --dir dashboard build`
