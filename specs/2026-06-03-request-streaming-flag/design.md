# 设计：请求 Streaming 标识与非流式请求的 header 超时豁免

## 背景

两条网关链路共用 `gatewayFlow`（`pkg/server/gateway_flow.go`）：

- path-based gateway（`handle_gateway.go`）；
- unified gateway（`handle_unified_gateway.go`）。

历史上存在**两个语义不同**的流式标志：

- `gatewayModelState.Streaming` —— 原本只被 unified 链路通过 `extractUnifiedModelAndStream` 填充，path 链路恒为 `false`，只喂给 `beforeTransform` JS hook 的 `Stream` 入参；
- `gatewayModelMode.Streaming` —— 窄义标志（仅 Gemini 路由 + body `stream`），用于 unified 候选解析（`candidateEndpointTypes`）：在 Anthropic/OpenAI 源面对 Gemini 上游时，用它在 `geminiGenerateContent` 与 `geminiStreamGenerateContent` 两个上游变体之间选择。

**本次决策：合并为单一来源。** 两个字段统一由五规则 `detectStreaming` 填充，`gatewayModelState.Streaming` 是 `gatewayModelMode.Streaming` 的镜像。候选解析、header 超时、`beforeTransform` hook 三者共用同一个流式判定。

上游 HTTP 请求经 `Server.forwardRequest` 发出，使用按 proxy URL 缓存的 `*http.Transport`（`proxy_transport.go`）。该 transport 的 `ResponseHeaderTimeout = GatewayResponseHeaderTimeout`（默认 16s），对所有上游请求统一生效。

## 目标

1. 在两条链路上都用统一的五条规则计算「请求是否流式」，写入 `gatewayModelState.Streaming`。
2. 非流式请求发往上游时不应用 `ResponseHeaderTimeout`（允许上游缓冲整段响应、长时间后才返回 header）。

## 设计

### 1. 统一的流式探测

新增纯函数：

```go
func detectStreaming(srcFormat llmbridge.Format, r *http.Request, body []byte) bool
```

按 proposal 的五条规则，命中任意一条即为 `true`：

1. `srcFormat == llmbridge.FormatGeminiStreamGenerateContent`（覆盖 path 与 unified 两条链路的 Gemini stream 端点：path 经 `sourceEndpointTypeForPath`，unified 经路由 `srcFormat`）；
2. `gjson.GetBytes(body, "stream").Bool()` 为真；
3. `Accept` 头包含 `text/event-stream`；
4. `Accept` 头包含 `application/x-ndjson`；
5. 否则 `false`。

`Accept` 判定遍历 `r.Header.Values("Accept")` 并大小写不敏感匹配。

在 `resolveAndRewriteModel`（`gateway_flow.go`）构造 `gatewayModelState` 时调用，对两条链路一致填充 `Streaming`。此时 `f.body` 仍是客户端原始 body（`rewriteModel` hook 只改 `model` 字段，不影响 `stream` 与 Accept），探测到的是客户端意图。

### 2. 合并为单一流式判定（候选解析改用五规则）

`gatewayModelMode.Streaming` 与 `gatewayModelState.Streaming` 合并为单一来源：在 `resolveAndRewriteModel` 中调用一次 `detectStreaming` 写入 `mode.Streaming`，`gatewayModelState.Streaming` 取自同一值。三处消费者共用：

- `candidateEndpointTypes` 的上游变体选择（unified）；
- header 超时决策；
- `beforeTransform` hook 的 `Stream` 入参。

`extractUnifiedModelAndStream` 不再自行推导 stream，简化为 `extractUnifiedModel`（只返回 model）；其原先的「Gemini 路由 + body `stream`」逻辑已被五规则完全覆盖（规则 1 覆盖 Gemini stream 路由，规则 2 覆盖 body `stream`）。

**取舍**：候选解析现在也受 Accept 头影响。当出现「body `stream:false` 但 `Accept: text/event-stream`」这类矛盾请求时，会被判为流式，在 Anthropic/OpenAI 源面对 Gemini 上游时选中 `geminiStreamGenerateContent` 变体、输出 SSE。这是合并为单一来源后接受的代价——以五规则（含 Accept）作为统一权威，而非让候选解析单独以 body/路由为准。

### 3. 非流式使用更宽松的 header 超时

非流式请求不应用 16s 的 `ResponseHeaderTimeout`，但仍受全局读超时 `GatewayReadTimeout`（默认 300s）约束——即把非流式的 header 等待上限从 16s 放宽到 300s，而非完全无上限。这样 HTTP/1.1 上游在 header 等待阶段也有硬上限，慢但存活的非流式上游不会被 16s 误杀，死掉的上游最终仍会超时。

`ResponseHeaderTimeout` 是 `*http.Transport` 上的连接级字段，无法逐请求覆盖，因此准备两套 transport：

- 现有 `baseTransport`（`ResponseHeaderTimeout = GatewayResponseHeaderTimeout`，默认 16s）→ 现有 `proxyCache`，用于流式请求；
- 新增 `nonStreamBase := baseTransport.Clone()` 并 `nonStreamBase.ResponseHeaderTimeout = config.GatewayReadTimeout` → 新增 `nonStreamProxyCache`，用于非流式请求。

克隆在 `http2.ConfigureTransports(baseTransport)` **之后**进行，使克隆体继承 HTTP/2 配置（与现有 proxyCache 对非空 proxy URL 的克隆同理）。HTTP/2 的 `ReadIdleTimeout`/`PingTimeout` 在两套 transport 上都生效。

`forwardRequest` 增加一个 `streaming bool` 入参，决定取哪套缓存：

```go
func (s *Server) forwardRequest(req *http.Request, proxyURL string, streaming bool) (*http.Response, error)
```

- 网关分发（`gateway_flow_attempts.go:runSingleAttempt`）传 `f.model.Streaming`：流式 → `proxyCache`（16s）；非流式 → `nonStreamProxyCache`（`GatewayReadTimeout`，300s）。
- 管理侧的「拉取上游模型列表」（`handle_provider_endpoint.go`）传 `true`，保持现状（16s）。

### 不引入的内容

- 不新增配置项 / 环境变量：非流式复用既有的 `GatewayReadTimeout` 作为 header 超时上限。
- 不引入兼容层或回退分支：直接更新全部 `forwardRequest` 调用点。
- 不改动任何 REST/contract/OpenAPI，因此无需 `api.md`、无需重新生成 openapi。
