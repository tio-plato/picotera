# API — SSE/JSONL Stream 聚合 Artifact

本方案不新增管理 API endpoint，不修改 OpenAPI contract。变更只发生在 presigned artifact JSON 的响应文档结构中。

## Response Artifact JSON

现有 response artifact：

```json
{
  "statusCode": 200,
  "headers": {
    "Content-Type": ["text/event-stream"]
  },
  "body": "data: {...}\n\n",
  "bodyEncoding": "utf8"
}
```

扩展后，已聚合的 stream response artifact 包含 `aggregated`：

```json
{
  "statusCode": 200,
  "headers": {
    "Content-Type": ["text/event-stream"]
  },
  "body": "data: {...}\n\n",
  "bodyEncoding": "utf8",
  "aggregated": {
    "format": "openaiChatCompletions",
    "bodyEncoding": "json",
    "body": {
      "id": "chatcmpl_...",
      "object": "chat.completion",
      "created": 1760000000,
      "model": "gpt-4.1",
      "choices": [
        {
          "index": 0,
          "message": {
            "role": "assistant",
            "content": "hello"
          },
          "finish_reason": "stop"
        }
      ],
      "usage": {
        "prompt_tokens": 10,
        "completion_tokens": 4,
        "total_tokens": 14
      }
    }
  }
}
```

聚合失败时：

```json
{
  "statusCode": 200,
  "headers": {
    "Content-Type": ["text/event-stream"]
  },
  "body": "data: {...}\n\n",
  "bodyEncoding": "utf8",
  "aggregated": {
    "format": "openaiResponses",
    "bodyEncoding": "json",
    "error": "llmbridge: aggregate stream: empty stream chunks"
  }
}
```

## TypeScript Shape

Dashboard 本地 artifact payload 类型扩展为：

```ts
type ArtifactPayload = {
  method?: string
  url?: string
  statusCode?: number
  headers?: Record<string, string[]>
  body?: string
  bodyEncoding?: 'utf8' | 'base64'
  aggregated?: AggregatedResponse
  logs?: LogEntry[]
}

type AggregatedResponse = {
  format:
    | 'openaiChatCompletions'
    | 'openaiResponses'
    | 'anthropicMessages'
    | 'geminiStreamGenerateContent'
  bodyEncoding: 'json'
  body?: unknown
  error?: string
}
```

## Format Semantics

`aggregated.body` 的 JSON shape 由 `format` 决定：

| `format` | `aggregated.body` |
| --- | --- |
| `openaiChatCompletions` | OpenAI Chat Completions non-streaming response |
| `openaiResponses` | OpenAI Responses non-streaming response |
| `anthropicMessages` | Anthropic Messages non-streaming response |
| `geminiStreamGenerateContent` | Gemini GenerateContent non-streaming response |

`geminiGenerateContent` 不出现在 `aggregated.format` 中。该 endpoint type 是非流式响应，artifact body 本身是 Gemini GenerateContent JSON。

## 旧 Artifact 行为

旧 artifact 没有 `aggregated` 字段。Dashboard 必须正常展示 Raw、Events 和 JSON 视图，并在聚合 tab 中显示无后端聚合结果。

`aggregated` 字段只出现在完成聚合的 response artifact 中，不改变 artifact object key、presigned URL、压缩方式、request row schema 或 management API response schema。
