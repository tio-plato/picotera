# HTTP Client 自动解压设计

## 目标

PicoTera 对上游响应增加内部自动解压能力。上游响应带有 `Content-Encoding` 时，内部处理统一读取解压后的响应体；返回给客户端的响应体保持上游原始压缩字节和原始 `Content-Encoding` 头。

内部处理范围包括：

- response artifacts
- llmbridge stream / non-stream response bridge
- stream 聚合 artifact
- usage token 记录
- TTFT 检测
- 非 200 上游错误体记录和重试错误信息
- fetch models 的 provider models 解析

## 解压边界

解压发生在 server 读取上游响应之后、内部消费者读取响应体之前。客户端转发路径不改写上游 payload：

- path gateway 成功响应：客户端收到原始压缩字节；内部 extractor、artifact、aggregation 使用解压字节。
- unified gateway 成功响应：
  - 非桥接场景：客户端收到原始压缩字节；内部 upstream artifact、meta artifact、usage、TTFT、aggregation 使用解压字节。
  - 桥接场景：桥接器消费解压后的上游字节，客户端收到桥接后的未压缩响应；上游 artifact 使用解压后的上游响应，meta artifact 使用桥接后的响应。
- 非 200 上游响应：不写给客户端，读取解压后的 body 作为 artifact body、错误消息和 `LastError.Message`。
- fetch models：解析解压后的 `/models` body。

## 辅助类型

在 `pkg/server` 增加响应解压辅助模块，例如 `response_decompression.go`。

核心接口：

```go
type decodedResponseBody struct {
    Body        io.ReadCloser
    Encoding    string
    Compressed  bool
}

func decodedBody(resp *http.Response) (*decodedResponseBody, error)
func decodedReadCloser(src io.ReadCloser, encoding string) (io.ReadCloser, error)
```

`encoding` 从 `Content-Encoding` 读取并严格匹配支持值：

- `gzip`
- `br`
- `zstd`
- 空字符串表示不解压

不支持多层编码和未知编码；出现未知或组合编码时直接返回明确错误。该项目约定 fail fast，不做宽松猜测或兼容分支。

## 压缩算法实现

使用现有依赖，不新增第三方库：

- gzip：标准库 `compress/gzip`
- br：已有依赖 `github.com/andybalholm/brotli`
- zstd：已有依赖 `github.com/klauspost/compress/zstd`

zstd reader 需要在 close 时释放 decoder 资源。实现一个小的 `ReadCloser` 包装器，在 `Close` 中先关闭原始 body，再关闭 zstd decoder。

## Path Gateway 数据流

`streamSuccess` 当前用同一个 `resp.Body` 同时驱动客户端写入、metrics extractor、artifact capture 和 aggregation。改造后拆成两路：

1. 原始 `resp.Body` 进入 `io.TeeReader`。
2. tee 的主输出写给客户端，保持原始压缩字节。
3. tee 的副本写入 `io.PipeWriter`。
4. pipe reader 根据 `Content-Encoding` 解压。
5. `ResponseExtractor`、内部 capture buffer、artifact、aggregation 从解压 reader 读取。

读循环以解压 reader 为驱动；每次读取解压 chunk 时，上游原始字节同步被 tee 到客户端。这样 TTFT 仍由解压后的第一段可解析事件触发，同时客户端持续收到原始压缩流。

响应头复制规则保持原样：继续去掉 `Content-Length`，保留 `Content-Encoding`。客户端 body 未解压，因此不移除 `Content-Encoding`。

## Unified Gateway 数据流

unified gateway 需要按是否桥接分开处理。

### 非桥接

非桥接时客户端必须收到原始压缩字节，内部使用解压字节。复用 path gateway 的双路读取策略：

- 原始 tee 主输出写客户端。
- tee 副本进入解压 reader。
- extractor、upstream artifact、meta artifact、aggregation 使用解压字节。
- meta artifact header 使用发送给客户端的 header，但 body 存解压后的响应体。

### 桥接

桥接需要先理解上游协议，不能把压缩字节交给 llmbridge：

- 客户端不直接接收上游原始 body。
- `decodedBody(resp)` 直接作为 extractor 和 upstream tee 输入。
- stream bridge 和 non-stream bridge 消费解压后的上游字节。
- upstream artifact 使用解压后的上游字节。
- meta artifact 使用桥接输出字节。
- 转发给客户端的 header 仍按现有桥接逻辑移除上游 `Content-Type`、`Content-Length`、`Transfer-Encoding`，并额外移除 `Content-Encoding`，因为桥接输出不是上游压缩字节。

## 非 200 响应

普通网关和 unified gateway 的非 200 分支改为读取解压 body。读取失败时该 attempt 标记失败，错误消息说明解压失败；响应 artifact 在无法得到完整解压体时不写半成品。

## Fetch Models

`handleFetchModels` 在读取 `/models` 响应前使用 `decodedBody(resp)`。成功响应和非 2xx 错误摘要都读取解压后的 body。

## JSX Fetch

`pkg/jsx/helpers.go` 中 `picotera.fetch` 当前使用独立 `http.Client`。Go 默认 transport 会在请求未显式设置 `Accept-Encoding` 时自动处理 gzip，但不会处理 br/zstd。该 helper 不属于 gateway 上游转发路径，不参与 response artifacts、llmbridge、usage 或 TTFT。本次计划不改 JSX fetch，避免扩大行为面。

## 测试策略

新增 server 层单元测试覆盖解压辅助函数：

- gzip 解压成功
- br 解压成功
- zstd 解压成功
- 空 `Content-Encoding` 返回原始 reader
- 未知 encoding 返回错误
- 组合 encoding 返回错误

新增网关成功路径测试使用 `httptest.Server` 返回 gzip SSE 或 JSON，断言：

- 客户端响应保留 `Content-Encoding: gzip`
- 客户端 body 是压缩字节
- 内部 artifact / aggregation / metrics 使用解压后的内容

新增 unified 桥接路径测试至少覆盖一个 compressed non-stream upstream，断言桥接输入使用解压后的 JSON。
