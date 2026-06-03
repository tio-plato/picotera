# 设计：请求 Streaming 标识与非流式请求的 header 超时豁免

## 背景

两条网关链路共用 `gatewayFlow`（`pkg/server/gateway_flow.go`）：

- path-based gateway（`handle_gateway.go`）；
- unified gateway（`handle_unified_gateway.go`）。

`gatewayFlow.model`（`gatewayModelState`）上已经存在一个 `Streaming bool`，但它当前只被 unified 链路通过 `extractUnifiedModelAndStream` 填充，path 链路恒为 `false`。该字段目前只喂给 `beforeTransform` JS hook 的 `Stream` 入参。

此外还存在另一个**语义不同**的流式标志 `gatewayModelMode.Streaming`：它专门用于 unified 候选解析（`candidateEndpointTypes`）——在 Anthropic/OpenAI 源面对 Gemini 上游时，用它在 `geminiGenerateContent` 与 `geminiStreamGenerateContent` 两个上游变体之间选择。

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

### 2. 与候选解析标志的关系（不改动路由行为）

`gatewayModelMode.Streaming`（窄义：仅由 Gemini 路由 + body `stream` 决定）保持不变，继续驱动 `candidateEndpointTypes` 的上游变体选择。新增的 Accept-头规则**不**进入候选解析——避免出现「body `stream:false` 但 Accept 为 SSE」时错误地选中 Gemini stream 上游、进而把非流式请求转成 SSE 输出。遵循「不擅自猜测、对输入严格」的约定，候选解析以 body/路由为权威。

两个字段语义不同，在结构体上各自加注释明确区分：

- `gatewayModelMode.Streaming`：上游变体选择用的窄义流式标志。
- `gatewayModelState.Streaming`：五规则探测出的「客户端是否期望流式响应」，用于 `beforeTransform` hook 与 header 超时决策。

`beforeTransform` hook 的 `Stream` 入参继续取 `f.model.Streaming`，因此其值从「仅 body/路由」收窄义放宽为五规则结果——更贴近「请求是否流式」的本意。

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
