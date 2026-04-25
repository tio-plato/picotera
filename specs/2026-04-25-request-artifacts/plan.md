# Plan — Request Artifacts

## Task 1 — Config 与依赖

**目标**：把 MinIO 与 zstd 的依赖、配置接入项目。

**改动**

- `go get github.com/minio/minio-go/v7 github.com/klauspost/compress/zstd`
- `pkg/configx/configx.go`：新增 `S3Config` 子结构与 `Config.S3` 字段，按 `design.md` 表格定义；`bindEnvs` 已支持嵌套结构，`S3.UseSSL` 默认 `false`，`S3.Region` 默认 `us-east-1`。
- `docker-compose.yaml`：新增 `minio` 服务（`minio/minio` 镜像、9000 → 34050、9001 → 34049、`MINIO_ROOT_USER=picotera`、`MINIO_ROOT_PASSWORD=picotera-dev`），`volumes: minio-data:/data`；新增 `mc-init` 一次性容器自动创建 bucket `picotera-artifacts`。

**验证**

- `go build ./...` 通过。
- `docker compose up -d minio` 启动后浏览器访问 `localhost:34049` 能登录。
- 启动 picotera，未配置 S3 时不报错，`logx` 输出 `artifact disabled`。

## Task 2 — `pkg/artifacts` 包

**目标**：提供 `Sink` 接口、MinIO 实现、payload 构建工具。

**改动**

- `pkg/artifacts/key.go`
  ```go
  func RequestKey(id string, ts time.Time) string
  func ResponseKey(id string, ts time.Time) string
  ```
  内部 `ts.UTC().Format("2006-01-02")`，拼出 `artifacts/<date>/<id>.{request,response}.json.zst`。
- `pkg/artifacts/payload.go`
  ```go
  type Payload struct { ... }  // 对应 api.md JSON schema
  func BuildRequest(method, url string, header http.Header, body []byte) ([]byte, error)
  func BuildResponse(statusCode int, header http.Header, body []byte) ([]byte, error)
  ```
  内部步骤：构造 struct → `utf8.Valid(body)` 判定 encoding → `json.Marshal` → `zstd.NewWriter` 压缩到 buffer → 返回字节。
- `pkg/artifacts/sink.go`
  ```go
  type Sink interface {
    Put(ctx context.Context, key string, payload []byte)            // 异步
    PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error)
    Enabled() bool
  }
  ```
  - `noopSink` 实现：`Enabled()` 返回 false，所有方法 no-op / 返回空。
  - `minioSink` 实现：构造时调用 `minio.New(...)`，启动 N=4 个 worker 消费 `chan job{key, payload}`，buffered 256；`Put` 非阻塞 `select`，满则 warn log 丢弃；上传调用 `client.PutObject(ctx, bucket, key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: "application/json", ContentEncoding: "zstd"})`；`PresignedGet` 调 `client.PresignedGetObject(ctx, bucket, key, ttl, nil)`，若配置了 `PublicURL`，把返回 URL 的 host 替换为该值。
  - 工厂函数 `NewSink(cfg configx.S3Config, logger *logrus.Entry) (Sink, error)`，`Endpoint == ""` 时返回 `noopSink`。

**验证**

- `go build ./...` 通过。
- 写一个手动小程序或在 `cmd/picotera` 加临时 flag 验证 `PutObject` + `PresignedGetObject` 走通；接受手动验证。

## Task 3 — Server 接入 sink

**改动**

- `pkg/server/server.go`：
  - `Server` 添加 `artifacts artifacts.Sink` 字段。
  - `NewServer` 中调用 `artifacts.NewSink(config.S3, logx.WithContext(ctx))`，失败 → warn log + 用 noop。
- `pkg/server/handle_gateway.go`：
  - 在 `insertRequest(meta)` 后立即记录 `metaCreatedAt := time.Now().UTC()` 用于 key 生成（DB 默认 `CURRENT_TIMESTAMP`，但本地推算即可，前端用列表返回的 `createdAt` 拉链接，必须保证两者用同一个时间——所以 sink 用本地 ts，列表 handler 也用 DB 的 `created_at` 直接 format；二者偏差可接受，下面 Task 4 单独说）。
  - **方案修正**：直接在 `InsertRequest` 时显式传入 `created_at`（migration / sqlc 暂不支持显式 created_at）；改为：handler 推算 key 时**只用 DB 返回的 `created_at`**。需要在 `InsertRequest` 后 `SELECT created_at` 回读，或在 sql 中改为 `RETURNING created_at`。最简单：把 `InsertRequest` 改为 `:one` 返回 `created_at`，gateway 拿到后传给 sink。
  - 改写流程：
    1. 入口 `body, _ := io.ReadAll(r.Body)` 后，构造 `metaReqHeader := r.Header.Clone()`；插入 meta 时拿到 `metaCreatedAt`；提交 `s.artifacts.Put(ctx, RequestKey(metaID, metaCreatedAt), BuildRequest(r.Method, r.URL.String(), metaReqHeader, body))`。
    2. 进入 provider 循环，每次构建 `req` 后插入 upstream 行（同样改 `:one` 拿 `upstreamCreatedAt`），把 `req.Body` 的字节（`buildUpstreamRequest` 已知 `reqBody`，让它也 return `reqBody`）打成 upstream request artifact。
    3. forward 后 `resp` 200 路径：把现有 `for { reader.Read; w.Write }` 改成 `tee := io.MultiWriter(w, &captureBuf)`；用 `io.Copy(tee, reader)` 替换原循环（保留 flusher 行为：可以用自定义 writer 包装 `w`，每次 Write 后 flush）。完成后：
       - 上传 upstream response：`BuildResponse(resp.StatusCode, resp.Header, captureBuf.Bytes())` → `ResponseKey(upstreamID, upstreamCreatedAt)`。
       - 上传 meta response：用我们写出去的 header（即剥离 `Content-Length` 后的 header）和同字节 body → `ResponseKey(metaID, metaCreatedAt)`。
    4. 非 200 路径：`respBody` 已读到，构造 upstream response artifact 上传。meta response artifact 在「all providers failed」分支统一上传：使用 `writeGatewayError` 写入的 JSON 字节作为 body（把 `writeGatewayError` 改为返回它写入的字节，或抽出 helper 返回字节再 `w.Write`）。
    5. 早期失败分支（resolveEndpoint 等）：调用 `failMetaResponse(status, msg)` 同时写 DB + 上传 meta response artifact（body 是错误 JSON）。

