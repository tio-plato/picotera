# 请求体自动解压

如果请求也有 Content-Encoding（比如 zstd/gzip/br 之类我们可以识别的格式），也将请求解压。这事找个中间件，早早地做，适用于所有网关请求和 unified 的请求。

## 补充说明

- 解压发生在请求生命周期最早期（HTTP 中间件层），位于 body 被读取、写入 artifact、项目/模型提取、JS hook、转发上游之前。
- 适用范围：catch-all 网关 mount（`/`）与 5 个 unified 生成路由，不包括 `/api/picotera` 管理 API。
- 识别的编码：`gzip`、`br`、`zstd`（与现有响应解压 `pkg/server/response_decompression.go` 一致）。
- 解压后删除 `Content-Encoding` 与 `Content-Length` 头，使下游（包括 identity 透传转发上游）看到的是解压后的明文 body 且头部一致。
- 严格校验：无法识别的 `Content-Encoding`、或出现多个 `Content-Encoding` 值时，直接返回错误，不做宽松处理。
