# 执行计划

## 后端

### 0. 依赖与常量
- `go get github.com/orcaman/concurrent-map/v2`（更新 `go.mod` / `go.sum`）。
- `pkg/db/request_constants.go`：新增 `FinishReasonDashboardCancelled = 7`。

### 1. 进行中请求注册表
- 新建 `pkg/server/live_requests.go`：用 `cmap.ConcurrentMap[string, *liveEntry]`（concurrent-map/v2）实现 `liveRequestRegistry`、`liveEntry`、`liveProgress` 及方法（`RegisterMeta` / `RegisterUpstream` / `Remove` / `Interrupt(id, reason)` / `StopReason` / `Snapshot`、`liveProgress.markHeaders` / `recordChunk`），见 `design.md`。`liveProgress` 内保留一把 `sync.Mutex` 保护 `bytes.Buffer`。
- `pkg/server/server.go`：`Server` 增加 `liveRequests *liveRequestRegistry`，`NewServer` 用 `newLiveRequestRegistry()` 初始化。

### 2. 取消上下文改造
- `pkg/server/gateway_flow_context.go`：`gatewayContexts` 增加 `cancelRequest context.CancelFunc`；`newGatewayContexts` 用 `context.WithCancel(r.Context())` 生成可取消的 `Request`；`persistBase` 保持由 `WithoutCancel` 派生。暴露 `cancelRequest` 给 flow（字段或方法）。
- `pkg/server/gateway_flow.go` `run()`：`defer f.ctxs.cancelRequest()`；并 `defer f.h.liveRequests.Remove(metaID)`（在 metaID 已知后）。

### 3. meta 注册与循环守卫
- `gateway_flow.go insertMetaRequest`：写入 meta 行后 `f.h.liveRequests.RegisterMeta(metaID, f.ctxs.cancelRequest)`。
- `gateway_flow_attempts.go runAttempts`：主循环顶部加 `if f.ctxs.Request.Err() != nil { break }`。

### 4. upstream 注册与进度透传
- `gateway_flow_attempts.go insertUpstreamAttempt`：写入 upstream 行后 `entry := f.h.liveRequests.RegisterUpstream(upstreamID, cancel)`；将 `entry`（或 `entry.progress`）放入 `attemptInput`（新增字段）。
- `runSingleAttempt`：拿到 `input` 后 `defer f.h.liveRequests.Remove(input.UpstreamID)`。
- 新增 flow 辅助 `finishReasonFor(rowID, fallback)`（见 `design.md`：查该行停止原因，回落到 meta 停止原因，再回落到 fallback）。
- `forwardRequest` 错误分支：`recordAttemptFailure` 的 finish reason 改为 `finishReasonFor(input.UpstreamID, classifyForwardError(...))`。

### 5. 路径 success 路径进度记录
- `gateway_flow_success.go markPathHeadersReceived`：对 upstream entry `progress.markHeaders(statusCode)`；对 meta entry `active.Store(upstream.progress)`。
- `pipePathResponse` 主循环：`buf[:n]` 写出处调用 `progress.recordChunk(buf[:n])`。
- 流式结束 finish reason：`completeGatewaySuccess` 中 meta 行用 `finishReasonFor(metaID, …)`、upstream 行用 `finishReasonFor(upstreamID, …)`（fallback 为 `classifyStreamFinishReason` 结果）。

### 6. 统一网关 success 路径进度记录
- `gateway_unified_helpers.go unifiedStreamSuccess`：写 `RequestStatusHeaderReceived` 处 `markHeaders` + 设 meta `active`；客户端写出主循环 `recordChunk(buf[:n])`；流式结束按 `finishReasonFor` 解析 meta / upstream 两行的 finish reason。
- 需把 upstream entry/progress 从 `successInput`（其 `attemptInput`）取到；确认 `unifiedStreamArgsFromSuccess` 能拿到 `input` 中的 entry。

### 7. Contract 与 handler
- `pkg/contract/request.go`：新增 `InterruptRequest*`、`GetRequestLive*` 类型与 `OperationInterruptRequest`、`OperationGetRequestLive`（见 `api.md`）。
- `pkg/server/handle_requests.go`（或新建 `handle_request_live.go`）：实现 `handleInterruptRequest`、`handleGetRequestLive`，读/调用 `s.liveRequests`。`interrupt` 调用 `Interrupt(id, db.FinishReasonDashboardCancelled)`。`live` 把 `liveSnapshot` 映射为 `RequestLiveView`，`phase` 由 `headersReceived` + `bytes>0` 推导。
- `server.go registerOperations`：注册两个新操作。

### 8. 重新生成 OpenAPI
- `mise run openapi`，再 `pnpm --dir dashboard generate-openapi`。

## 前端

### 9. API 客户端与 finish reason 标签
- `dashboard/src/api/client.ts`：`getRequestLive(id)`（GET `/requests/{id}/live`）、`interruptRequest(id)`（POST `/requests/{id}/interrupt`）。
- `dashboard/src/api/queryKeys.ts`：新增 `requestLive.detail(id)` key。
- `dashboard/src/utils/requestLabels.ts`：`finishReasonLabel` 增加 `case 7: return '控制台打断'`（与 `case 2: '已取消'` 区分）。

### 10. span 列表 UI
- `dashboard/src/components/RequestDetailsContent.vue`：
  - 进行中判定复用 `status === 0 || status === 1`。
  - 进行中 span 行显示「打断」按钮（用 `src/ui/` 现有按钮原语），点击 `interruptRequest(span.id)`，成功后 `invalidateQueries` spans 与 live。
  - 选中进行中 span 时，详情区显示实时快照：阶段、字节数、响应体至今内容（文本块）。
  - live `useQuery`：`enabled` 仅在选中行进行中时为真，**不设** `refetchInterval`。
  - 新增「刷新」动作，手动 `refetch` spans + live。

## 验证
- 后端 `go build ./...`；如涉及 helper 可加/跑 `pkg/server` 现有测试。
- 前端 `pnpm --dir dashboard type-check`、`pnpm --dir dashboard lint`、`pnpm --dir dashboard build`。
- 手动验证（参考 `docker compose up -d` + `mise run server` + `mise run web`）：
  1. 发起一个慢的流式请求，详情页能看到 headers/字节/响应体实时内容（手动刷新更新）。
  2. headers 之前打断某 upstream → 走下一个 provider。
  3. headers 之后打断某 upstream → 客户端流被截断，不再尝试下一个 provider。
  4. 打断 meta → 整条链路终止。
  5. 完成后内存条目释放（`live` 返回 `inFlight=false`）。