**验证**

- `go build ./...` 通过。
- 启动后访问一次 gateway，MinIO console 中能看到 4 个文件（meta req+resp，upstream req+resp）。
- 失败重试场景：能看到 N 对 upstream artifacts + 1 对 meta artifacts。

## Task 4 — 列表 / 详情返回 presigned URL

**改动**

- `pkg/contract/request.go`：`RequestView` 新增 `RequestArtifactUrl`、`ResponseArtifactUrl`（`string`，`omitempty`）。
- `toRequestView` 不知道 sink，因此在所有 handler 拿到 view 后再单独填充：抽 helper `func (s *Server) attachArtifactUrls(ctx context.Context, v *contract.RequestView, createdAt time.Time)`，内部用 `v.ID` + `createdAt` 算 key，调 `s.artifacts.PresignedGet(ctx, key, time.Hour)`，错误时不填、warn log。
- `pkg/server/handle_requests.go`：
  - `handleListRequests`：在 for 循环里对每个 row 调 `attachArtifactUrls`。
  - `handleGetRequest`、`handleListRequestSpans` 同样。
- 注意 `requestLike.CreatedAt` 是 `pgtype.Timestamp`，全部 query row 都包含它；不必额外查询。

**验证**

- `mise run openapi` 生成的 `openapi.yaml` 中 `RequestView` 包含两个新字段。
- `pnpm --dir dashboard type-check` 通过；`dashboard/src/api.d.ts` 含新字段。
- `curl /api/picotera/requests | jq '.items[0] | {requestArtifactUrl, responseArtifactUrl}'` 看到非空 URL。

## Task 5 — 前端 Raw 标签页

**改动**

- `dashboard/src/components/RequestDetailsPanel.vue`：
  - 在 selected 详情区上方包一层 `<Tabs>`（来自 `@/ui`，参考已有用法）：tab1 「概览」（保留所有 Field section），tab2 「请求」（artifact = request），tab3 「响应」（artifact = response）。
  - 新增 `RawArtifactView.vue`（`dashboard/src/components/`）：props `{ url?: string; kind: 'request' | 'response' }`。
    - `url` 为空时显示 `StateText`「未启用 artifact 记录」。
    - 否则 `onMounted` `fetch(url)` 后 `await res.json()`，浏览器原生处理 `Content-Encoding: zstd`。
    - 渲染：
      - Method / URL / Status：顶部 `Field` 行。
      - **Headers**：`DataTable`，两列 key / value（多值 `,` 拼）。
      - **Body**：
        - `bodyEncoding === 'base64'` → `[binary, ${len} bytes]` + 「下载」`<a :href="url" download>`。
        - 否则尝试 `JSON.parse` + `JSON.stringify(_, null, 2)`；失败则原样字符串。
        - 用 `<pre class="font-mono text-xs whitespace-pre-wrap bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink overflow-auto max-h-[480px]">` 包裹。
  - 切换 tab 时复用同一个 selected span；切换 selected 时重新拉取。
- 选择 `Tabs`：先确认 `src/ui/Tabs.vue` 的 props，按其 API 接入；若 API 不合需求，扩展或新加一个轻量 segmented switch（沿用现有 SegmentedControl 也可——以最贴近 UI 库现状的为准）。

**验证**

- `pnpm --dir dashboard build` 通过。
- 触发若干请求：
  - 默认进 RequestsView，点开任意 meta：「请求」tab 渲染出 client headers + JSON body；「响应」tab 渲染 LLM 回复（流式响应也能看到完整 body）。
  - 切到某个 upstream span：tabs 内容刷新为 upstream request / response。
  - 关闭 S3：tabs 显示「未启用 artifact 记录」，无报错。

## Task 6 — 端到端冒烟与提交

**步骤**

1. `docker compose up -d`（含 minio）。
2. `mise run server`，环境变量含 S3。
3. 用 curl 触发：
   - 一次成功的 chat completion；
   - 一次故意失败（错 model）；
   - 一次首选 provider 失败、备选成功的重试。
4. MinIO console 检查文件存在并能下载、用 `zstd -d` 还原成 JSON。
5. `mise run web`，在 RequestsView 打开各请求，三个 tab 内容正确。
6. 关闭 S3 后再跑一次：网关 200 正常返回，前端 tabs 显示禁用文案。

**提交**（按改动分两个 commit）

- 后端：`feat(artifacts): record request/response artifacts to s3`
- 前端：`feat(dashboard): show raw request and response`
