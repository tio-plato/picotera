# Design — Request Artifacts

## 总体

为每条 `request` 行（meta 与 upstream 各自）保存一对 artifact 到对象存储：

- `<id>.request.json` —— 该 request 实际发送出去的 HTTP 头 + body。对 meta，是客户端发给我们的；对 upstream，是我们发给上游的。
- `<id>.response.json` —— 该 request 实际收到的 HTTP 头 + body。对 meta，是我们返还给客户端的；对 upstream，是上游返给我们的。

JSON 序列化后用 zstd 压缩，以 `Content-Encoding: zstd` 上传到 S3 兼容的对象存储（MinIO）。列请求 API 在每条 `RequestView` 中追加两个可选 presigned URL（`requestArtifactUrl` / `responseArtifactUrl`），前端用 `fetch` 拿到后由浏览器透明解压。

artifact 是否存在不写入数据库——由 `request.id` + `request.created_at` 推算 key，列表返回时无条件给出 presigned URL，前端打开后若 fetch 报 404 则提示「artifact 不可用」。

## Object key 模板

```
artifacts/<YYYY-MM-DD>/<id>.request.json.zst
artifacts/<YYYY-MM-DD>/<id>.response.json.zst
```

`<YYYY-MM-DD>` 取自 `request.created_at` 的 UTC 日期。`request.id` 是 xid（时间排序、URL 安全），不会撞键。

后端推算 key 时若 `created_at` 不可读（行未落库），降级为 `time.Now().UTC()`；推算时序：列表 handler 拿到 `created_at` 后直接计算字符串。

## 缓冲与上传时机

- **请求 body**：在 gateway 入口 `io.ReadAll` 后即得到完整字节，已经在内存里。
- **响应 body**：流式转发给下游的同时，用 `io.MultiWriter(w, &captureBuf)` 把字节同时写入一个内存 buffer。`captureBuf` 不设上限（按用户要求）。
- 上游请求的 request body 与 meta 相同（除非替换了 `model` 字段，因此各 upstream 按改写后的字节单独保存）。
- 上游响应 body：同样在 forward 循环里用 `io.MultiWriter` 把读到的字节复制一份。

上传通过单进程内的 worker pool 异步完成。`Server` 持有一个 `*ArtifactSink`，提供 `Put(ctx, key, payload)` 接口，内部把任务塞进 buffered channel（容量例如 256），N 个 goroutine 消费，调用 minio-go 的 `PutObject` 上传。每个 payload 已经 zstd 压缩，上传时设置 `ContentEncoding: "zstd"` 和 `ContentType: "application/json"`。

队列满 / sink 未配置时直接丢弃并 warn log，不阻塞 gateway。

## JSON 结构

```json
{
  "method": "POST",                 // 仅 request 文档
  "url": "https://...",             // 仅 request 文档
  "statusCode": 200,                // 仅 response 文档
  "headers": { "Content-Type": ["application/json"], ... },
  "body": "...",
  "bodyEncoding": "utf8" | "base64"
}
```

- `headers` 直接序列化 `http.Header`（已经是 `map[string][]string`）。
- `body` 检查是否合法 UTF-8：是则原样存字符串、`bodyEncoding="utf8"`；否则 base64 编码、`bodyEncoding="base64"`。
- 敏感头不做剥离（用户未要求；artifact 仅给已登录管理员看）。

## 配置

新增 env：

| Env                         | Config 字段        | 说明                              |
| --------------------------- | ------------------ | --------------------------------- |
| `PICOTERA_S3_ENDPOINT`      | `S3.Endpoint`      | host:port，例如 `localhost:34050` |
| `PICOTERA_S3_REGION`        | `S3.Region`        | 默认 `us-east-1`                  |
| `PICOTERA_S3_ACCESS_KEY`    | `S3.AccessKey`     |                                   |
| `PICOTERA_S3_SECRET_KEY`    | `S3.SecretKey`     |                                   |
| `PICOTERA_S3_BUCKET`        | `S3.Bucket`        | 例如 `picotera-artifacts`         |
| `PICOTERA_S3_USE_SSL`       | `S3.UseSSL`        | 默认 `false`                      |
| `PICOTERA_S3_PUBLIC_URL`    | `S3.PublicURL`     | 可选，用于 presigned URL 中重写 host（例如内部 MinIO 与浏览器可达 host 不同） |

