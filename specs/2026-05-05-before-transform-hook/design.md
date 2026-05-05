# Design — beforeTransform hook

## 目标

unified gateway 在跨格式转换前新增一个 JS waterfall hook：`beforeTransform`。该 hook 接收当前 attempt 的上下文和当前 outbound 配置，返回或改写 outbound 的 `type` 与 `config`，由返回值决定本次 attempt 使用哪个 axonhub outbound transformer。

本设计删除 unified gateway 里从 merged annotations 读取 `ah.outbound.type`、`ah.outbound.config`、`ah.outbound.fallback` 的路径。annotations 仍然作为 hook ctx 暴露给脚本，但 core gateway 不再把任何 `ah.outbound.*` key 解释成 outbound 行为。

## 执行位置

`beforeTransform` 只在 unified routes 中运行，位置固定在 `rewriteRequest` 之后、`BridgeRequest` 之前：

1. `beforeRequest` 选择 attempt 行为和 upstream model。
2. gateway 构造 source-format upstream request。
3. `rewriteRequest` 改写 source-format pending request。
4. `beforeTransform` 决定 outbound profile。
5. `BridgeRequest` 在需要跨格式时使用该 profile 生成 upstream-format body。
6. response bridge 使用同一个 profile 解析 upstream response 或 stream。

identity path 不运行 transformer。`srcFormat == upFormat` 时 `BridgeRequest`、`BridgeNonStream`、`BridgeStream` 继续 byte-for-byte passthrough；`beforeTransform` 仍然可以运行并产生日志，但 profile 不影响 identity bridge。这样 hook 的时序稳定，且不会改变 1:1 上游请求。

## JS ctx 与 input

`beforeTransform` 使用与 `rewriteRequest` 相同的 attempt 语义，并增加 transform 相关字段：

- `endpoint`
- `model`
- `provider`
- `mpe`
- `currentRetryCount`
- `totalAttemptCount`
- `clientRequest`
- `pendingRequest`
- `apiKey`
- `annotations`
- `sourceFormat`
- `upstreamFormat`
- `stream`

hook 的 waterfall input 是当前 outbound profile：

```js
{
  type: "openai",
  config: {}
}
```

初始 `type` 由本次 attempt 的 upstream endpoint format 映射而来，而不是空字符串：

| upstream format | initial outbound type |
| --- | --- |
| `anthropicMessages` | `anthropic` |
| `openaiChatCompletions` | `openai` |
| `openaiResponses` | `openaiResponses` |
| `geminiGenerateContent` | `gemini` |
| `geminiStreamGenerateContent` | `gemini` |

`config` 是一个 object，默认 `{}`。脚本可以返回完整对象，也可以原地修改 input 后返回它；返回 `undefined` 表示沿用上一个 tap 的值。

示例：

```js
picotera.hooks.beforeTransform.tap("openrouter", function (ctx, outbound) {
  if (ctx.provider.name !== "openrouter") return
  outbound.type = "openrouter"
  outbound.config = {}
  return outbound
})
```

## 严格校验

Go 侧对 hook 返回值严格校验：

- 返回值必须是 object。
- `type` 必须是 string；缺失表示沿用进入该 hook 的当前 type。
- `config` 必须是 object；缺失等同 `{}`。
- 非 object config、数组 config、非 string type 都返回 hook 错误。
- 不 trim、不 lower-case、不猜测 type；脚本必须返回精确支持值。

支持的 `type` 是 canonical outbound type 加 provider-specific outbound type：

- `anthropic`
- `openai`
- `openaiResponses`
- `gemini`
- `openrouter`
- `deepseek`
- `fireworks`

`anthropic`、`openai`、`openaiResponses`、`gemini` 使用当前默认 transformer。它们必须与 upstream endpoint format 兼容：Anthropic endpoint 只接受 `anthropic`，OpenAI Chat Completions endpoint 接受 `openai` 以及 provider-specific OpenAI-compatible type，OpenAI Responses endpoint 只接受 `openaiResponses`，Gemini endpoint 只接受 `gemini`。

`openrouter`、`deepseek`、`fireworks` 只允许用于 `upstreamFormat == openaiChatCompletions`。其它 format 返回 attempt 失败，并进入现有 retry 流程。未知 type 也按 attempt 失败处理。

## Go profile

`pkg/llmbridge.OutboundProfile` 改为 hook 输出模型：

```go
type OutboundProfile struct {
    Type   string
    Config map[string]any
}
```

新增 `DefaultOutboundProfileForFormat(Format) (OutboundProfile, error)`，用于把 upstream format 映射成初始 profile。删除 `OutboundProfileFromAnnotations` 和 `Fallback` 字段。`Config` 在传给 axonhub config 前经 JSON round-trip 解到对应 config struct，并使用 `json.Decoder.DisallowUnknownFields()`。这样仍然使用 axonhub config 的 JSON tag 字段名，同时让 hook 在 JS 侧使用普通 object 而不是手写 JSON string。

picotera 在解码前后强制写入 placeholder `BaseURL` 与 `APIKeyProvider`。真实 upstream URL、credentials 和 send resolver 仍然来自 provider endpoint，不允许 hook 或 axonhub config 接管。

## 错误行为

`beforeTransform` JS 执行错误或返回结构校验错误属于 hook 错误，直接按现有 hook failure 路径结束当前 unified 请求。原因是脚本自身返回了非法 hook contract，继续 retry 只会重复同一 hook 错误。

profile type 不支持、type 与 upstream format 不兼容、config 字段未知、config 无法解码、transformer 构造失败属于本次 attempt 的 bridge 错误。gateway 记录 upstream request row 的失败信息，更新 `lastError`，继续尝试下一个候选或下一次 retry。

## 不变项

- 不改 DB schema、REST API、dashboard UI。
- 不改 annotations 合并与 JS ctx 暴露；脚本仍可自行读取 `ctx.annotations` 作为决策输入。
- 不新增 annotation compatibility path，不继续读取 `ah.outbound.*`。
- 不改 path-based gateway。
- 不新增第三方依赖。
