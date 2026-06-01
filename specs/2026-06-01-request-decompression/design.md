# 请求体自动解压 — 设计

## 目标

在 HTTP 中间件层，对带 `Content-Encoding`（`gzip`/`br`/`zstd`）的网关请求与 unified 请求，于生命周期最早期解压请求体。解压后下游所有环节（artifact 存储、project/model 提取、JS hook、llmbridge 转换、上游转发）一律只看到解压后的明文 body。

## 现状

- 所有网关请求（catch-all `/` mount）与 5 个 unified 路由都收敛到 `gatewayFlow.run()`，由 `gatewayFlow.readBody()`（`pkg/server/gateway_flow.go:123`）用 `io.ReadAll(r.Body)` 读取整段 body，存入 `f.body`，后续全部基于 `f.body`。
- identity 透传转发上游时（`pkg/server/gateway_helpers.go:535`）从客户端 `r.Header` 复制头部，仅排除 auth/host/content-length，**不**排除 `Content-Encoding`。因此今天若客户端发压缩 body，透传会把压缩 body + `Content-Encoding` 一起转发上游，而 `f.body` 仍是压缩字节，project/model 提取等会失败。
- 已存在响应侧解压工具 `pkg/server/response_decompression.go`，其中 `contentEncoding(http.Header) (string, error)` 与 `decodedReadCloser(io.ReadCloser, string) (io.ReadCloser, error)` 是通用的，可直接复用。

## 方案

新增 HTTP 中间件 `decompressRequest`，包裹 catch-all 网关 handler 与 5 个 unified handler，使解压发生在 `gatewayFlow.run()` 之前。

### 中间件 `pkg/server/request_decompression.go`

```go
func decompressRequest(next http.Handler) http.Handler
```

逻辑：

1. 调 `contentEncoding(r.Header)` 读取并校验编码。
   - 返回 `""`（无 `Content-Encoding`）：直接 `next.ServeHTTP`，不做任何改动。
   - 返回错误（多个值、或不识别的编码）：以 `415 Unsupported Media Type` + 结构化错误体直接拒绝，不进入 `next`。
2. 对识别的编码调 `decodedReadCloser(r.Body, encoding)` 构造解压 reader。
   - 构造失败（如 gzip 头损坏）：以 `400 Bad Request` 拒绝。
3. 替换 `r.Body` 为解压 reader；删除 `r.Header["Content-Encoding"]`，删除 `r.Header["Content-Length"]`，置 `r.ContentLength = -1`。
4. `next.ServeHTTP(w, r)`。

删除 `Content-Encoding`/`Content-Length` 的作用：让 identity 透传（`buildUpstreamRequest` 从 `r.Header` 复制）转发的是解压后的明文 body，且不再带误导性的编码头；`Content-Length` 因 body 长度改变而失效，由下游按实际长度重设。

错误响应复用网关现有的 `writeGatewayError`（与 `gatewayFlow` 一致的结构化错误格式）。

### 注册（`pkg/server/server.go:registerEndpoints`）

5 个 unified 路由与 catch-all mount 均用 `decompressRequest` 包裹；`/api/picotera` 管理 API 不包裹。

```go
s.router.Post("/api/picotera/v1/messages",
    decompressRequest(s.handleUnifiedGenerate(llmbridge.FormatAnthropicMessages)).ServeHTTP)
// ... 其余 4 个 unified 路由同样包裹 ...
s.router.Mount("/", decompressRequest(&gatewayHandler{s}))
```

## 取舍与边界

- **复用而非重写**：直接复用 `response_decompression.go` 的 `contentEncoding` / `decodedReadCloser`，不引入新依赖。
- **严格校验**：不识别的编码与多值 `Content-Encoding` 直接拒绝（符合 CLAUDE.md「快速失败、不宽容」）。
- **流损坏的处理时机**：编码名校验与 reader 构造失败在中间件即拒绝；解压流中途损坏的错误在 `readBody()` 的 `io.ReadAll` 处暴露，沿用既有的 500「failed to read request body」。
- **无解压大小上限**：与现有 `io.ReadAll` 对未压缩 body 同样不设上限，本次不引入大小限制。
- **管理 API 不受影响**：中间件只挂在网关与 unified 路由上。
