# Design — Unified outbound annotations

## 目标

unified gateway 在执行 outbound transform 时读取候选 provider 的合并 annotations，并用 `ah.outbound.type` 选择 axonhub outbound transformer：

- `openrouter` → `github.com/looplj/axonhub/llm/transformer/openrouter`
- `deepseek` → `github.com/looplj/axonhub/llm/transformer/deepseek`
- `fireworks` → `github.com/looplj/axonhub/llm/transformer/fireworks`

同时读取 `ah.outbound.config`。该值存在时必须按 JSON object 解析，并合并到对应 transformer 的 Config。合并优先级为默认占位配置 < `ah.outbound.config` < picotera 强制字段；picotera 强制字段包括占位 `BaseURL` 和 `APIKeyProvider`，确保 llmbridge 仍只产出 body/Content-Type，不接管 picotera 的真实 URL 和鉴权。

annotations 来源沿用已有合并结果，优先级是 `model < provider < provider models < api key`。本规格的 TODO 原文未列 api key，但代码当前已经把 api key 纳入合并；unified outbound 必须消费这份最终合并 map，保持与 hook ctx 一致。

## 现有基础

`pkg/server/handle_unified_gateway.go` 已经在 candidate sidecar 中保存每个候选的 `annotations map[string]string`。这份 map 已经由 `pkg/annotations` 合并完成，并且会跟随 JS sort/beforeRequest/rewriteRequest 之后的候选选择进入 retry loop。

`pkg/llmbridge/outboundFor` 目前只按 `llmbridge.Format` 返回固定 transformer：

- Anthropic → `anthropic.NewOutboundTransformerWithConfig`
- OpenAI chat → `openai.NewOutboundTransformerWithConfig`
- OpenAI responses → `openai/responses.NewOutboundTransformerWithConfig`
- Gemini → `gemini.NewOutboundTransformerWithConfig`

本次改造只扩展 outbound 选择，不改变 inbound 解析、endpoint type 选择、provider 路由、artifact 记录、请求 URL 构造和真实 credentials 发送。

## OutboundProfile

在 `pkg/llmbridge` 新增一个小型配置结构：

```go
type OutboundProfile struct {
    Type      string
    ConfigRaw string
}

func OutboundProfileFromAnnotations(ann map[string]string) OutboundProfile
```

`Type` 取 `strings.ToLower(strings.TrimSpace(ann["ah.outbound.type"]))`。空值表示使用 format 默认 outbound。`ConfigRaw` 取 `strings.TrimSpace(ann["ah.outbound.config"])`。空值表示不应用额外配置。

在 `pkg/server/handle_unified_gateway.go` 的 retry loop 中，从 `candAnno` 构造 profile 并传给 llmbridge：

- request bridge：`BridgeRequest(ctx, srcFormat, side.upFormat, reqBody, req.Header, bridgeURL, profile)`
- non-stream response bridge：把同一 profile 放入 `unifiedStreamArgs`，调用 `BridgeNonStream`
- stream response bridge：把同一 profile 放入 `unifiedStreamArgs`，调用 `BridgeStream`

`BridgeRequest` / `BridgeNonStream` / `BridgeStream` 的现有签名直接增加 `OutboundProfile` 参数。`pkg/llmbridge` 当前是 unified gateway 的专用适配层，调用点集中，直接改签名比新增兼容包装更清楚。

## Outbound 选择规则

`outboundFor` 直接增加 profile 参数：

```go
func outboundFor(f Format, profile OutboundProfile) (transformer.Outbound, error)
```

规则：

1. `profile.Type == ""`：使用当前按 `Format` 选择 transformer 的行为。
2. `profile.Type` 为 `openrouter` / `deepseek` / `fireworks`：只在 `f` 是 `FormatOpenAIChatCompletions` 时启用对应 transformer。其它 Format 返回明确错误，避免把 OpenAI-compatible outbound 套到 Anthropic/Gemini/Responses endpoint 上产生错误协议。
3. 其它非空 type 返回错误：`llmbridge: unsupported outbound type "..."`。

限制在 OpenAI Chat Completions format 的理由：当前 axonhub 的 `openrouter`、`deepseek`、`fireworks` 包都以 OpenAI-compatible chat/completions 为主体；`openai/responses`、Anthropic、Gemini 有自己的 wire format 和 response/stream 解析路径。

## Config 合并

每个 profile transformer 使用对应包自己的 Config 类型：

- `openrouter.Config`
- `deepseek.Config`
- `fireworks.Config`

合并流程：

1. 创建默认 config，包含占位 `BaseURL` 和占位 `APIKeyProvider`。
2. 如果 `profile.ConfigRaw != ""`，用 `encoding/json.Decoder` 解到该 config。必须 `DisallowUnknownFields`，防止拼错字段静默失效。
3. 再次强制写回 `BaseURL = "https://upstream.invalid"` 和 `APIKeyProvider = auth.NewStaticKeyProvider("placeholder")`。
4. 调对应 `NewOutboundTransformerWithConfig`。

`ah.outbound.config` 使用 axonhub Config 的 JSON tag 字段名，例如：

```json
{
  "base_url": "https://example.invalid/v1"
}
```

当前三类 Config 只有 `base_url` 这个可 JSON 配置字段。该字段仍会被 picotera 强制替换为占位 URL，因为 unified bridge 只读取 transformer 输出 body 和 Content-Type；真实上游 URL 始终来自 provider endpoint 的 `upstream_url`。

实现仅接受当前 Config 类型通过 JSON tag 暴露的字段。新增字段必须经过本规格的强制字段审查：URL、auth 或请求发送职责相关字段在 merge 后由 picotera 覆盖。

## Request 与 Response 使用同一 profile

同一次 upstream attempt 必须使用同一个 `OutboundProfile` 转换 request、non-stream response 和 stream response。否则 OpenRouter/DeepSeek/Fireworks 的 request 特殊字段和 response/stream 解析字段会不一致。

`unifiedStreamArgs` 增加：

```go
outboundProfile llmbridge.OutboundProfile
```

`unifiedStreamSuccess` 内部在以下分支使用它：

- upstream 非流式响应转 source 响应时调用 `BridgeNonStream`
- upstream 流式响应转 source SSE 时调用 `BridgeStream`

identity path 不受影响：`src == upstream` 时仍直接返回原 body/reader，不创建 outbound transformer。

## 错误行为

`ah.outbound.config` 不是合法 JSON object、包含未知字段、或对应 transformer 构造失败时，本次 attempt 失败并进入现有 retry 流程。错误 message 进入 upstream request row 的失败信息和 `lastError`，现有 retry loop 继续处理下一个候选。

如果 `ah.outbound.type` 不支持或与 endpoint format 不兼容，也按 attempt 失败处理。这样一个 provider 的错误 annotations 不会直接让整个 unified 请求在还有其它 candidate 时失败。

## 不变项

- 不改 DB schema、REST API、dashboard UI。
- 不改 annotations 合并工具和 hook ctx 结构。
- 不让 axonhub transformer 的 URL/Auth 结果覆盖 picotera 组装好的 HTTP request。
- 不在 path-based gateway 使用该 profile；本需求只作用于 unified bridge。
