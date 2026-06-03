# 执行计划

## 1. 新增 `detectStreaming` 探测函数

文件：`pkg/server/gateway_flow.go`

- 新增函数 `detectStreaming(srcFormat llmbridge.Format, r *http.Request, body []byte) bool`，实现五条规则：
  1. `srcFormat == llmbridge.FormatGeminiStreamGenerateContent` → `true`；
  2. `gjson.GetBytes(body, "stream").Bool()` → `true`；
  3. 遍历 `r.Header.Values("Accept")`，大小写不敏感包含 `text/event-stream` → `true`；
  4. 同上包含 `application/x-ndjson` → `true`；
  5. 否则 `false`。
- 在 import 块补充 `strings` 与 `github.com/tidwall/gjson`。

## 2. 用五规则填充单一 Streaming 来源

文件：`pkg/server/gateway_flow.go`，函数 `resolveAndRewriteModel`

- 在 `mode.RoutedModel = mode.OriginalModel` 之后，用五规则覆盖 `mode.Streaming`，并让 `gatewayModelState.Streaming` 取自同一值：
  ```go
  mode.RoutedModel = mode.OriginalModel
  mode.Streaming = detectStreaming(f.config.SourceFormat, f.r, f.body)
  f.model = gatewayModelState{Mode: mode, Original: mode.OriginalModel, Routed: mode.RoutedModel, Streaming: mode.Streaming}
  ```
- 这样 `mode.Streaming`（喂 `candidateEndpointTypes`）与 `gatewayModelState.Streaming`（喂 header 超时 + `beforeTransform` hook）同源。

简化 unified 链路的 model 提取（`gateway_unified_helpers.go` + `handle_unified_gateway.go`）：

- 将 `extractUnifiedModelAndStream(src, r, body) (string, bool, error)` 改为 `extractUnifiedModel(src, r, body) (string, error)`，去掉 stream 推导（已被五规则覆盖）。
- unified 的 `ExtractModel` 回调改为 `return gatewayModelMode{OriginalModel: model, HasModel: true}, err`（不再设 `Streaming`）。
- 同步更新 `handle_unified_gateway_test.go` 中相关测试，并新增 `TestDetectStreaming` 覆盖五规则。

## 3. 合并后两个 Streaming 字段的注释

文件：`pkg/server/gateway_flow.go`

- 在 `gatewayModelMode.Streaming` 上加注释：五规则探测出的「客户端是否期望流式响应」（见 `detectStreaming`），在 `resolveAndRewriteModel` 填充，驱动 `candidateEndpointTypes` 变体选择、header 超时与 `beforeTransform` hook，单一来源。
- 在 `gatewayModelState.Streaming` 上加注释：镜像 `Mode.Streaming`。

## 4. 新增非流式 transport 与缓存

文件：`pkg/server/server.go`

- 在 `http2.ConfigureTransports(baseTransport)` 之后、构造 `proxyCache` 处，新增：
  ```go
  nonStreamBase := baseTransport.Clone()
  nonStreamBase.ResponseHeaderTimeout = config.GatewayReadTimeout
  proxyCache := newProxyTransportCache(baseTransport)
  nonStreamProxyCache := newProxyTransportCache(nonStreamBase)
  ```
- 在 `Server` 结构体（约 `server.go:36`）新增字段 `nonStreamProxyCache *proxyTransportCache`。
- 在构造 `Server{...}`（约 `server.go:147`）处赋值 `nonStreamProxyCache: nonStreamProxyCache`。

## 5. `forwardRequest` 增加 streaming 开关

文件：`pkg/server/gateway_helpers.go`

- 函数签名改为：
  ```go
  func (s *Server) forwardRequest(req *http.Request, proxyURL string, streaming bool) (*http.Response, error) {
      cache := s.proxyCache
      if !streaming {
          cache = s.nonStreamProxyCache
      }
      return cache.get(proxyURL).RoundTrip(req)
  }
  ```
- 更新注释：流式用默认 `ResponseHeaderTimeout`，非流式改用更宽松的 `GatewayReadTimeout` 作为 header 超时上限。

## 6. 更新全部 `forwardRequest` 调用点

- `pkg/server/gateway_flow_attempts.go:149`（`runSingleAttempt`）：
  ```go
  resp, err := f.h.forwardRequest(prepared.Request, side.ProxyURL, f.model.Streaming)
  ```
- `pkg/server/handle_provider_endpoint.go:96`（拉取上游模型列表）：
  ```go
  resp, err := s.forwardRequest(req, proxyURL, true)
  ```

## 7. 验证

- `go build ./...`（或 `go build -o picotera ./cmd/picotera`）编译通过。
- `go vet ./pkg/server/...`。
- 运行现有 Go 测试：`go test ./pkg/server/... ./pkg/llmbridge/...`。
- 人工核对：
  - path-based Gemini stream 端点、body `stream:true`、`Accept: text/event-stream`、`Accept: application/x-ndjson` 均判定为流式并应用 header 超时；
  - 普通非流式请求（如 body `stream:false` 的 chat/completions）走 `nonStreamProxyCache`，header 超时上限放宽为 `GatewayReadTimeout`（300s）而非 16s；
  - unified 候选解析（Gemini 变体选择）行为不变。