`S3.Endpoint` 为空 → artifact 功能关闭：gateway 跳过捕获，列表 API 不生成 URL。

`docker-compose.yaml` 增补一个 `minio` 服务（port 34050，console 34049），使用 `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` 默认凭据，并通过 minio-mc 自动 `mb` 出 bucket。

## 依赖

- `github.com/minio/minio-go/v7` —— 官方 S3 / MinIO SDK，提供 `PutObject` 与 `PresignedGetObject`。
- `github.com/klauspost/compress/zstd` —— 纯 Go 的 zstd 实现，已被广泛使用。

前端依赖浏览器原生支持 `Content-Encoding: zstd`，无需额外解压库。

## 数据流

```
Client ─body→ Gateway
              │  ├── ReadAll → metaReqBytes
              │  ├── 上传 meta request artifact
              │  └── 转发到 upstream
                       │  ├── 改写 model 后的字节 → upstreamReqBytes
                       │  ├── 上传 upstream request artifact
                       │  ├── HTTP do
                       │  └── ResponseBody  ─Tee→ MultiWriter(client, buf)
                                       │
                                       ├── 上传 upstream response artifact
                                       └── 上传 meta response artifact
                                            (与 upstream response 同字节，只是 key 不同)
```

`meta response artifact` 与最终成功的那次 upstream response 字节相同，但 header 反映我们写出去给客户端的 header（剔除了 `Content-Length`，并保留我们 `WriteHeader` 时实际响应给客户端的 status code）。

## 模块布局

- `pkg/artifacts/`
  - `sink.go` —— `Sink` 接口、MinIO 实现、空实现；worker pool；`Put` / `PresignedGet`。
  - `payload.go` —— JSON 结构、`Build(method, url, statusCode, header, body)` 序列化 + zstd 压缩。
  - `key.go` —— `RequestKey(id, createdAt)` / `ResponseKey(id, createdAt)`，日期段使用 `time.Format("2006-01-02")`。
- `pkg/server/handle_gateway.go` —— 在 6 个关键节点调用 `s.artifacts.Put`。
- `pkg/server/handle_requests.go` —— `RequestView` 填充 presigned URL。
- `pkg/contract/request.go` —— `RequestView` 添加 `RequestArtifactUrl`、`ResponseArtifactUrl`。

## 错误与降级

- artifact 上传失败 → warn log，不影响请求。
- presigned 失败 → 列表行不带 URL；前端隐藏「Raw」标签页或显示「artifact 不可用」。
- artifact 功能未开启 → 列表行不带 URL；前端 Raw 标签页禁用。

## 前端

`RequestDetailsPanel.vue` 在现有 sections 之上新增 `<Tabs>`：「概览 / 原始请求 / 原始响应」。

- 概览：保留现有 Field 列表。
- 原始请求 / 原始响应：根据 `selected` 拉取对应 URL：
  - 若无 URL：显示 `StateText`「未启用 artifact 记录」。
  - 否则 `fetch(url)`，按 JSON 解析（fetch 出错则报错）。
  - 渲染分两块：
    - **Headers**：`DataTable` 两列（key / value），多值用 `, ` 拼接。
    - **Body**：JSON 格式化（如果 `Content-Type: application/json` 或 body 是合法 JSON）—— 用 `JSON.stringify(JSON.parse(body), null, 2)`；否则原样显示。`bodyEncoding === 'base64'` 时显示 `[binary, NN bytes]` + 「下载原始数据」按钮（直接锚点 presigned URL）。

artifact JSON 与压缩流均由后端给的 presigned URL 直接拉取，不经过 management API，避免占用 huma 请求路径。
