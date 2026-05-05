# Plan — beforeTransform hook

## 1. 更新 JS SDK hook 列表

1. 在 `pkg/jsx/sdk.js` 的 `picotera.hooks` 中新增 `beforeTransform: new Waterfall()`。
2. 不改变 `Waterfall` 行为；priority、undefined passthrough、返回值替换沿用已有 hook 机制。

## 2. 新增 JSX 类型与 session 方法

1. 在 `pkg/jsx/types.go` 新增 `OutboundProfile`：
   - `Type string`
   - `Config map[string]any`
2. 在 `pkg/jsx/types.go` 新增 `BeforeTransformInput`，字段按 `api.md` 定义。
3. 在 `pkg/jsx/session.go` 新增 `RunBeforeTransformHook`：
   - 初始值由 Go 调用方传入。
   - 执行 `picotera.hooks.beforeTransform.runWaterfall(ctx, initial)`。
   - 返回 `undefined` / `null` 时使用初始值。
   - 最终结果必须是 object。
   - `type` 缺失时沿用进入 hook 的当前 type，存在时必须是 string。
   - `config` 缺失时设为 `{}`，存在时必须是非数组 object。
   - 校验失败返回 `jsx: beforeTransform ...` 错误。
4. 增加 `pkg/jsx/engine_test.go` 覆盖：
   - 默认 passthrough。
   - 返回新 profile。
   - 原地修改 profile。
   - priority waterfall 覆盖。
   - 非 string type 报错。
   - 非 object config 和数组 config 报错。

## 3. 替换 llmbridge profile 模型

1. 在 `pkg/llmbridge/llmbridge.go` 把 `OutboundProfile` 改成 `Type string; Config map[string]any`。
2. 新增 `DefaultOutboundProfileForFormat(f Format) (OutboundProfile, error)`：
   - `FormatAnthropicMessages` → `Type: "anthropic"`
   - `FormatOpenAIChatCompletions` → `Type: "openai"`
   - `FormatOpenAIResponses` → `Type: "openaiResponses"`
   - `FormatGeminiGenerateContent` → `Type: "gemini"`
   - `FormatGeminiStreamGenerateContent` → `Type: "gemini"`
3. 删除 `OutboundProfileFromAnnotations`。
4. 删除 `Fallback` 字段和 `profile.Fallback == "default"` 分支。
5. `outboundFor` 改成按显式 `Type` 选择：
   - `anthropic` 只支持 `FormatAnthropicMessages`。
   - `openai` 只支持 `FormatOpenAIChatCompletions`。
   - `openaiResponses` 只支持 `FormatOpenAIResponses`。
   - `gemini` 只支持 `FormatGeminiGenerateContent` 和 `FormatGeminiStreamGenerateContent`。
   - `openrouter` / `deepseek` / `fireworks` 只支持 `FormatOpenAIChatCompletions`。
   - 未知 type 返回错误。
6. 把 `decodeOutboundConfig(raw string, dst any)` 改成 `decodeOutboundConfig(config map[string]any, dst any)`：
   - `nil` 或空 map 直接返回 nil。
   - `json.Marshal(config)` 后用 `json.Decoder` + `DisallowUnknownFields()` 解到目标 config。
   - 保留 multiple JSON values 防御检查。
7. default outbound helper 也消费 `profile.Config`，让 `anthropic`、`openai`、`openaiResponses`、`gemini` 的 config 由 hook 可控，但 decode 后继续强制 placeholder `BaseURL` 和 `APIKeyProvider`。
8. 三个 provider-specific helper 使用 `profile.Config`，并在 decode 前后强制 placeholder `BaseURL` 和 `APIKeyProvider`。

## 4. 接入 unified handler

1. 在 `pkg/server/handle_unified_gateway.go` retry loop 中删除：
   ```go
   outboundProfile := llmbridge.OutboundProfileFromAnnotations(candAnno)
   ```
2. 在 `rewriteRequest` 完成并 `buildRequestFromPending` 成功后，构造初始 profile：
   ```go
   baseProfile, err := llmbridge.DefaultOutboundProfileForFormat(side.upFormat)
   initialProfile := jsx.OutboundProfile{Type: baseProfile.Type, Config: map[string]any{}}
   ```
3. 调用 `session.RunBeforeTransformHook`，ctx 使用当前 attempt 的 `endpoint`、`model`、`provider`、`mpe`、retry 计数、`clientRequest`、`pendingRequest`、`apiKey`、`annotations`、`srcFormat.String()`、`side.upFormat.String()` 和 `streaming`。
4. hook 错误走现有 `failHook` 路径并结束请求。
5. 将 `jsx.OutboundProfile` 转成 `llmbridge.OutboundProfile`，传给 `BridgeRequest`。
6. `unifiedStreamArgs.outboundProfile` 继续携带同一个 profile，response bridge 继续复用。
7. identity bridge 仍传入 profile，但 llmbridge 在 identity 分支不使用它。

## 5. 删除 annotation outbound 行为

1. 删除所有 `ah.outbound.type`、`ah.outbound.config`、`ah.outbound.fallback` 的 Go core 读取逻辑。
2. 更新 `pkg/llmbridge/bridge_test.go`：
   - 删除 `TestOutboundProfileFromAnnotations`。
   - 删除 fallback default 测试。
   - 把 config 测试改成 map config。
   - 保留 openrouter、deepseek、fireworks、未知 type、未知 config 字段、不兼容 format 覆盖。
3. 搜索确认 `rg "ah\\.outbound|OutboundProfileFromAnnotations|Fallback"` 不再命中 Go 实现代码；规格文档命中可以保留。

## 6. 测试 unified 接线

1. 在 `pkg/server/handle_unified_gateway_test.go` 中增加或调整轻量测试，验证 `beforeTransform` 产生的 profile 能进入 request bridge 的调用路径。
2. 如果现有 server 测试无法完整执行 postgres-backed gateway，保持 server 层测试聚焦编译接线，核心行为由 `pkg/jsx` 与 `pkg/llmbridge` 单测覆盖。

## 7. 验证

1. 运行：
   ```bash
   go test ./pkg/jsx/... ./pkg/llmbridge/... ./pkg/server/...
   go build ./...
   ```
2. 确认 `go.mod` 没有新增依赖。
3. 通读 `design.md`、`api.md`、`plan.md`，删除犹豫、兼容层、fallback 和 deprecated-path 表述。
