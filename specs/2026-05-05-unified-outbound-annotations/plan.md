# Plan — Unified outbound annotations

## 1. 扩展 llmbridge profile 类型

1. 在 `pkg/llmbridge/llmbridge.go` 新增：
   - `type OutboundProfile struct { Type string; ConfigRaw string }`
   - `func OutboundProfileFromAnnotations(map[string]string) OutboundProfile`
2. `OutboundProfileFromAnnotations` 对 `ah.outbound.type` 做 `strings.TrimSpace` + `strings.ToLower`，对 `ah.outbound.config` 做 `strings.TrimSpace`。

## 2. 新增 profile-aware outbound 构造

1. 在 `pkg/llmbridge/llmbridge.go` 引入 axonhub transformer 包：
   - `deepseektrans`
   - `fireworkstrans`
   - `openroutertrans`
2. 把现有 `outboundFor(f Format)` 改成 `outboundFor(f Format, profile OutboundProfile)`。
3. 更新 `outboundFor`：
   - `profile.Type == ""` 时走现有 switch。
   - `profile.Type` 为 `openrouter` / `deepseek` / `fireworks` 时要求 `f == FormatOpenAIChatCompletions`。
   - format 不兼容时返回错误。
   - type 不支持时返回错误。
4. 新增三个 helper：
   - `openRouterOutbound(profile OutboundProfile) (transformer.Outbound, error)`
   - `deepSeekOutbound(profile OutboundProfile) (transformer.Outbound, error)`
   - `fireworksOutbound(profile OutboundProfile) (transformer.Outbound, error)`
5. 三个 helper 共享 `decodeOutboundConfig(raw string, dst any) error` 处理 JSON config：
   - 默认 config 写入 placeholder `BaseURL` 和 placeholder `APIKeyProvider`。
   - `profile.ConfigRaw` 非空时用 `json.Decoder` + `DisallowUnknownFields` 解到 config。
   - 解完后重新强制写入 placeholder `BaseURL` 和 `APIKeyProvider`。

## 3. 扩展 bridge 函数

1. 在 `pkg/llmbridge/bridge.go`：
   - `BridgeRequest` 签名增加 `profile OutboundProfile` 参数。
   - 把 `outboundFor(dst)` 改为 `outboundFor(dst, profile)`。
2. 在 `pkg/llmbridge/bridge.go`：
   - `BridgeNonStream` 签名增加 `profile OutboundProfile` 参数。
   - 把 `outboundFor(upstream)` 改为 `outboundFor(upstream, profile)`。
3. 在 `pkg/llmbridge/bridge_stream.go`：
   - `BridgeStream` 签名增加 `profile OutboundProfile` 参数。
   - 把 `outboundFor(upstream)` 改为 `outboundFor(upstream, profile)`。
4. 更新 `pkg/llmbridge` 内全部调用点和测试调用点，不新增 `WithProfile` 包装函数，不保留旧签名。
5. 确认 identity path (`src == dst` / `src == upstream`) 仍优先返回，不解析 config。

## 4. 接入 unified handler

1. 在 `pkg/server/handle_unified_gateway.go` 的 retry loop 中，`candAnno` fallback 到 sidecar annotations 后立即生成：
   ```go
   outboundProfile := llmbridge.OutboundProfileFromAnnotations(candAnno)
   ```
2. request bridge 调用改成：
   ```go
   llmbridge.BridgeRequest(ctx, srcFormat, side.upFormat, reqBody, req.Header, bridgeURL, outboundProfile)
   ```
3. `unifiedStreamArgs` 结构体新增 `outboundProfile llmbridge.OutboundProfile`。
4. 构造 `unifiedStreamArgs` 时写入该 profile。
5. `unifiedStreamSuccess` 中所有 response bridge 调用传入 profile：
   - `BridgeNonStream(..., a.outboundProfile)`
   - `BridgeStream(..., a.outboundProfile)`
6. 不改 path-based gateway。

## 5. 测试 llmbridge

1. 扩展 `pkg/llmbridge/bridge_test.go`：
   - `OutboundProfileFromAnnotations` 覆盖 trim、小写、缺失 key。
   - `BridgeRequest` 对 OpenAI Chat Completions + `openrouter` 输出包含 OpenRouter 预期的 reasoning 字段行为。
   - `deepseek` profile 对 reasoning effort 的 DeepSeek 特殊 thinking 字段有覆盖。
   - `fireworks` profile 可成功构造 OpenAI-compatible body。
   - `openrouter` + `FormatAnthropicMessages` 返回 format 不兼容错误。
   - malformed `ah.outbound.config` 返回错误。
   - unknown config field 返回错误。
   - unknown outbound type 返回错误。
2. 更新旧 `BridgeRequest` / `BridgeNonStream` / `BridgeStream` 测试调用，为默认路径传入空 `OutboundProfile{}`。

## 6. 测试 unified handler

1. 扩展 `pkg/server/handle_unified_gateway_test.go` 中 bridge 相关 helper，覆盖 candidate annotations 中 `ah.outbound.type` 能进入 llmbridge profile。
2. unified handler 现有测试不提供完整 postgres harness 时，行为覆盖放在 llmbridge 层，server 测试验证 `OutboundProfileFromAnnotations(candAnno)` 的接线点可编译。

## 7. 验证

1. 运行：
   ```bash
   go test ./pkg/llmbridge/... ./pkg/server/...
   go build ./...
   ```
2. 检查 `go.mod` 不新增依赖；三个 axonhub transformer 包来自现有 `github.com/looplj/axonhub/llm` 模块。
3. 通读 `design.md`、`api.md`、`plan.md`，删除犹豫和备选措辞。
