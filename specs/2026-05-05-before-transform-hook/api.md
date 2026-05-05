# API — beforeTransform hook

## JavaScript SDK

`picotera.hooks` 新增：

```js
picotera.hooks.beforeTransform
```

用法：

```js
picotera.hooks.beforeTransform.tap("name", function (ctx, outbound) {
  outbound.type = "openrouter"
  outbound.config = {}
  return outbound
}, 100)
```

### Hook input

`outbound` 初始值：

```ts
type OutboundProfile = {
  type: "anthropic" | "openai" | "openaiResponses" | "gemini" | "openrouter" | "deepseek" | "fireworks" | string
  config: Record<string, unknown>
}
```

初始 `type` 由本次 attempt 的 upstream endpoint format 映射而来：

| upstreamFormat | initial type |
| --- | --- |
| `anthropicMessages` | `anthropic` |
| `openaiChatCompletions` | `openai` |
| `openaiResponses` | `openaiResponses` |
| `geminiGenerateContent` | `gemini` |
| `geminiStreamGenerateContent` | `gemini` |

脚本可以返回 canonical type，也可以返回 provider-specific type。`type` 必须是当前 Go 侧支持的精确字符串。

### Hook context

```ts
type BeforeTransformInput = {
  endpoint: unknown
  model: ModelSummary | null
  provider: ProviderSummary
  mpe: CandidateMPE
  currentRetryCount: number
  totalAttemptCount: number
  clientRequest: RequestShape
  pendingRequest: PendingRequestShape
  apiKey: ApiKeySummary | null
  annotations: Record<string, string>
  sourceFormat: "anthropicMessages" | "openaiChatCompletions" | "openaiResponses" | "geminiGenerateContent" | "geminiStreamGenerateContent"
  upstreamFormat: "anthropicMessages" | "openaiChatCompletions" | "openaiResponses" | "geminiGenerateContent" | "geminiStreamGenerateContent"
  stream: boolean
}
```

`pendingRequest` 是 `rewriteRequest` 后、`BridgeRequest` 前的 source-format request。

### Return contract

每个 tap 可以：

- 返回 `undefined`：沿用当前 outbound。
- 返回同一个 outbound object：使用修改后的值。
- 返回新 object：替换当前 outbound。

最终返回值必须满足：

```ts
type BeforeTransformResult = {
  type?: string
  config?: Record<string, unknown>
}
```

缺失 `type` 沿用进入 hook 的当前 type。缺失 `config` 视为 `{}`。`type` 非 string、`config` 非 plain JSON object 或数组会触发 hook 错误。

## Go: `pkg/jsx`

新增类型：

```go
type OutboundProfile struct {
    Type   string         `json:"type"`
    Config map[string]any `json:"config"`
}

type BeforeTransformInput struct {
    Endpoint          any                 `json:"endpoint"`
    Model             *ModelSummary       `json:"model"`
    Provider          ProviderSummary     `json:"provider"`
    MPE               CandidateMPE        `json:"mpe"`
    CurrentRetryCount int                 `json:"currentRetryCount"`
    TotalAttemptCount int                 `json:"totalAttemptCount"`
    ClientRequest     RequestShape        `json:"clientRequest"`
    PendingRequest    PendingRequestShape `json:"pendingRequest"`
    ApiKey            *ApiKeySummary      `json:"apiKey"`
    Annotations       map[string]string   `json:"annotations"`
    SourceFormat      string              `json:"sourceFormat"`
    UpstreamFormat    string              `json:"upstreamFormat"`
    Stream            bool                `json:"stream"`
}
```

新增 session 方法：

```go
func (s *Session) RunBeforeTransformHook(in BeforeTransformInput, initial OutboundProfile) (OutboundProfile, error)
```

该方法执行 `picotera.hooks.beforeTransform.runWaterfall(ctx, initial)`，严格解码最终结果。

## Go: `pkg/llmbridge`

`OutboundProfile` 改为：

```go
type OutboundProfile struct {
    Type   string
    Config map[string]any
}
```

删除：

```go
func OutboundProfileFromAnnotations(map[string]string) OutboundProfile
```

删除 `Fallback` 字段和 fallback default 行为。

新增：

```go
func DefaultOutboundProfileForFormat(f Format) (OutboundProfile, error)
```

映射关系：

| Format | Type |
| --- | --- |
| `FormatAnthropicMessages` | `anthropic` |
| `FormatOpenAIChatCompletions` | `openai` |
| `FormatOpenAIResponses` | `openaiResponses` |
| `FormatGeminiGenerateContent` | `gemini` |
| `FormatGeminiStreamGenerateContent` | `gemini` |

现有 bridge 函数继续接收 `OutboundProfile`：

```go
func BridgeRequest(ctx context.Context, src Format, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error)

func BridgeNonStream(ctx context.Context, src Format, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error)

func BridgeStream(ctx context.Context, src Format, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error)
```

`profile.Config` 用 JSON object 语义解到具体 transformer config。未知字段返回错误。

## Removed behavior

以下 annotations key 不再由 Go core 读取：

- `ah.outbound.type`
- `ah.outbound.config`
- `ah.outbound.fallback`

脚本可以继续把这些 key 当普通 annotations 读取，但必须显式在 `beforeTransform` 中转成 outbound profile。
