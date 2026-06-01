# 执行计划 — 请求体自动解压

## 1. 新增中间件 `pkg/server/request_decompression.go`

实现 `func decompressRequest(next http.Handler) http.Handler`：

- 复用同包的 `contentEncoding(r.Header)`：
  - 返回 `("", nil)` → 直接 `next.ServeHTTP(w, r)` 并 return。
  - 返回 `err`（多个值 / 不识别编码）→ `writeGatewayError(w, http.StatusUnsupportedMediaType, err.Error(), errorx.InvalidRequest.Error())`，return。
- 调 `decodedReadCloser(r.Body, encoding)`：
  - 返回 `err`（如 gzip 头损坏）→ `writeGatewayError(w, http.StatusBadRequest, "failed to decode request body: "+err.Error(), errorx.InvalidRequest.Error())`，return。
- 成功后：
  - `r.Body = decoded`
  - `r.Header.Del("Content-Encoding")`
  - `r.Header.Del("Content-Length")`
  - `r.ContentLength = -1`
  - `next.ServeHTTP(w, r)`

注意 import：`net/http`、`picotera/pkg/errorx`。

## 2. 在 `pkg/server/server.go` 的 `registerEndpoints()` 挂载

把 5 个 unified `s.router.Post(...)` 的 handler 用 `decompressRequest(...).ServeHTTP` 包裹；catch-all 改为 `s.router.Mount("/", decompressRequest(&gatewayHandler{s}))`。`/api/picotera` 管理 API 注册（`registerOperations`）保持不变。

```go
s.router.Post("/api/picotera/v1/messages",
    decompressRequest(s.handleUnifiedGenerate(llmbridge.FormatAnthropicMessages)).ServeHTTP)
s.router.Post("/api/picotera/v1/responses",
    decompressRequest(s.handleUnifiedGenerate(llmbridge.FormatOpenAIResponses)).ServeHTTP)
s.router.Post("/api/picotera/v1/chat/completions",
    decompressRequest(s.handleUnifiedGenerate(llmbridge.FormatOpenAIChatCompletions)).ServeHTTP)
s.router.Post("/api/picotera/v1beta/models/{model}:generateContent",
    decompressRequest(s.handleUnifiedGenerate(llmbridge.FormatGeminiGenerateContent)).ServeHTTP)
s.router.Post("/api/picotera/v1beta/models/{model}:streamGenerateContent",
    decompressRequest(s.handleUnifiedGenerate(llmbridge.FormatGeminiStreamGenerateContent)).ServeHTTP)

s.router.Mount("/", decompressRequest(&gatewayHandler{s}))
```

## 3. 测试 `pkg/server/request_decompression_test.go`

用 `httptest` 驱动中间件，断言：

- 无 `Content-Encoding`：body 原样透传，`next` 收到原始字节。
- `gzip` / `br` / `zstd`：`next` 读到解压后的明文；`r.Header` 中 `Content-Encoding`、`Content-Length` 已删除，`r.ContentLength == -1`。
- 多个 `Content-Encoding` 值：返回 415，`next` 未被调用。
- 不识别的编码（如 `deflate`）：返回 415，`next` 未被调用。
- 损坏的 gzip 头：返回 400，`next` 未被调用。

压缩输入构造分别用 `compress/gzip`、`github.com/andybalholm/brotli`、`github.com/klauspost/compress/zstd`。

## 4. 验证

- `go build -o /dev/null ./cmd/picotera`
- `go test ./pkg/server/`
- 手动冒烟（可选）：对 `POST /api/picotera/v1/messages` 发送 gzip 压缩 body + `Content-Encoding: gzip`，确认正常路由、artifact 记录的是明文、identity 透传上游不带 `Content-Encoding`。

## 不改动项

- 不新增 contract / OpenAPI / dashboard（无 API 形变），无需跑 `mise run openapi`。
- 不改 `gatewayFlow.readBody()` 与上游头部复制逻辑——解压后头部已一致。
- 不引入请求体大小上限。
